package changelog

import (
	"strings"
	"time"

	"github.com/codested/chagg/internal/changeentry"
)

// EntryWithMeta pairs a parsed change entry with the metadata derived from
// git history: when the file was first committed and which version it belongs to.
type EntryWithMeta struct {
	Entry   changeentry.Entry
	Module  changeentry.ModuleConfig
	Path    string    // absolute path to the .md file
	AddedAt time.Time // author date of the commit that first added the file
	HasGit  bool      // false when the file is untracked or git is unavailable
	Version string    // "staging" or a version tag name / pinned release value
}

// Preview returns the first non-empty, non-heading line of the entry body,
// suitable for single-line display in the log output.
func (e EntryWithMeta) Preview() string {
	for _, line := range strings.Split(e.Entry.Body, "\n") {
		line = strings.TrimLeft(strings.TrimSpace(line), "#")
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}

// TypeGroup holds all entries that share the same change type within one
// version group.
type TypeGroup struct {
	ChangeType changeentry.ChangeType
	Title      string
	Entries    []EntryWithMeta
}

// VersionGroup holds all entries attributed to one version (or staging).
type VersionGroup struct {
	Version    string // "staging" or a version tag/release name
	Tag        *Tag   // nil for staging and pinned-but-untagged versions
	TypeGroups []TypeGroup
}

// IsStaging reports whether this group contains unreleased changes.
func (g VersionGroup) IsStaging() bool {
	return g.Version == "staging"
}

// VersionTitle returns a display-friendly heading for the version.
func (g VersionGroup) VersionTitle() string {
	if g.IsStaging() {
		return "Unreleased"
	}
	return g.Version
}

// FormattedDate returns the tag date as "YYYY-MM-DD", or an empty string when
// the date is unknown.
func (g VersionGroup) FormattedDate() string {
	if g.Tag == nil || g.Tag.CommitDate.IsZero() {
		return ""
	}
	return g.Tag.CommitDate.Format("2006-01-02")
}

// TotalEntries returns the sum of all entries across all type groups.
func (g VersionGroup) TotalEntries() int {
	n := 0
	for _, tg := range g.TypeGroups {
		n += len(tg.Entries)
	}
	return n
}

// ChangeLog is the full ordered set of version groups.
// Groups are ordered: staging first (when present), then newest tag → oldest.
type ChangeLog struct {
	Module changeentry.ModuleConfig
	Groups []VersionGroup
}

// FilterOptions controls which entries are retained after loading.
type FilterOptions struct {
	Audience  string // keep only entries whose audience list contains this value
	Component string // keep only entries whose component list contains this value
	Type      string // keep only entries whose type matches this value
}
