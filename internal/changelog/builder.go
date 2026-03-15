package changelog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/codested/chagg/internal/changeentry"
)

// typeOrder controls the display order of change types within a version group.
var typeOrder = []changeentry.ChangeType{
	changeentry.ChangeTypeFeature,
	changeentry.ChangeTypeFix,
	changeentry.ChangeTypeRemoval,
	changeentry.ChangeTypeSecurity,
	changeentry.ChangeTypeDocs,
}

var typeTitles = map[changeentry.ChangeType]string{
	changeentry.ChangeTypeFeature:  "Features",
	changeentry.ChangeTypeFix:      "Bug Fixes",
	changeentry.ChangeTypeRemoval:  "Removals",
	changeentry.ChangeTypeSecurity: "Security",
	changeentry.ChangeTypeDocs:     "Documentation",
}

// LoadChangeLog reads every .md file from changesDir, resolves each file's
// version using git tags discovered at repoRoot, applies filter, and returns
// the fully grouped changelog.
//
// Git errors are treated as "no history available": all entries are then
// assigned to the staging bucket. File-system errors are returned as-is.
func LoadChangeLog(repoRoot string, module changeentry.ModuleConfig, filter FilterOptions) (*ChangeLog, error) {
	tags, _ := ListSemVerTags(repoRoot, module.TagPrefix) // non-fatal; proceed without history

	entries, invalidEntries, err := loadEntries(repoRoot, module, tags)
	if err != nil {
		return nil, err
	}

	if len(invalidEntries) > 0 {
		return nil, invalidEntriesError(invalidEntries)
	}

	entries = applyFilter(entries, filter)

	cl := buildChangeLog(entries, tags)
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
//  1. Since: keep staging (when ShowStaged) + every version >= Since.
//  2. N + ShowStaged: keep up to N tagged versions newest-first, plus staging when ShowStaged.
//     N = 0 means unlimited.
type VersionFilterOptions struct {
	N          int    // max tagged releases to include (0 = all)
	ShowStaged bool   // include staging group
	Since      string // lower version boundary (inclusive)
}

// ApplyVersionFilter restricts the changelog according to opts.
func ApplyVersionFilter(cl *ChangeLog, opts VersionFilterOptions) *ChangeLog {
	if opts.Since != "" {
		var filtered []VersionGroup
		for _, g := range cl.Groups {
			if g.IsStaging() {
				if opts.ShowStaged {
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
			if opts.ShowStaged {
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
func loadEntries(repoRoot string, module changeentry.ModuleConfig, tags []Tag) ([]EntryWithMeta, []InvalidEntry, error) {
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

		entry, errs := changeentry.ParseEntryWithDefaults(string(contentBytes), path, module.DefaultAudience)
		if len(errs) > 0 {
			invalidEntries = append(invalidEntries, InvalidEntry{Path: path, Errors: errs})
			continue // keep collecting to report all invalid files at once
		}

		// Per-file lookup keeps --follow semantics so moved files are attributed
		// to the commit where they originally entered history.
		addedAt, addedCommitHash, originalFilename, hasGit := FileAddedMeta(repoRoot, path)

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
func resolveVersion(entry changeentry.Entry, addedAt time.Time, hasGit bool, tags []Tag) string {
	if entry.Release != "" {
		// Try to find a tag whose name matches the pinned value.
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

func buildChangeLog(entries []EntryWithMeta, tags []Tag) *ChangeLog {
	groupMap := make(map[string][]EntryWithMeta)
	for _, e := range entries {
		groupMap[e.Version] = append(groupMap[e.Version], e)
	}

	var groups []VersionGroup

	// Staging first.
	if stagingEntries, ok := groupMap["staging"]; ok {
		groups = append(groups, buildVersionGroup("staging", nil, stagingEntries))
	}

	// Tagged versions, newest → oldest (tags slice is oldest → newest).
	tagSet := make(map[string]bool, len(tags))
	for i := len(tags) - 1; i >= 0; i-- {
		tag := tags[i]
		tagSet[tag.Name] = true
		if tagEntries, ok := groupMap[tag.Name]; ok {
			groups = append(groups, buildVersionGroup(tag.Name, new(tag), tagEntries))
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
		groups = append(groups, buildVersionGroup(version, nil, groupMap[version]))
	}

	return &ChangeLog{Groups: groups}
}

func buildVersionGroup(version string, tag *Tag, entries []EntryWithMeta) VersionGroup {
	// Sort: rank desc (higher first), then addedAt desc, then path asc (deterministic).
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Entry.Priority != entries[j].Entry.Priority {
			return entries[i].Entry.Priority > entries[j].Entry.Priority
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

	var typeGroups []TypeGroup
	for _, ct := range typeOrder {
		if typeEntries, ok := typeMap[ct]; ok {
			typeGroups = append(typeGroups, TypeGroup{
				ChangeType: ct,
				Title:      typeTitles[ct],
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

	aVersion, _, aErr := ParseSemVersion(semverFromTag(a))
	bVersion, _, bErr := ParseSemVersion(semverFromTag(b))
	if aErr != nil || bErr != nil {
		return strings.EqualFold(stripV(semverFromTag(a)), stripV(semverFromTag(b)))
	}

	return CompareSemVersion(aVersion, bVersion) == 0
}

func stripV(v string) string {
	return strings.TrimPrefix(strings.TrimSpace(v), "v")
}

func releaseMatchesTag(release string, tag Tag) bool {
	trimmed := strings.TrimSpace(release)
	if strings.EqualFold(trimmed, tag.Name) {
		return true
	}

	if strings.EqualFold(trimmed, semverFromTag(tag.Name)) {
		return true
	}

	releaseVersion, _, err := ParseSemVersion(semverFromTag(trimmed))
	if err != nil {
		return false
	}

	return CompareSemVersion(releaseVersion, tag.Version) == 0
}

func semverFromTag(value string) string {
	trimmed := strings.TrimSpace(value)
	index := strings.LastIndex(trimmed, "/")
	if index == -1 {
		return trimmed
	}

	candidate := trimmed[index+1:]
	if _, _, err := ParseSemVersion(candidate); err == nil {
		return candidate
	}

	return trimmed
}

func NextPreReleaseLabelVersion(base SemVersion, label string, tags []Tag) SemVersion {
	next := base
	next.PreRelease = label + ".1"
	next.Build = ""

	maxN := 0
	prefix := label + "."
	for _, tag := range tags {
		if tag.Version.Major != base.Major || tag.Version.Minor != base.Minor || tag.Version.Patch != base.Patch {
			continue
		}
		if !strings.HasPrefix(tag.Version.PreRelease, prefix) {
			continue
		}

		nPart := strings.TrimPrefix(tag.Version.PreRelease, prefix)
		n, err := strconv.Atoi(nPart)
		if err != nil {
			continue
		}
		if n > maxN {
			maxN = n
		}
	}

	next.PreRelease = fmt.Sprintf("%s.%d", label, maxN+1)
	return next
}
