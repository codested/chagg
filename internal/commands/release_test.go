package commands

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/changelog"
	"github.com/urfave/cli/v3"
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

func TestReleaseActionReturnsArchiveValidationErrorWhenArchiveWasMoved(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")

	mustInitReleaseGitRepo(t, repoDir)

	archiveV1Dir := filepath.Join(repoDir, ".changes", "archive", "v1.0.0")
	archiveV2Dir := filepath.Join(repoDir, ".changes", "archive", "v2.0.0")
	if err := os.MkdirAll(archiveV1Dir, 0o755); err != nil {
		t.Fatalf("mkdir archive v1: %v", err)
	}

	entryPath := filepath.Join(archiveV1Dir, "entry.md")
	entryContent := "---\ntype: fix\n---\n\nArchived entry\n"
	if err := os.WriteFile(entryPath, []byte(entryContent), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}
	mustGitAddCommitRelease(t, repoDir, "add archived entry")

	if err := os.MkdirAll(archiveV2Dir, 0o755); err != nil {
		t.Fatalf("mkdir archive v2: %v", err)
	}
	movedPath := filepath.Join(archiveV2Dir, "entry.md")
	if err := os.Rename(entryPath, movedPath); err != nil {
		t.Fatalf("move archive entry: %v", err)
	}
	mustGitAddCommitRelease(t, repoDir, "move archived entry")

	originalCwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() {
		if chdirErr := os.Chdir(originalCwd); chdirErr != nil {
			t.Fatalf("restore cwd: %v", chdirErr)
		}
	})
	if err := os.Chdir(repoDir); err != nil {
		t.Fatalf("chdir repo: %v", err)
	}

	err = releaseAction(context.Background(), &cli.Command{})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr *changeentry.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "archive" {
		t.Fatalf("expected field archive, got %q", validationErr.Field)
	}
}

func mustInitReleaseGitRepo(t *testing.T, repoDir string) {
	t.Helper()

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	if err := runGitCommand(repoDir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if err := runGitCommand(repoDir, "config", "user.name", "chagg-test"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if err := runGitCommand(repoDir, "config", "user.email", "chagg-test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
}

func mustGitAddCommitRelease(t *testing.T, repoDir string, message string) {
	t.Helper()

	if err := runGitCommand(repoDir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGitCommand(repoDir, "commit", "-m", message); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func runGitCommand(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		message := strings.TrimSpace(string(out))
		if message == "" {
			message = err.Error()
		}
		return errors.New(message)
	}

	return nil
}
