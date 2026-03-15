package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkChangesDir creates a directory at the given repo-root-relative path.
func mkChangesDir(t *testing.T, root, relPath string) {
	t.Helper()
	dir := filepath.Join(root, filepath.FromSlash(relPath))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir %s: %v", relPath, err)
	}
}

func TestResolveModuleInfosEmpty(t *testing.T) {
	root := makeGitRepo(t)

	infos, err := resolveModuleInfos(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("expected empty infos, got %d", len(infos))
	}
}

func TestResolveModuleInfosSingleRootModule(t *testing.T) {
	root := makeGitRepo(t)
	mkChangesDir(t, root, ".changes")

	infos, err := resolveModuleInfos(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 module, got %d", len(infos))
	}

	m := infos[0]
	if m.ChangesDir != ".changes" {
		t.Fatalf("expected changes_dir '.changes', got %q", m.ChangesDir)
	}
	// Root module has empty tag prefix.
	if m.TagPrefix != "" {
		t.Fatalf("expected empty tag prefix for root module, got %q", m.TagPrefix)
	}
}

func TestResolveModuleInfosMultiModule(t *testing.T) {
	root := makeGitRepo(t)
	mkChangesDir(t, root, "api/.changes")
	mkChangesDir(t, root, "worker/.changes")

	infos, err := resolveModuleInfos(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(infos))
	}

	// Build a name→info map for order-independent assertions.
	byName := make(map[string]ModuleInfo)
	for _, m := range infos {
		byName[m.Name] = m
	}

	api, ok := byName["api"]
	if !ok {
		t.Fatalf("expected module 'api' in results, got: %+v", infos)
	}
	if api.ChangesDir != "api/.changes" {
		t.Fatalf("expected api changes_dir 'api/.changes', got %q", api.ChangesDir)
	}

	worker, ok := byName["worker"]
	if !ok {
		t.Fatalf("expected module 'worker' in results, got: %+v", infos)
	}
	if worker.ChangesDir != "worker/.changes" {
		t.Fatalf("expected worker changes_dir 'worker/.changes', got %q", worker.ChangesDir)
	}
}

func TestResolveModuleInfosPathUsesForwardSlashes(t *testing.T) {
	root := makeGitRepo(t)
	mkChangesDir(t, root, "services/api/.changes")

	infos, err := resolveModuleInfos(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 module, got %d", len(infos))
	}

	if strings.Contains(infos[0].ChangesDir, "\\") {
		t.Fatalf("expected forward slashes in path, got: %q", infos[0].ChangesDir)
	}
	if infos[0].ChangesDir != "services/api/.changes" {
		t.Fatalf("expected 'services/api/.changes', got %q", infos[0].ChangesDir)
	}
}

func TestRenderModulesTable(t *testing.T) {
	infos := []ModuleInfo{
		{Name: "api", ChangesDir: "api/.changes", TagPrefix: "api-"},
		{Name: "worker", ChangesDir: "worker/.changes", TagPrefix: "worker-"},
	}

	var buf bytes.Buffer
	if err := renderModulesTable(infos, &buf); err != nil {
		t.Fatalf("renderModulesTable error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "NAME") {
		t.Fatalf("expected header 'NAME', got:\n%s", out)
	}
	if !strings.Contains(out, "api") || !strings.Contains(out, "worker") {
		t.Fatalf("expected both module names in output, got:\n%s", out)
	}
	if !strings.Contains(out, "api-") || !strings.Contains(out, "worker-") {
		t.Fatalf("expected tag prefixes in output, got:\n%s", out)
	}
}

func TestRenderModulesJSON(t *testing.T) {
	infos := []ModuleInfo{
		{Name: "api", ChangesDir: "api/.changes", TagPrefix: "api-"},
	}

	var buf bytes.Buffer
	if err := renderModulesJSON(infos, &buf); err != nil {
		t.Fatalf("renderModulesJSON error: %v", err)
	}

	var decoded []ModuleInfo
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(decoded))
	}
	if decoded[0].Name != "api" {
		t.Fatalf("expected name 'api', got %q", decoded[0].Name)
	}
	if decoded[0].ChangesDir != "api/.changes" {
		t.Fatalf("expected changes_dir 'api/.changes', got %q", decoded[0].ChangesDir)
	}
	if decoded[0].TagPrefix != "api-" {
		t.Fatalf("expected tag_prefix 'api-', got %q", decoded[0].TagPrefix)
	}
}

func TestRenderModulesJSONEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := renderModulesJSON([]ModuleInfo{}, &buf); err != nil {
		t.Fatalf("renderModulesJSON error: %v", err)
	}

	var decoded []ModuleInfo
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("expected empty array, got %d entries", len(decoded))
	}
}
