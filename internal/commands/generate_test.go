package commands

import (
	"testing"

	"github.com/codested/chagg/internal/changeentry"
)

func TestNormalizeGenerateFormat(t *testing.T) {
	if value := normalizeGenerateFormat("md"); value != "markdown" {
		t.Fatalf("expected markdown, got %q", value)
	}
	if value := normalizeGenerateFormat("json"); value != "json" {
		t.Fatalf("expected json, got %q", value)
	}
}

func TestValidateGenerateFlagsAllowsDefaults(t *testing.T) {
	cmd := GenerateCommand()
	if err := validateGenerateFlags(cmd); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateGenerateFlagsRejectsOnlyStagingWithSince(t *testing.T) {
	cmd := GenerateCommand()
	_ = cmd.Set("only-staging", "true")
	_ = cmd.Set("since", "v1.2.0")

	err := validateGenerateFlags(cmd)
	if err == nil {
		t.Fatalf("expected error")
	}
	if _, ok := err.(*changeentry.ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestValidateGenerateFlagsRejectsOnlyStagingWithN(t *testing.T) {
	cmd := GenerateCommand()
	_ = cmd.Set("only-staging", "true")
	_ = cmd.Set("n", "2")

	err := validateGenerateFlags(cmd)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateGenerateFlagsRejectsOnlyStagingWithNoShowStaging(t *testing.T) {
	cmd := GenerateCommand()
	_ = cmd.Set("only-staging", "true")
	_ = cmd.Set("show-staging", "false")

	err := validateGenerateFlags(cmd)
	if err == nil {
		t.Fatalf("expected error")
	}
}
