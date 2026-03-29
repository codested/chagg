package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// ── IsWithinDir ───────────────────────────────────────────────────────────────

func TestIsWithinDirSameDir(t *testing.T) {
	if !IsWithinDir("/a/b", "/a/b") {
		t.Fatal("expected same dir to be within itself")
	}
}

func TestIsWithinDirChild(t *testing.T) {
	if !IsWithinDir("/a/b", "/a/b/c/d") {
		t.Fatal("expected child to be within parent")
	}
}

func TestIsWithinDirSibling(t *testing.T) {
	if IsWithinDir("/a/b", "/a/c") {
		t.Fatal("expected sibling to not be within dir")
	}
}

func TestIsWithinDirParent(t *testing.T) {
	if IsWithinDir("/a/b", "/a") {
		t.Fatal("expected parent to not be within child")
	}
}

func TestIsWithinDirDotDotPath(t *testing.T) {
	// A path that uses .. to escape should not be considered within parent.
	if IsWithinDir("/a/b", "/a/b/../c") {
		t.Fatal("expected path escaping via .. to not be within dir")
	}
}

func TestIsWithinDirTrailingSlash(t *testing.T) {
	if !IsWithinDir("/a/b/", "/a/b/c") {
		t.Fatal("expected trailing slash to be handled correctly")
	}
}

// ── FindGitRoot ───────────────────────────────────────────────────────────────

func TestFindGitRootFindsRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	found, hasGit, err := FindGitRoot(sub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !hasGit {
		t.Fatal("expected hasGit=true")
	}
	// Resolve symlinks for macOS /var → /private/var comparison.
	realRoot, _ := filepath.EvalSymlinks(root)
	realFound, _ := filepath.EvalSymlinks(found)
	if realFound != realRoot {
		t.Fatalf("expected git root %q, got %q", realRoot, realFound)
	}
}

func TestFindGitRootNotFound(t *testing.T) {
	dir := t.TempDir() // no .git
	_, hasGit, err := FindGitRoot(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasGit {
		t.Fatal("expected hasGit=false when no .git present")
	}
}

// ── FindAllChangesDirs ────────────────────────────────────────────────────────

func TestFindAllChangesDirsEmpty(t *testing.T) {
	root := t.TempDir()
	dirs, err := FindAllChangesDirs(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 0 {
		t.Fatalf("expected no dirs, got %v", dirs)
	}
}

func TestFindAllChangesDirsRootOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".changes"), 0o755); err != nil {
		t.Fatalf("create .changes: %v", err)
	}

	dirs, err := FindAllChangesDirs(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 1 {
		t.Fatalf("expected 1 dir, got %v", dirs)
	}
}

func TestFindAllChangesDirsMultiple(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"api/.changes", "worker/.changes"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(sub)), 0o755); err != nil {
			t.Fatalf("create %s: %v", sub, err)
		}
	}

	dirs, err := FindAllChangesDirs(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 2 {
		t.Fatalf("expected 2 dirs, got %v", dirs)
	}
}

// ── HasAbbreviatedVersionTags ─────────────────────────────────────────────────

// initGitRepoWithTags creates a temporary git repo with the given tags and
// returns the root path. The test is skipped if git is unavailable.
func initGitRepoWithTags(t *testing.T, tags []string) string {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git %v: %v: %s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "test")
	run("commit", "--allow-empty", "-m", "init")
	for _, tag := range tags {
		run("tag", tag)
	}
	return dir
}

func TestHasAbbreviatedVersionTagsEmpty(t *testing.T) {
	dir := initGitRepoWithTags(t, nil)
	got, err := HasAbbreviatedVersionTags(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Fatal("expected false for repo with no tags")
	}
}

func TestHasAbbreviatedVersionTagsFullSemverOnly(t *testing.T) {
	dir := initGitRepoWithTags(t, []string{"v1.2.3", "v1.3.0"})
	got, err := HasAbbreviatedVersionTags(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Fatal("expected false when only full semver tags exist")
	}
}

func TestHasAbbreviatedVersionTagsWithAbbreviated(t *testing.T) {
	dir := initGitRepoWithTags(t, []string{"v1.2.3", "v1.2", "v1"})
	got, err := HasAbbreviatedVersionTags(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Fatal("expected true when abbreviated tags exist")
	}
}

func TestHasAbbreviatedVersionTagsWithPrefix(t *testing.T) {
	dir := initGitRepoWithTags(t, []string{"myaction-v1.2.3", "myaction-v1.2", "myaction-v1", "other-v1.2.3"})
	got, err := HasAbbreviatedVersionTags(dir, "myaction-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Fatal("expected true when abbreviated tags with prefix exist")
	}
}

func TestHasAbbreviatedVersionTagsPrefixOnlyFullSemver(t *testing.T) {
	dir := initGitRepoWithTags(t, []string{"myaction-v1.2.3", "other-v1"})
	got, err := HasAbbreviatedVersionTags(dir, "myaction-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Fatal("expected false when only full semver tags match the prefix")
	}
}

func TestFindAllChangesDirsSkipsGit(t *testing.T) {
	root := t.TempDir()
	// .git/.changes should not be found.
	if err := os.MkdirAll(filepath.Join(root, ".git", ".changes"), 0o755); err != nil {
		t.Fatalf("create .git/.changes: %v", err)
	}

	dirs, err := FindAllChangesDirs(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dirs) != 0 {
		t.Fatalf("expected 0 dirs (git dir skipped), got %v", dirs)
	}
}
