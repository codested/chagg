package changeentry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveModuleForChangesDirSupportsChaggYml(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	config := "modules:\n  - name: default\n    changesDir: .changes\n"
	if err := os.WriteFile(filepath.Join(repoDir, "chagg.yml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if module.Name != "default" {
		t.Fatalf("expected module name default, got %q", module.Name)
	}
}

func TestResolveModuleForChangesDirAppliesGitWriteDefaultsAndOverrides(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	config := "defaults:\n  autoAddToGit: false\ngitWrite:\n  enabled: true\n  add: false\n  releaseTag: true\n  releasePush: false\nmodules:\n  - name: default\n    changesDir: .changes\n    defaults:\n      autoAddToGit: true\n    gitWrite:\n      releasePush: true\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".chagg.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if !module.Defaults.AutoAddToGit {
		t.Fatalf("expected module default autoAddToGit to be true")
	}
	if module.GitWrite.Add {
		t.Fatalf("expected gitWrite.add to inherit false")
	}
	if !module.GitWrite.ReleaseTag {
		t.Fatalf("expected gitWrite.releaseTag to be true")
	}
	if !module.GitWrite.ReleasePush {
		t.Fatalf("expected module override gitWrite.releasePush to be true")
	}
}

func TestResolveModuleForChangesDirFailsWhenMultipleConfigFilesExist(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	content := "modules:\n  - name: default\n    changesDir: .changes\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".chagg.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write .chagg.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "chagg.yml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write chagg.yml: %v", err)
	}

	_, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err == nil {
		t.Fatalf("expected error")
	}
}
