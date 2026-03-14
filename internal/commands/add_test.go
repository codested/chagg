package commands

import "testing"

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
