package commands

import (
	"testing"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/codested/chagg/internal/semver"
)

func TestBumpVersionPatch(t *testing.T) {
	version := semver.Bump(semver.SemVersion{Major: 1, Minor: 2, Patch: 3}, semver.BumpPatch)

	if version.String(true) != "v1.2.4" {
		t.Fatalf("expected v1.2.4, got %s", version.String(true))
	}
}

func TestBumpVersionMinor(t *testing.T) {
	version := semver.Bump(semver.SemVersion{Major: 1, Minor: 2, Patch: 3}, semver.BumpMinor)

	if version.String(false) != "1.3.0" {
		t.Fatalf("expected 1.3.0, got %s", version.String(false))
	}
}

func TestBumpVersionMajor(t *testing.T) {
	version := semver.Bump(semver.SemVersion{Major: 1, Minor: 2, Patch: 3}, semver.BumpMajor)

	if version.String(true) != "v2.0.0" {
		t.Fatalf("expected v2.0.0, got %s", version.String(true))
	}
}

func TestLatestStableTagPrefersStableOverPreRelease(t *testing.T) {
	tags := []semver.Tag{
		{Name: "v1.2.0-beta.1", Version: semver.SemVersion{Major: 1, Minor: 2, Patch: 0, PreRelease: "beta.1"}},
		{Name: "v1.1.0", Version: semver.SemVersion{Major: 1, Minor: 1, Patch: 0}},
	}

	tag, ok := semver.LatestStable(tags)
	if !ok {
		t.Fatalf("expected a stable tag")
	}

	if tag.Name != "v1.1.0" {
		t.Fatalf("expected v1.1.0, got %s", tag.Name)
	}
}

func TestDetectBumpLevelMajorForBumpOverride(t *testing.T) {
	group := changelog.VersionGroup{
		TypeGroups: []changelog.TypeGroup{{
			Entries: []changelog.EntryWithMeta{{
				Entry: changeentry.Entry{Type: changeentry.ChangeTypeFix, Bump: changeentry.BumpLevelMajor},
			}},
		}},
	}

	if level := detectBumpLevel(group, changeentry.DefaultTypeRegistry()); level != semver.BumpMajor {
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

	if level := detectBumpLevel(group, changeentry.DefaultTypeRegistry()); level != semver.BumpMinor {
		t.Fatalf("expected minor bump, got %d", level)
	}
}

func TestDetectBumpLevelMinorForRemoval(t *testing.T) {
	group := changelog.VersionGroup{
		TypeGroups: []changelog.TypeGroup{{
			Entries: []changelog.EntryWithMeta{{
				Entry: changeentry.Entry{Type: changeentry.ChangeTypeRemoval},
			}},
		}},
	}

	if level := detectBumpLevel(group, changeentry.DefaultTypeRegistry()); level != semver.BumpMinor {
		t.Fatalf("expected minor bump for removal (type default), got %d", level)
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

	if level := detectBumpLevel(group, changeentry.DefaultTypeRegistry()); level != semver.BumpPatch {
		t.Fatalf("expected patch bump, got %d", level)
	}
}

func TestResolveReleaseModeDefaultsToWriteMode(t *testing.T) {
	mode, err := resolveReleaseMode(ReleaseCommand())
	if err != nil {
		t.Fatalf("resolveReleaseMode returned error: %v", err)
	}

	if !mode.willCreateTag || mode.dryRun || mode.versionOnly || mode.pushTag {
		t.Fatalf("unexpected default mode: %+v", mode)
	}
}

func TestResolveReleaseModeRejectsInvalidCombinations(t *testing.T) {
	cmd := ReleaseCommand()
	if err := cmd.Set("dry-run", "true"); err != nil {
		t.Fatalf("set dry-run: %v", err)
	}
	if err := cmd.Set("push", "true"); err != nil {
		t.Fatalf("set push: %v", err)
	}

	_, err := resolveReleaseMode(cmd)
	if err == nil {
		t.Fatalf("expected error")
	}

	validationErr, ok := err.(*changeentry.ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if validationErr.Field != "flags" {
		t.Fatalf("expected flags validation error, got %q", validationErr.Field)
	}
}

func TestResolveReleaseModeVersionOnly(t *testing.T) {
	cmd := ReleaseCommand()
	if err := cmd.Set("version-only", "true"); err != nil {
		t.Fatalf("set version-only: %v", err)
	}

	mode, err := resolveReleaseMode(cmd)
	if err != nil {
		t.Fatalf("resolveReleaseMode returned error: %v", err)
	}

	if !mode.versionOnly || mode.willCreateTag {
		t.Fatalf("unexpected mode: %+v", mode)
	}
	if mode.requiresGitWrites() {
		t.Fatalf("version-only should not require git writes")
	}
}

func TestResolveReleaseModePush(t *testing.T) {
	cmd := ReleaseCommand()
	if err := cmd.Set("push", "true"); err != nil {
		t.Fatalf("set push: %v", err)
	}

	mode, err := resolveReleaseMode(cmd)
	if err != nil {
		t.Fatalf("resolveReleaseMode returned error: %v", err)
	}

	if !mode.pushTag || !mode.willCreateTag {
		t.Fatalf("unexpected mode: %+v", mode)
	}
	if !mode.requiresGitWrites() {
		t.Fatalf("push mode should require git writes")
	}
}

func TestConfigAutoPushSetsModePushTagWhenPolicyEnabled(t *testing.T) {
	cmd := ReleaseCommand()
	mode := releaseMode{willCreateTag: true}
	policy := changeentry.GitWritePolicy{Enabled: true, ReleasePush: true}

	applyConfigPushOverride(cmd, &mode, policy)

	if !mode.pushTag {
		t.Fatal("expected auto-push to be enabled from config")
	}
}

func TestConfigAutoPushDoesNotOverrideExplicitFlagFalse(t *testing.T) {
	cmd := ReleaseCommand()
	// Simulate --push=false explicitly set by user.
	if err := cmd.Set("push", "false"); err != nil {
		t.Fatalf("set push: %v", err)
	}
	mode := releaseMode{willCreateTag: true}
	policy := changeentry.GitWritePolicy{Enabled: true, ReleasePush: true}

	applyConfigPushOverride(cmd, &mode, policy)

	if mode.pushTag {
		t.Fatal("expected explicit --push=false to prevent auto-push")
	}
}

func TestConfigAutoPushDoesNothingWhenPolicyDisabled(t *testing.T) {
	cmd := ReleaseCommand()
	mode := releaseMode{willCreateTag: true}
	policy := changeentry.GitWritePolicy{Enabled: true, ReleasePush: false}

	applyConfigPushOverride(cmd, &mode, policy)

	if mode.pushTag {
		t.Fatal("expected no auto-push when ReleasePush is false")
	}
}

func TestConfigAutoPushDoesNothingInDryRun(t *testing.T) {
	cmd := ReleaseCommand()
	// willCreateTag is false in dry-run, so auto-push should not fire.
	mode := releaseMode{dryRun: true, willCreateTag: false}
	policy := changeentry.GitWritePolicy{Enabled: true, ReleasePush: true}

	applyConfigPushOverride(cmd, &mode, policy)

	if mode.pushTag {
		t.Fatal("expected no auto-push in dry-run mode")
	}
}
