package changelog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/gitutil"
)

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

func TestLoadChangeLogUsesOriginalAddCommitForMovedFiles(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	mustInitGitRepo(t, repoDir)

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}

	originalPath := filepath.Join(changesDir, "feature__create-prototype.md")
	entry := "Create prototype\n"
	if err := os.WriteFile(originalPath, []byte(entry), 0o644); err != nil {
		t.Fatalf("write initial entry: %v", err)
	}
	mustGitAddCommit(t, repoDir, "add prototype entry")
	mustGitTag(t, repoDir, "v0.1.0")

	archivePath := filepath.Join(changesDir, "archive", "0.1.0", "feature__create-prototype.md")
	if err := os.MkdirAll(filepath.Dir(archivePath), 0o755); err != nil {
		t.Fatalf("mkdir archive target: %v", err)
	}
	if err := os.Rename(originalPath, archivePath); err != nil {
		t.Fatalf("move entry: %v", err)
	}
	mustGitAddCommit(t, repoDir, "move prototype entry")
	mustGitTag(t, repoDir, "v0.2.2")

	cl, err := LoadChangeLog(repoDir, changeentry.ModuleConfig{Name: "default", ChangesDir: changesDir}, FilterOptions{})
	if err != nil {
		t.Fatalf("LoadChangeLog returned error: %v", err)
	}

	if len(cl.Groups) != 1 {
		t.Fatalf("expected exactly one group, got %d", len(cl.Groups))
	}

	if cl.Groups[0].Version != "v0.1.0" {
		t.Fatalf("expected moved file to stay on v0.1.0, got %q", cl.Groups[0].Version)
	}
}

func mustInitGitRepo(t *testing.T, repoDir string) {
	t.Helper()

	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	if _, err := gitutil.RunGit(repoDir, "init"); err != nil {
		t.Fatalf("git init: %v", err)
	}
	if _, err := gitutil.RunGit(repoDir, "config", "user.name", "chagg-test"); err != nil {
		t.Fatalf("git config user.name: %v", err)
	}
	if _, err := gitutil.RunGit(repoDir, "config", "user.email", "chagg-test@example.com"); err != nil {
		t.Fatalf("git config user.email: %v", err)
	}
}

func mustGitAddCommit(t *testing.T, repoDir string, message string) {
	t.Helper()

	if _, err := gitutil.RunGit(repoDir, "add", "."); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if _, err := gitutil.RunGit(repoDir, "commit", "-m", message); err != nil {
		t.Fatalf("git commit: %v", err)
	}
}

func mustGitTag(t *testing.T, repoDir string, tag string) {
	t.Helper()

	if _, err := gitutil.RunGit(repoDir, "tag", tag); err != nil {
		t.Fatalf("git tag %s: %v", tag, err)
	}
}
