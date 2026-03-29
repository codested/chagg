package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/codested/chagg/internal/changeentry"
)

// makeGitRepo creates a temporary directory with a .git sub-directory so that
// gitutil.FindGitRoot considers it a valid git repo root.
func makeGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("create .git: %v", err)
	}
	return dir
}

func runInitTest(t *testing.T, cwd string, cio changeentry.ConfigIO, input string, noPrompt bool) string {
	t.Helper()
	in := strings.NewReader(input)
	var out bytes.Buffer
	if err := runInit(cwd, cio, in, &out, noPrompt); err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}
	return out.String()
}

func runInitTestExpectError(t *testing.T, cwd string, cio changeentry.ConfigIO, input string, noPrompt bool) error {
	t.Helper()
	in := strings.NewReader(input)
	var out bytes.Buffer
	return runInit(cwd, cio, in, &out, noPrompt)
}

// ── outside git repo ──────────────────────────────────────────────────────────

func TestInitFailsOutsideGitRepo(t *testing.T) {
	dir := t.TempDir() // no .git
	err := runInitTestExpectError(t, dir, &changeentry.MockConfigIO{}, "", true)
	if err == nil {
		t.Fatal("expected error when outside a git repo")
	}
	if !strings.Contains(err.Error(), "git repository") {
		t.Fatalf("expected git error, got: %v", err)
	}
}

// ── single module from repo root ──────────────────────────────────────────────

func TestInitRepoRootSingleModuleCreatesChangesDir(t *testing.T) {
	root := makeGitRepo(t)
	mock := &changeentry.MockConfigIO{}

	// no-prompt, single module (default: not multi-module)
	out := runInitTest(t, root, mock, "", true)

	changesDir := filepath.Join(root, ".changes")
	if _, err := os.Stat(changesDir); err != nil {
		t.Fatalf(".changes directory not created: %v", err)
	}

	// Single module at root writes no config.
	if mock.WrittenRepoCfg != nil {
		t.Fatalf("expected no config written for single-module root, got: %+v", mock.WrittenRepoCfg)
	}

	if !strings.Contains(out, "Created") {
		t.Fatalf("expected 'Created' in output, got:\n%s", out)
	}
}

func TestInitRepoRootSingleModuleSkipsExistingDir(t *testing.T) {
	root := makeGitRepo(t)
	changesDir := filepath.Join(root, ".changes")
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("pre-create .changes: %v", err)
	}

	mock := &changeentry.MockConfigIO{}
	out := runInitTest(t, root, mock, "", true)

	if !strings.Contains(out, "already exists") {
		t.Fatalf("expected 'already exists' message, got:\n%s", out)
	}
}

// ── multi-module from repo root ───────────────────────────────────────────────

func TestInitRepoRootMultiModuleCreatesModuleDirsAndConfig(t *testing.T) {
	root := makeGitRepo(t)
	mock := &changeentry.MockConfigIO{}

	// Interactive: answer "y" to multi-module, then provide two modules.
	// Prompt order:
	//   "Is this a multi-module project?" → y
	//   module 1 name → api
	//   module 1 changes-dir → (default: api/.changes)
	//   module 1 tag-prefix → (default: api-)
	//   "Add another module?" → y
	//   module 2 name → worker
	//   module 2 changes-dir → (default: worker/.changes)
	//   module 2 tag-prefix → (default: worker-)
	//   "Add another module?" → n
	input := "y\napi\n\n\ny\nworker\n\n\nn\n"
	out := runInitTest(t, root, mock, input, false)

	// Both .changes dirs must exist.
	for _, sub := range []string{"api/.changes", "worker/.changes"} {
		dir := filepath.Join(root, filepath.FromSlash(sub))
		if _, err := os.Stat(dir); err != nil {
			t.Fatalf("expected %s to exist: %v", sub, err)
		}
	}

	// Config must have been written with two modules.
	if mock.WrittenRepoCfg == nil {
		t.Fatal("expected config to be written")
	}
	if len(mock.WrittenRepoCfg.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(mock.WrittenRepoCfg.Modules))
	}

	names := []string{mock.WrittenRepoCfg.Modules[0].Name, mock.WrittenRepoCfg.Modules[1].Name}
	if names[0] != "api" || names[1] != "worker" {
		t.Fatalf("unexpected module names: %v", names)
	}

	if !strings.Contains(out, "Wrote") {
		t.Fatalf("expected 'Wrote' in output, got:\n%s", out)
	}
}

