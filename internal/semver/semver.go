package semver

import (
	"fmt"
	"regexp"
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

const (
	BumpPatch = iota
	BumpMinor
	BumpMajor
)

func Bump(v SemVersion, level int) SemVersion {
	next := SemVersion{Major: v.Major, Minor: v.Minor, Patch: v.Patch}

	switch level {
	case BumpMajor:
		next.Major++
		next.Minor = 0
		next.Patch = 0
	case BumpMinor:
		next.Minor++
		next.Patch = 0
	default:
		next.Patch++
	}

	return next
}

func LatestStable(tags []Tag) (Tag, bool) {
	for i := len(tags) - 1; i >= 0; i-- {
		if !tags[i].Version.IsPreRelease() {
			return tags[i], true
		}
	}

	return Tag{}, false
}
