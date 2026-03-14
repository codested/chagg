package commands

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTidyCommandContainsExpectedNameAndAction(t *testing.T) {
	cmd := TidyCommand()

	if cmd.Name != "tidy" {
		t.Fatalf("expected command name tidy, got %q", cmd.Name)
	}

	if cmd.Action == nil {
		t.Fatalf("expected tidy command action to be set")
	}
}

func TestSanitizeVersionDirNameReplacesSlashes(t *testing.T) {
	value := sanitizeVersionDirName("msal-browser/v1.2.3")
	if value != "msal-browser_v1.2.3" {
		t.Fatalf("expected msal-browser_v1.2.3, got %q", value)
	}
}

func TestDedupeTidyMovesRemovesDuplicateMoves(t *testing.T) {
	moves := []tidyMove{
		{ModuleName: "default", From: "a", To: "b"},
		{ModuleName: "default", From: "a", To: "b"},
		{ModuleName: "default", From: "c", To: "d"},
	}

	result := dedupeTidyMoves(moves)
	if len(result) != 2 {
		t.Fatalf("expected 2 moves, got %d", len(result))
	}
}

func TestArchiveTargetPathFlattensOriginalSubdirectories(t *testing.T) {
	changesDir := filepath.Join(string(filepath.Separator), "repo", ".changes")
	source := filepath.Join(changesDir, "0.x.x", "nested", "entry.md")

	target := archiveTargetPath(changesDir, "0.1.1", source)
	expected := filepath.Join(changesDir, "archive", "0.1.1", "entry.md")
	if target != expected {
		t.Fatalf("expected %q, got %q", expected, target)
	}
}

func TestIsAlreadyArchivedInVersion(t *testing.T) {
	changesDir := filepath.Join(string(filepath.Separator), "repo", ".changes")
	archived := filepath.Join(changesDir, "archive", "0.1.1", "entry.md")
	notArchived := filepath.Join(changesDir, "0.x.x", "entry.md")

	if !isAlreadyArchivedInVersion(changesDir, "0.1.1", archived) {
		t.Fatalf("expected archived path to be recognized")
	}

	if isAlreadyArchivedInVersion(changesDir, "0.1.1", notArchived) {
		t.Fatalf("expected non-archived path to be rejected")
	}
}

func TestPruneEmptyChangeDirectoriesRemovesEmptyNestedDirs(t *testing.T) {
	tempDir := t.TempDir()
	changesDir := filepath.Join(tempDir, ".changes")
	emptyNested := filepath.Join(changesDir, "legacy", "old")
	nonEmpty := filepath.Join(changesDir, "archive", "0.1.0")

	if err := os.MkdirAll(emptyNested, 0o755); err != nil {
		t.Fatalf("mkdir empty nested: %v", err)
	}
	if err := os.MkdirAll(nonEmpty, 0o755); err != nil {
		t.Fatalf("mkdir non-empty: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nonEmpty, "entry.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	if err := pruneEmptyChangeDirectories(changesDir); err != nil {
		t.Fatalf("pruneEmptyChangeDirectories returned error: %v", err)
	}

	if _, err := os.Stat(emptyNested); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, err=%v", emptyNested, err)
	}

	if _, err := os.Stat(nonEmpty); err != nil {
		t.Fatalf("expected non-empty dir to remain: %v", err)
	}
}
