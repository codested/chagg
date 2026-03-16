package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/gitutil"
	"github.com/codested/chagg/internal/semver"
)

// LoadChangeLog reads every .md file from changesDir, resolves each file's
// version using git tags discovered at repoRoot, applies filter, and returns
// the fully grouped changelog.
//
// When git is unavailable, all entries are assigned to the staging bucket.
// When git is available but tag metadata cannot be read, an error is returned.
// File-system errors are returned as-is.
func LoadChangeLog(repoRoot string, module changeentry.ModuleConfig, filter FilterOptions) (*ChangeLog, error) {
	tags, err := gitutil.ListSemVerTags(repoRoot, module.TagPrefix)
	if err != nil {
		return nil, fmt.Errorf("load version tags: %w", err)
	}

	entries, invalidEntries, err := loadEntries(repoRoot, module, tags)
	if err != nil {
		return nil, err
	}

	if len(invalidEntries) > 0 {
		return nil, invalidEntriesError(invalidEntries)
	}

	entries = applyFilter(entries, filter)

	cl := buildChangeLog(entries, tags, module)
	cl.Module = module
	cl.InvalidEntries = invalidEntries
	return cl, nil
}

// StagingOnly returns a changelog that contains only the staging group.
func StagingOnly(cl *ChangeLog) *ChangeLog {
	for _, g := range cl.Groups {
		if g.IsStaging() {
			return &ChangeLog{Module: cl.Module, Groups: []VersionGroup{g}}
		}
	}
	return &ChangeLog{Module: cl.Module}
}

// VersionOnly returns a changelog that contains only the named version group.
// Version matching is case-insensitive and tolerates a leading "v" on either side.
func VersionOnly(cl *ChangeLog, version string) *ChangeLog {
	for _, g := range cl.Groups {
		if versionMatches(g.Version, version) {
			return &ChangeLog{Module: cl.Module, Groups: []VersionGroup{g}}
		}
	}
	return &ChangeLog{Module: cl.Module}
}

// VersionFilterOptions controls which version groups are retained after loading.
//
// Evaluation order:
//  1. OnlyStaging: keep only the staging group.
//  2. Since: keep staging (when ShowStaging) + every version >= Since.
//  3. N + ShowStaging: keep up to N tagged versions newest-first, plus staging when ShowStaging.
//     N = 0 means unlimited.
type VersionFilterOptions struct {
	N           int    // max tagged releases to include (0 = all)
	ShowStaging bool   // include staging group
	OnlyStaging bool   // include only staging group
	Since       string // lower version boundary (inclusive)
}

// ApplyVersionFilter restricts the changelog according to opts.
func ApplyVersionFilter(cl *ChangeLog, opts VersionFilterOptions) *ChangeLog {
	if opts.OnlyStaging {
		return StagingOnly(cl)
	}

	if opts.Since != "" {
		var filtered []VersionGroup
		for _, g := range cl.Groups {
			if g.IsStaging() {
				if opts.ShowStaging {
					filtered = append(filtered, g)
				}
				continue
			}
			filtered = append(filtered, g)
			if versionMatches(g.Version, opts.Since) {
				break
			}
		}
		return &ChangeLog{Module: cl.Module, Groups: filtered}
	}

	var filtered []VersionGroup
	tagCount := 0
	for _, g := range cl.Groups {
		if g.IsStaging() {
			if opts.ShowStaging {
				filtered = append(filtered, g)
			}
			continue
		}
		if opts.N == 0 || tagCount < opts.N {
			filtered = append(filtered, g)
			tagCount++
		}
	}
	return &ChangeLog{Module: cl.Module, Groups: filtered}
}

