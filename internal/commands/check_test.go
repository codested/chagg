package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEntry(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write entry %s: %v", name, err)
	}
	return path
}

func TestCheckAllNoEntries(t *testing.T) {
	root := makeGitRepo(t)
	if err := checkAll(root, root); err != nil {
		t.Fatalf("expected no error for empty repo: %v", err)
	}
}

func TestCheckAllValidEntry(t *testing.T) {
	root := makeGitRepo(t)
	changesDir := filepath.Join(root, ".changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("create .changes: %v", err)
	}
	writeEntry(t, changesDir, "feature__test.md", "---\ntype: feature\n---\nA feature.\n")

	if err := checkAll(root, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCheckAllInvalidEntry(t *testing.T) {
	root := makeGitRepo(t)
	changesDir := filepath.Join(root, ".changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("create .changes: %v", err)
	}
	// Invalid: filename type prefix "notatype" is not a registered type.
	writeEntry(t, changesDir, "notatype__bad.md", "Bad entry with unknown type prefix in filename.\n")

	err := checkAll(root, root)
	if err == nil {
		t.Fatal("expected error for invalid entry")
	}
}

func TestCheckFilesGlobExpansion(t *testing.T) {
	root := makeGitRepo(t)
	changesDir := filepath.Join(root, ".changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("create .changes: %v", err)
	}

	writeEntry(t, changesDir, "feature__a.md", "---\ntype: feature\n---\nA.\n")
	writeEntry(t, changesDir, "feature__b.md", "---\ntype: feature\n---\nB.\n")

	// Change to changesDir to make glob work.
	old, _ := os.Getwd()
	defer os.Chdir(old) //nolint:errcheck
	os.Chdir(changesDir) //nolint:errcheck

	if err := checkFiles([]string{"*.md"}, root, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFindContainingChangesDir(t *testing.T) {
	root := makeGitRepo(t)
	changesDir := filepath.Join(root, ".changes")

	filePath := filepath.Join(changesDir, "feature__x.md")
	got := findContainingChangesDir(filePath, root)
	if got != changesDir {
		t.Fatalf("expected %q, got %q", changesDir, got)
	}
}

func TestCheckAllShowsVersionColumn(t *testing.T) {
	root := makeGitRepo(t)
	changesDir := filepath.Join(root, ".changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("create .changes: %v", err)
	}
	writeEntry(t, changesDir, "feature__v.md", "---\ntype: feature\n---\nVersion test.\n")

	// Just verify no error; output goes to stdout.
	if err := checkAll(root, root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
