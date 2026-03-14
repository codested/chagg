package commands

import (
	"testing"

	"github.com/codested/chagg/internal/changeentry"
)

func TestAddCommandContainsExpectedNameAndAlias(t *testing.T) {
	cmd := AddCommand()

	if cmd.Name != "add" {
		t.Fatalf("expected command name add, got %q", cmd.Name)
	}

	if len(cmd.Aliases) != 1 || cmd.Aliases[0] != "a" {
		t.Fatalf("expected alias a, got %v", cmd.Aliases)
	}

	if cmd.Action == nil {
		t.Fatalf("expected add command action to be set")
	}
}

func TestResolveGitAddBehaviorDefaultsToModuleConfig(t *testing.T) {
	cmd := AddCommand()

	value, err := resolveGitAddBehavior(cmd, changeentry.ModuleConfig{Defaults: changeentry.ModuleDefaults{AutoAddToGit: true}})
	if err != nil {
		t.Fatalf("resolveGitAddBehavior returned error: %v", err)
	}
	if !value {
		t.Fatalf("expected module default true")
	}
}

func TestResolveGitAddBehaviorNoGitAddOverridesDefault(t *testing.T) {
	cmd := AddCommand()
	if err := cmd.Set("no-git-add", "true"); err != nil {
		t.Fatalf("set flag: %v", err)
	}

	value, err := resolveGitAddBehavior(cmd, changeentry.ModuleConfig{Defaults: changeentry.ModuleDefaults{AutoAddToGit: true}})
	if err != nil {
		t.Fatalf("resolveGitAddBehavior returned error: %v", err)
	}
	if value {
		t.Fatalf("expected no-git-add to disable staging")
	}
}
