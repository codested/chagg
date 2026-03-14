package changelog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/codested/chagg/internal/changeentry"
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
