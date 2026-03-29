package changelog

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/semver"
)

// EntryWithMeta pairs a parsed change entry with the metadata derived from
// git history: when the file was first committed and which version it belongs to.
type EntryWithMeta struct {
	Entry            changeentry.Entry
	Module           changeentry.ModuleConfig
	Path             string    // absolute path to the .md file
	AddedAt          time.Time // author date of the commit that first added the file
	AddedCommitHash  string    // commit hash where the file was first introduced
	OriginalFilename string    // filename at first introduction
	HasGit           bool      // false when the file is untracked or git is unavailable
	Version          string    // "staging" or a version tag name / pinned release value
}

func (e EntryWithMeta) ID() string {
	filename := strings.TrimSpace(e.OriginalFilename)
	if filename == "" {
		filename = filepath.Base(e.Path)
	}

	hash := strings.TrimSpace(e.AddedCommitHash)
	if hash == "" {
		hash = "untracked"
	}

	return filename + "@" + hash
}

// Preview returns the first non-empty line of the entry body with markdown
// formatting stripped (headings, bold, italic, etc.), suitable for
// single-line display.
func (e EntryWithMeta) Preview() string {
	for _, line := range strings.Split(e.Entry.Body, "\n") {
		line = stripMarkdownInline(strings.TrimSpace(line))
		if line != "" {
			return line
		}
	}
	return ""
}

// BodyWithoutPreviewLine returns the body with the first content line removed
// (the line that Preview() extracts). This avoids duplicating the preview
// text inside the body. Returns empty string when nothing remains.
func (e EntryWithMeta) BodyWithoutPreviewLine() string {
	lines := strings.Split(e.Entry.Body, "\n")
	// Find and skip the first non-empty line (the one Preview extracts).
	found := false
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			rest := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			if rest != "" {
				found = true
			}
			if found {
				return rest
			}
			break
		}
	}
	return ""
}

// stripMarkdownInline removes common markdown inline formatting from a line:
// heading markers (#), bold (**), italic (*), strikethrough (~~), inline code (`).
func stripMarkdownInline(line string) string {
	// Strip leading heading markers.
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)

	// Strip bold/italic markers and inline code backticks.
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "~~", "")
	line = strings.ReplaceAll(line, "*", "")
	line = strings.ReplaceAll(line, "`", "")

	return strings.TrimSpace(line)
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
	Version    string      // "staging" or a version tag/release name
	Tag        *semver.Tag // nil for staging and pinned-but-untagged versions
	TypeGroups []TypeGroup
}

// IsStaging reports whether this group contains unreleased changes.
func (g VersionGroup) IsStaging() bool {
	return g.Version == "staging"
}

// VersionTitle returns a display-friendly heading for the version.
// For released versions, it strips a leading "v" prefix so that
// "v1.2.3" becomes "1.2.3" while the raw tag is preserved in Version.
func (g VersionGroup) VersionTitle() string {
	if g.IsStaging() {
		return "Unreleased"
	}
	return strings.TrimPrefix(g.Version, "v")
}

// FormattedDate returns the tag date as an RFC 3339 timestamp, or an empty
// string when the date is unknown.
func (g VersionGroup) FormattedDate() string {
	if g.Tag == nil || g.Tag.CommitDate.IsZero() {
		return ""
	}
	return g.Tag.CommitDate.Format(time.RFC3339)
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
	Module         changeentry.ModuleConfig
	InvalidEntries []InvalidEntry
	Groups         []VersionGroup
}

type InvalidEntry struct {
	Path   string
	Errors []error
}

// FilterOptions controls which entries are retained after loading.
type FilterOptions struct {
	Audience  string // keep only entries whose audience list contains this value
	Component string // keep only entries whose component list contains this value
	Type      string // keep only entries whose type matches this value
}
