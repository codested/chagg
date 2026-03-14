package commands

import (
	"testing"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
)

func TestBumpVersionPatch(t *testing.T) {
	version := bumpVersion(changelog.SemVersion{Major: 1, Minor: 2, Patch: 3}, bumpPatch)

	if version.String(true) != "v1.2.4" {
		t.Fatalf("expected v1.2.4, got %s", version.String(true))
	}
}

func TestBumpVersionMinor(t *testing.T) {
	version := bumpVersion(changelog.SemVersion{Major: 1, Minor: 2, Patch: 3}, bumpMinor)

	if version.String(false) != "1.3.0" {
		t.Fatalf("expected 1.3.0, got %s", version.String(false))
	}
}

func TestBumpVersionMajor(t *testing.T) {
	version := bumpVersion(changelog.SemVersion{Major: 1, Minor: 2, Patch: 3}, bumpMajor)

	if version.String(true) != "v2.0.0" {
		t.Fatalf("expected v2.0.0, got %s", version.String(true))
	}
}

func TestLatestStableTagPrefersStableOverPreRelease(t *testing.T) {
	tags := []changelog.Tag{
		{Name: "v1.2.0-beta.1", Version: changelog.SemVersion{Major: 1, Minor: 2, Patch: 0, PreRelease: "beta.1"}},
		{Name: "v1.1.0", Version: changelog.SemVersion{Major: 1, Minor: 1, Patch: 0}},
	}

	tag, ok := latestStableTag(tags)
	if !ok {
		t.Fatalf("expected a stable tag")
	}

	if tag.Name != "v1.1.0" {
		t.Fatalf("expected v1.1.0, got %s", tag.Name)
	}
}

func TestDetectBumpLevelMajorForBreaking(t *testing.T) {
	group := changelog.VersionGroup{
		TypeGroups: []changelog.TypeGroup{{
			Entries: []changelog.EntryWithMeta{{
				Entry: changeentry.Entry{Type: changeentry.ChangeTypeFix, Breaking: true},
			}},
		}},
	}

	if level := detectBumpLevel(group); level != bumpMajor {
		t.Fatalf("expected major bump, got %d", level)
	}
}

func TestDetectBumpLevelMinorForFeature(t *testing.T) {
	group := changelog.VersionGroup{
		TypeGroups: []changelog.TypeGroup{{
			Entries: []changelog.EntryWithMeta{{
				Entry: changeentry.Entry{Type: changeentry.ChangeTypeFeature},
			}},
		}},
	}

	if level := detectBumpLevel(group); level != bumpMinor {
		t.Fatalf("expected minor bump, got %d", level)
	}
}

func TestDetectBumpLevelPatchForFix(t *testing.T) {
	group := changelog.VersionGroup{
		TypeGroups: []changelog.TypeGroup{{
			Entries: []changelog.EntryWithMeta{{
				Entry: changeentry.Entry{Type: changeentry.ChangeTypeFix},
			}},
		}},
	}

	if level := detectBumpLevel(group); level != bumpPatch {
		t.Fatalf("expected patch bump, got %d", level)
	}
}
