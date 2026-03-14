package changelog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/codested/chagg/internal/changeentry"
)

const validEntryContent = "---\ntype: fix\n---\n\nArchived change\n"

func TestLoadChangeLogReturnsValidationErrorForInvalidEntries(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	invalidEntry := filepath.Join(changesDir, "bad.md")
	content := "---\ntype: fix---\n\nBroken entry"
	if err := os.WriteFile(invalidEntry, []byte(content), 0o644); err != nil {
		t.Fatalf("write invalid entry: %v", err)
	}

	_, err := LoadChangeLog(repoDir, changeentry.ModuleConfig{Name: "default", ChangesDir: changesDir}, FilterOptions{})
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr *changeentry.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "changes" {
		t.Fatalf("expected field changes, got %q", validationErr.Field)
	}
}

func TestLoadChangeLogReturnsValidationErrorForArchiveDirectoryMoves(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	mustInitGitRepo(t, repoDir)

	archiveV1Dir := filepath.Join(changesDir, "archive", "v1.0.0")
	archiveV2Dir := filepath.Join(changesDir, "archive", "v2.0.0")
	if err := os.MkdirAll(archiveV1Dir, 0o755); err != nil {
		t.Fatalf("mkdir archive v1: %v", err)
	}

	originalPath := filepath.Join(archiveV1Dir, "entry.md")
	if err := os.WriteFile(originalPath, []byte(validEntryContent), 0o644); err != nil {
		t.Fatalf("write archived entry: %v", err)
	}
	mustGitAddCommit(t, repoDir, "add archived entry")

	if err := os.MkdirAll(archiveV2Dir, 0o755); err != nil {
		t.Fatalf("mkdir archive v2: %v", err)
	}
	movedPath := filepath.Join(archiveV2Dir, "entry.md")
	if err := os.Rename(originalPath, movedPath); err != nil {
		t.Fatalf("move archived entry: %v", err)
	}
	mustGitAddCommit(t, repoDir, "move archived entry")

	_, err := LoadChangeLog(repoDir, changeentry.ModuleConfig{Name: "default", ChangesDir: changesDir}, FilterOptions{})
	if err == nil {
		t.Fatalf("expected archive validation error")
	}

	var validationErr *changeentry.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.Field != "archive" {
		t.Fatalf("expected field archive, got %q", validationErr.Field)
	}
}

func TestLoadChangeLogAllowsArchiveContentEdits(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	mustInitGitRepo(t, repoDir)

	archiveV1Dir := filepath.Join(changesDir, "archive", "v1.0.0")
	if err := os.MkdirAll(archiveV1Dir, 0o755); err != nil {
		t.Fatalf("mkdir archive v1: %v", err)
	}

	entryPath := filepath.Join(archiveV1Dir, "entry.md")
	if err := os.WriteFile(entryPath, []byte(validEntryContent), 0o644); err != nil {
		t.Fatalf("write archived entry: %v", err)
	}
	mustGitAddCommit(t, repoDir, "add archived entry")

	updatedContent := "---\ntype: fix\n---\n\nUpdated archived body\n"
	if err := os.WriteFile(entryPath, []byte(updatedContent), 0o644); err != nil {
		t.Fatalf("update archived entry: %v", err)
	}
	mustGitAddCommit(t, repoDir, "edit archived entry")

	cl, err := LoadChangeLog(repoDir, changeentry.ModuleConfig{Name: "default", ChangesDir: changesDir}, FilterOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(cl.Groups) != 1 || cl.Groups[0].Version != "v1.0.0" {
		t.Fatalf("expected one archive group v1.0.0, got %+v", cl.Groups)
	}
}

func mustInitGitRepo(t *testing.T, repoDir string) {
	t.Helper()

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	if _, err := runGit(repoDir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := runGit(repoDir, "config", "user.name", "chagg-test"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if _, err := runGit(repoDir, "config", "user.email", "chagg-test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
}

func mustGitAddCommit(t *testing.T, repoDir string, message string) {
	t.Helper()

	if _, err := runGit(repoDir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := runGit(repoDir, "commit", "-m", message); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}
