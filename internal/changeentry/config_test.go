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

	config := "modules:\n  - name: default\n    changes-dir: .changes\n"
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

func TestResolveModuleForChangesDirInferredTagPrefixUsesParentDirectory(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, "msal-react", ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if module.Name != "msal-react" {
		t.Fatalf("expected module name msal-react, got %q", module.Name)
	}
	if module.TagPrefix != "msal-react-" {
		t.Fatalf("expected inferred tag prefix msal-react-, got %q", module.TagPrefix)
	}
}

func TestResolveModuleForChangesDirRootTagPrefixIsEmpty(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if module.TagPrefix != "" {
		t.Fatalf("expected empty tag prefix for root .changes, got %q", module.TagPrefix)
	}
}

func TestResolveModuleForChangesDirAppliesGlobalGitWriteAllowBool(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	config := "git-write:\n  allow: false\nmodules:\n  - name: default\n    changes-dir: .changes\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".chagg.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if module.GitWrite.Enabled || module.GitWrite.Add || module.GitWrite.ReleaseTag || module.GitWrite.ReleasePush {
		t.Fatalf("expected allow:false to disable all git writes, got %+v", module.GitWrite)
	}
}

func TestResolveModuleForChangesDirReadsDefaultAudienceString(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	config := "default-audience: public\nmodules:\n  - changes-dir: .changes\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".chagg.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if len(module.DefaultAudience) != 1 || module.DefaultAudience[0] != "public" {
		t.Fatalf("expected default audience [public], got %#v", module.DefaultAudience)
	}
}

func TestResolveModuleForChangesDirReadsDefaultAudienceList(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	config := "default-audience:\n  - public\n  - developer\nmodules:\n  - changes-dir: .changes\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".chagg.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if len(module.DefaultAudience) != 2 || module.DefaultAudience[0] != "public" || module.DefaultAudience[1] != "developer" {
		t.Fatalf("expected default audience [public developer], got %#v", module.DefaultAudience)
	}
}

func TestResolveModuleForChangesDirInfersNameAndTagPrefixFromConfiguredChangesDir(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, "lib", "msal-browser", ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	config := "modules:\n  - changes-dir: lib/msal-browser/.changes\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".chagg.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if module.Name != "msal-browser" {
		t.Fatalf("expected inferred name msal-browser, got %q", module.Name)
	}
	if module.TagPrefix != "msal-browser-" {
		t.Fatalf("expected inferred tag prefix msal-browser-, got %q", module.TagPrefix)
	}
}

func TestResolveModuleForChangesDirFailsWhenInferredNamesCollideWithoutConfig(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	one := filepath.Join(repoDir, "a", "msal", ".changes")
	two := filepath.Join(repoDir, "b", "msal", ".changes")

	if err := os.MkdirAll(one, 0o755); err != nil {
		t.Fatalf("mkdir one: %v", err)
	}
	if err := os.MkdirAll(two, 0o755); err != nil {
		t.Fatalf("mkdir two: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	_, err := ResolveModuleForChangesDir(repoDir, one)
	if err == nil {
		t.Fatalf("expected collision error")
	}
}

func TestResolveModulesForChangesDirsFailsWhenInferredNamesCollideWithoutConfig(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	one := filepath.Join(repoDir, "a", "msal", ".changes")
	two := filepath.Join(repoDir, "b", "msal", ".changes")

	if err := os.MkdirAll(one, 0o755); err != nil {
		t.Fatalf("mkdir one: %v", err)
	}
	if err := os.MkdirAll(two, 0o755); err != nil {
		t.Fatalf("mkdir two: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	_, err := ResolveModulesForChangesDirs(repoDir, []string{one, two})
	if err == nil {
		t.Fatalf("expected collision error")
	}
}

func TestResolveModuleForChangesDirAppliesGlobalGitWriteAllowObject(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	config := "git-write:\n  allow:\n    add-change: false\n    push-release-tag: false\nmodules:\n  - name: default\n    changes-dir: .changes\n"
	if err := os.WriteFile(filepath.Join(repoDir, ".chagg.yaml"), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	module, err := ResolveModuleForChangesDir(repoDir, changesDir)
	if err != nil {
		t.Fatalf("ResolveModuleForChangesDir returned error: %v", err)
	}

	if !module.GitWrite.Enabled {
		t.Fatalf("expected allow object to keep writes enabled")
	}
	if module.GitWrite.Add {
		t.Fatalf("expected add-change to be false")
	}
	if !module.GitWrite.ReleaseTag {
		t.Fatalf("expected local tag creation to remain enabled")
	}
	if module.GitWrite.ReleasePush {
		t.Fatalf("expected push-release-tag to be false")
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

	content := "modules:\n  - name: default\n    changes-dir: .changes\n"
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