// loadEntries walks changesDir, parses each .md file, and resolves version info.
func loadEntries(repoRoot string, module changeentry.ModuleConfig, tags []semver.Tag) ([]EntryWithMeta, []InvalidEntry, error) {
	changesDir := module.ChangesDir
	if _, statErr := os.Stat(changesDir); os.IsNotExist(statErr) {
		return nil, nil, nil
	}

	paths, err := collectChangeEntryPaths(changesDir)
	if err != nil {
		return nil, nil, err
	}

	entries := make([]EntryWithMeta, 0, len(paths))
	invalidEntries := make([]InvalidEntry, 0)
	for _, path := range paths {
		contentBytes, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, nil, fmt.Errorf("read %s: %w", path, readErr)
		}

		entry, errs := changeentry.ParseEntry(string(contentBytes), path, module)
		if len(errs) > 0 {
			invalidEntries = append(invalidEntries, InvalidEntry{Path: path, Errors: errs})
			continue // keep collecting to report all invalid files at once
		}

		// Per-file lookup keeps --follow semantics so moved files are attributed
		// to the commit where they originally entered history.
		addedAt, addedCommitHash, originalFilename, hasGit := gitutil.FileAddedMeta(repoRoot, path)

		version := resolveVersion(entry, addedAt, hasGit, tags)

		entries = append(entries, EntryWithMeta{
			Entry:            entry,
			Module:           module,
			Path:             path,
			AddedAt:          addedAt,
			AddedCommitHash:  addedCommitHash,
			OriginalFilename: originalFilename,
			HasGit:           hasGit,
			Version:          version,
		})
	}

	return entries, invalidEntries, nil
}

func collectChangeEntryPaths(changesDir string) ([]string, error) {
	paths := make([]string, 0)

	err := filepath.WalkDir(changesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".md") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})

	return paths, err
}

func invalidEntriesError(invalidEntries []InvalidEntry) error {
	first := invalidEntries[0]
	firstMessage := "unknown validation error"
	if len(first.Errors) > 0 {
		firstMessage = first.Errors[0].Error()
	}

	return changeentry.NewValidationError(
		"changes",
		fmt.Sprintf(
			"found %d invalid change %s (first: %s: %s). Run `chagg check` for details",
			len(invalidEntries),
			pluralizeCount(len(invalidEntries), "entry", "entries"),
			first.Path,
			firstMessage,
		),
	)
}

func pluralizeCount(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

// resolveVersion determines which version label to assign to an entry.
//
// Priority:
//  1. The entry's release: field (pinned) — matched against known tags,
//     normalizing the "v" prefix on both sides.
//  2. Git history: file is assigned to the earliest tag whose commit date
//     is >= the file's add date.
//  3. Fallback: "staging".
func resolveVersion(entry changeentry.Entry, addedAt time.Time, hasGit bool, tags []semver.Tag) string {
	return ResolveVersion(entry, addedAt, hasGit, tags)
}

// ResolveVersion determines the version label for an entry given its git add time.
func ResolveVersion(entry changeentry.Entry, addedAt time.Time, hasGit bool, tags []semver.Tag) string {
	if entry.Release != "" {
		for _, tag := range tags {
			if releaseMatchesTag(entry.Release, tag) {
				return tag.Name
			}
		}
		return entry.Release // pinned to a version without a matching tag yet
	}

	if !hasGit || len(tags) == 0 {
		return "staging"
	}

	// Tags are ordered oldest → newest.
	// The entry belongs to the first tag whose commit date is >= the file's add date.
	for _, tag := range tags {
		if !addedAt.After(tag.CommitDate) {
			return tag.Name
		}
	}

	return "staging"
}

func buildChangeLog(entries []EntryWithMeta, tags []semver.Tag, module changeentry.ModuleConfig) *ChangeLog {
	groupMap := make(map[string][]EntryWithMeta)
	for _, e := range entries {
		groupMap[e.Version] = append(groupMap[e.Version], e)
	}

	var groups []VersionGroup

	// Staging first.
	if stagingEntries, ok := groupMap["staging"]; ok {
		groups = append(groups, buildVersionGroup("staging", nil, stagingEntries, module))
	}

	// Tagged versions, newest → oldest (tags slice is oldest → newest).
	tagSet := make(map[string]bool, len(tags))
	for i := len(tags) - 1; i >= 0; i-- {
		tag := tags[i]
		tagSet[tag.Name] = true
		if tagEntries, ok := groupMap[tag.Name]; ok {
			groups = append(groups, buildVersionGroup(tag.Name, new(tag), tagEntries, module))
		}
	}

	// Pinned-but-untagged versions (release: points to a version not yet released).
	tagSet["staging"] = true
	var untagged []string
	for version := range groupMap {
		if !tagSet[version] {
			untagged = append(untagged, version)
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(untagged))) // deterministic ordering
	for _, version := range untagged {
		groups = append(groups, buildVersionGroup(version, nil, groupMap[version], module))
	}

	return &ChangeLog{Groups: groups}
}

func buildVersionGroup(version string, tag *semver.Tag, entries []EntryWithMeta, module changeentry.ModuleConfig) VersionGroup {
	// Sort: rank desc (higher first), then addedAt desc, then path asc (deterministic).
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Entry.Rank != entries[j].Entry.Rank {
			return entries[i].Entry.Rank > entries[j].Entry.Rank
		}
		if !entries[i].AddedAt.Equal(entries[j].AddedAt) {
			return entries[i].AddedAt.After(entries[j].AddedAt)
		}
		return entries[i].Path < entries[j].Path
	})

	typeMap := make(map[changeentry.ChangeType][]EntryWithMeta)
	for _, e := range entries {
		typeMap[e.Entry.Type] = append(typeMap[e.Entry.Type], e)
	}

	// Build type groups in the order defined by the module's type registry.
	var typeGroups []TypeGroup
	for _, def := range module.Types.Definitions() {
		if typeEntries, ok := typeMap[def.ID]; ok {
			typeGroups = append(typeGroups, TypeGroup{
				ChangeType: def.ID,
				Title:      def.Title,
				Entries:    typeEntries,
			})
		}
	}

	return VersionGroup{
		Version:    version,
		Tag:        tag,
		TypeGroups: typeGroups,
	}
}

