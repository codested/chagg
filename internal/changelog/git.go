package changelog

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Tag represents a SemVer git tag together with the author date of its
// tagged commit. That date is used to decide which version window a
// change file falls into.
type Tag struct {
	Name       string
	CommitDate time.Time
	Version    SemVersion
	HasVPrefix bool
}

type SemVersion struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string
	Build      string
}

var semverTagPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+([0-9A-Za-z.-]+))?$`)

// ListSemVerTags returns every SemVer tag in the repository at repoRoot,
// optionally scoped by tagPrefix, ordered from oldest to newest by SemVer.
// When git is unavailable or the repository has no matching tags, an empty
// slice and no error are returned.
func ListSemVerTags(repoRoot string, tagPrefix string) ([]Tag, error) {
	raw, err := runGit(repoRoot, "tag", "-l",
		"--format=%(refname:short)")
	if err != nil {
		return nil, nil // git unavailable — caller proceeds without history
	}

	var tags []Tag
	for _, line := range splitLines(raw) {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}

		if tagPrefix != "" && !strings.HasPrefix(name, tagPrefix) {
			continue
		}

		versionPart := strings.TrimPrefix(name, tagPrefix)
		version, hasVPrefix, parseErr := ParseSemVersion(versionPart)
		if parseErr != nil {
			continue
		}

		dateStr, dateErr := runGit(repoRoot, "log", "--format=%aI", "-1", name)
		if dateErr != nil {
			continue
		}

		t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(dateStr))
		if parseErr != nil {
			continue
		}

		tags = append(tags, Tag{Name: name, CommitDate: t, Version: version, HasVPrefix: hasVPrefix})
	}

	sort.SliceStable(tags, func(i, j int) bool {
		cmp := CompareSemVersion(tags[i].Version, tags[j].Version)
		if cmp != 0 {
			return cmp < 0
		}
		return tags[i].CommitDate.Before(tags[j].CommitDate)
	})

	return tags, nil
}

func ParseSemVersion(value string) (SemVersion, bool, error) {
	trimmed := strings.TrimSpace(value)
	matches := semverTagPattern.FindStringSubmatch(trimmed)
	if len(matches) == 0 {
		return SemVersion{}, false, fmt.Errorf("invalid semver %q", value)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return SemVersion{}, false, err
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return SemVersion{}, false, err
	}
	patch, err := strconv.Atoi(matches[3])
	if err != nil {
		return SemVersion{}, false, err
	}

	return SemVersion{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: matches[4],
		Build:      matches[5],
	}, strings.HasPrefix(trimmed, "v"), nil
}

func (v SemVersion) String(withVPrefix bool) string {
	prefix := ""
	if withVPrefix {
		prefix = "v"
	}

	result := fmt.Sprintf("%s%d.%d.%d", prefix, v.Major, v.Minor, v.Patch)
	if v.PreRelease != "" {
		result += "-" + v.PreRelease
	}
	if v.Build != "" {
		result += "+" + v.Build
	}

	return result
}

func (v SemVersion) IsPreRelease() bool {
	return v.PreRelease != ""
}

func CompareSemVersion(a SemVersion, b SemVersion) int {
	if a.Major != b.Major {
		if a.Major < b.Major {
			return -1
		}
		return 1
	}
	if a.Minor != b.Minor {
		if a.Minor < b.Minor {
			return -1
		}
		return 1
	}
	if a.Patch != b.Patch {
		if a.Patch < b.Patch {
			return -1
		}
		return 1
	}

	// Stable has higher precedence than pre-release.
	if a.PreRelease == "" && b.PreRelease != "" {
		return 1
	}
	if a.PreRelease != "" && b.PreRelease == "" {
		return -1
	}
	if a.PreRelease == "" && b.PreRelease == "" {
		return 0
	}

	return comparePreRelease(a.PreRelease, b.PreRelease)
}

func comparePreRelease(a string, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		if i >= len(aParts) {
			return -1
		}
		if i >= len(bParts) {
			return 1
		}

		aPart := aParts[i]
		bPart := bParts[i]

		aNum, aErr := strconv.Atoi(aPart)
		bNum, bErr := strconv.Atoi(bPart)
		aIsNumeric := aErr == nil
		bIsNumeric := bErr == nil

		if aIsNumeric && bIsNumeric {
			if aNum < bNum {
				return -1
			}
			if aNum > bNum {
				return 1
			}
			continue
		}

		if aIsNumeric && !bIsNumeric {
			return -1
		}
		if !aIsNumeric && bIsNumeric {
			return 1
		}

		if aPart < bPart {
			return -1
		}
		if aPart > bPart {
			return 1
		}
	}

	return 0
}

// FileAddedAt returns the author date of the commit that first introduced the
// file at path in the repository at repoRoot.  The path may be absolute; it is
// made relative to repoRoot before being passed to git so that --follow works
// correctly.  The second return value is false when the file has no recorded
// git history (untracked, or git unavailable).
func FileAddedAt(repoRoot, path string) (time.Time, bool) {
	relPath, relErr := filepath.Rel(repoRoot, path)
	if relErr != nil {
		relPath = path
	}

	// --diff-filter=A   : only commits where the file was *added*
	// --follow           : follow renames back to the original creation
	// Newest commit first; we want the last line (the original add).
	raw, err := runGit(repoRoot,
		"log", "--diff-filter=A", "--follow", "--format=%aI", "--", relPath)
	if err != nil {
		return time.Time{}, false
	}

	lines := splitLines(strings.TrimSpace(raw))
	if len(lines) == 0 {
		return time.Time{}, false
	}

	t, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(lines[len(lines)-1]))
	if parseErr != nil {
		return time.Time{}, false
	}

	return t, true
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}