func TestInitRepoRootMultiModuleSetsTagPrefixDefaults(t *testing.T) {
	root := makeGitRepo(t)
	mock := &changeentry.MockConfigIO{}

	// Accept all defaults for a single module named "svc".
	// y → multi-module, svc → name, Enter → default dir, Enter → default prefix, n → no more
	input := "y\nsvc\n\n\nn\n"
	runInitTest(t, root, mock, input, false)

	if mock.WrittenRepoCfg == nil || len(mock.WrittenRepoCfg.Modules) != 1 {
		t.Fatal("expected 1 module in config")
	}
	m := mock.WrittenRepoCfg.Modules[0]
	if m.TagPrefix != "svc-" {
		t.Fatalf("expected tag prefix svc-, got %q", m.TagPrefix)
	}
	if m.ChangesDir != "svc/.changes" {
		t.Fatalf("expected changes dir svc/.changes, got %q", m.ChangesDir)
	}
}

func TestInitRepoRootMultiModuleNoPendingModulesDoesNothing(t *testing.T) {
	root := makeGitRepo(t)
	mock := &changeentry.MockConfigIO{}

	// non-interactive, multi-module chosen but no modules can be collected
	// because ask("Name","") returns "" immediately in no-prompt mode.
	in := strings.NewReader("y\n") // "yes" for multi-module
	var out bytes.Buffer
	_ = runInit(root, mock, in, &out, true)

	if mock.WrittenRepoCfg != nil {
		t.Fatalf("expected no config written when no modules provided")
	}
}

// ── from sub-directory ────────────────────────────────────────────────────────

func TestInitFromSubdirCreatesModuleEntry(t *testing.T) {
	root := makeGitRepo(t)
	subDir := filepath.Join(root, "services", "api")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	mock := &changeentry.MockConfigIO{}
	// no-prompt: default "yes" to "init here as a module"
	out := runInitTest(t, subDir, mock, "", true)

	changesDir := filepath.Join(subDir, ".changes")
	if _, err := os.Stat(changesDir); err != nil {
		t.Fatalf(".changes not created in subdir: %v", err)
	}

	if mock.WrittenRepoCfg == nil {
		t.Fatal("expected config to be written for module")
	}
	if len(mock.WrittenRepoCfg.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(mock.WrittenRepoCfg.Modules))
	}
	m := mock.WrittenRepoCfg.Modules[0]
	// Name defaults to the last component of cwd.
	if m.Name != "api" {
		t.Fatalf("expected module name api, got %q", m.Name)
	}
	// ChangesDir should be relative to repo root using forward slashes.
	if !strings.HasSuffix(m.ChangesDir, ".changes") {
		t.Fatalf("expected changes dir to end with .changes, got %q", m.ChangesDir)
	}

	if !strings.Contains(out, "Wrote") {
		t.Fatalf("expected 'Wrote' in output, got:\n%s", out)
	}
}

func TestInitFromSubdirFallbackToRepoRoot(t *testing.T) {
	root := makeGitRepo(t)
	subDir := filepath.Join(root, "pkg", "lib")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	mock := &changeentry.MockConfigIO{}
	// Answer "n" to "init here" → falls back to repo-root single module (no-prompt).
	in := strings.NewReader("n\n")
	var out bytes.Buffer
	if err := runInit(subDir, mock, in, &out, false); err != nil {
		t.Fatalf("runInit error: %v", err)
	}

	// Single module at root: .changes created there, no config written.
	changesDir := filepath.Join(root, ".changes")
	if _, err := os.Stat(changesDir); err != nil {
		t.Fatalf(".changes not created at repo root: %v", err)
	}
	if mock.WrittenRepoCfg != nil {
		t.Fatalf("expected no config for single-module root, got: %+v", mock.WrittenRepoCfg)
	}
}

func TestInitFromSubdirReplacesExistingModuleEntry(t *testing.T) {
	root := makeGitRepo(t)
	subDir := filepath.Join(root, "api")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	existing := &changeentry.RawConfig{
		Modules: []changeentry.RawModule{
			{Name: "api", ChangesDir: "api/.changes", TagPrefix: "api-old-"},
		},
	}
	mock := &changeentry.MockConfigIO{RepoCfg: existing}

	// no-prompt: accept defaults, which will replace the existing "api" module.
	runInitTest(t, subDir, mock, "", true)

	if mock.WrittenRepoCfg == nil {
		t.Fatal("expected config to be written")
	}
	// Should still have exactly one module named "api".
	if len(mock.WrittenRepoCfg.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(mock.WrittenRepoCfg.Modules))
	}
}

// ── next-steps output ─────────────────────────────────────────────────────────

func TestInitPrintsNextStepsHints(t *testing.T) {
	root := makeGitRepo(t)
	mock := &changeentry.MockConfigIO{}
	out := runInitTest(t, root, mock, "", true)

	if !strings.Contains(out, "chagg add") {
		t.Fatalf("expected 'chagg add' hint in output, got:\n%s", out)
	}
	if !strings.Contains(out, "defaults.audience") {
		t.Fatalf("expected audience hint in output, got:\n%s", out)
	}
}