func applyFilter(entries []EntryWithMeta, filter FilterOptions) []EntryWithMeta {
	if filter.Audience == "" && filter.Component == "" && filter.Type == "" {
		return entries
	}

	result := make([]EntryWithMeta, 0, len(entries))
	for _, e := range entries {
		if filter.Audience != "" && !containsIgnoreCase(e.Entry.Audience, filter.Audience) {
			continue
		}
		if filter.Component != "" && !containsIgnoreCase(e.Entry.Component, filter.Component) {
			continue
		}
		if filter.Type != "" && !strings.EqualFold(string(e.Entry.Type), filter.Type) {
			continue
		}
		result = append(result, e)
	}

	return result
}

func containsIgnoreCase(slice []string, value string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, value) {
			return true
		}
	}
	return false
}

// versionMatches compares two version strings, tolerating a leading "v" on
// either side (e.g. "v1.2.3" and "1.2.3" are considered equal).
func versionMatches(a, b string) bool {
	if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) {
		return true
	}

	aVersion, _, aErr := semver.ParseSemVersion(semverFromTag(a))
	bVersion, _, bErr := semver.ParseSemVersion(semverFromTag(b))
	if aErr != nil || bErr != nil {
		return strings.EqualFold(stripV(semverFromTag(a)), stripV(semverFromTag(b)))
	}

	return semver.CompareSemVersion(aVersion, bVersion) == 0
}

func stripV(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func releaseMatchesTag(release string, tag semver.Tag) bool {
	trimmed := strings.TrimSpace(release)
	if strings.EqualFold(trimmed, tag.Name) {
		return true
	}

	if strings.EqualFold(trimmed, semverFromTag(tag.Name)) {
		return true
	}

	releaseVersion, _, err := semver.ParseSemVersion(semverFromTag(trimmed))
	if err != nil {
		return false
	}

	return semver.CompareSemVersion(releaseVersion, tag.Version) == 0
}

func semverFromTag(value string) string {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "/")
	if index == -1 {
		return trimmed
	}

	candidate := trimmed[index+1:]
	if _, _, err := semver.ParseSemVersion(candidate); err == nil {
		return candidate
	}

	return trimmed
}
