package commands

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/codested/chagg/internal/changeentry"
)

// ── ConfigCommand structure ───────────────────────────────────────────────────

func TestConfigCommandHasExpectedNameAndAliases(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	cmd := ConfigCommand(mock)

	if cmd.Name != "config" {
		t.Fatalf("expected command name config, got %q", cmd.Name)
	}
	if len(cmd.Aliases) != 1 || cmd.Aliases[0] != "cfg" {
		t.Fatalf("expected alias cfg, got %v", cmd.Aliases)
	}
}

func TestConfigCommandHasTypesSubcommand(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	cmd := ConfigCommand(mock)

	if len(cmd.Commands) == 0 {
		t.Fatal("expected at least one subcommand")
	}
	found := false
	for _, sub := range cmd.Commands {
		if sub.Name == "types" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected subcommand named 'types'")
	}
}

func TestConfigCommandHasListGlobalAndUnsetFlags(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	cmd := ConfigCommand(mock)

	flagNames := map[string]bool{}
	for _, f := range cmd.Flags {
		for _, n := range f.Names() {
			flagNames[n] = true
		}
	}

	for _, required := range []string{"list", "l", "global", "unset"} {
		if !flagNames[required] {
			t.Fatalf("expected flag %q to be present", required)
		}
	}
}

// ── configSet (global scope) ──────────────────────────────────────────────────

func TestConfigSetGlobalDefaultsAudienceWritesUserConfig(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("defaults.audience", []string{"public"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if len(mock.WrittenUserCfg.Defaults.Audience) != 1 || string(mock.WrittenUserCfg.Defaults.Audience[0]) != "public" {
		t.Fatalf("expected audience [public], got %v", mock.WrittenUserCfg.Defaults.Audience)
	}
}

func TestConfigSetGlobalDefaultsAudienceExpandsCSV(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("defaults.audience", []string{"public,internal"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	got := []string(mock.WrittenUserCfg.Defaults.Audience)
	if len(got) != 2 || got[0] != "public" || got[1] != "internal" {
		t.Fatalf("expected audience [public internal], got %v", got)
	}
}

func TestConfigSetGlobalDefaultsRankWritesUserConfig(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("defaults.rank", []string{"5"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Defaults.Rank == nil || *mock.WrittenUserCfg.Defaults.Rank != 5 {
		t.Fatalf("expected rank 5, got %v", mock.WrittenUserCfg.Defaults.Rank)
	}
}

func TestConfigSetGlobalDefaultsRankRejectsNonInteger(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("defaults.rank", []string{"notanumber"}, true, mock)
	if err == nil {
		t.Fatal("expected error for non-integer rank")
	}
}

func TestConfigSetGlobalDefaultsRankRejectsMultipleValues(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("defaults.rank", []string{"1", "2"}, true, mock)
	if err == nil {
		t.Fatal("expected error for multiple values")
	}
}

func TestConfigSetGlobalGitWriteAllowWritesUserConfig(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("git.write.allow", []string{"false"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Git.Write.Allow == nil || *mock.WrittenUserCfg.Git.Write.Allow != false {
		t.Fatalf("expected git.write.allow=false, got %v", mock.WrittenUserCfg.Git.Write.Allow)
	}
}

func TestConfigSetGlobalGitWriteAllowRejectsInvalidBool(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("git.write.allow", []string{"yes"}, true, mock)
	if err == nil {
		t.Fatal("expected error for invalid boolean")
	}
}

func TestConfigSetGlobalGitWriteAddChangeWritesUserConfig(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("git.write.add-change", []string{"false"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Git.Write.Operations.AddChange == nil || *mock.WrittenUserCfg.Git.Write.Operations.AddChange != false {
		t.Fatalf("expected add-change=false, got %v", mock.WrittenUserCfg.Git.Write.Operations.AddChange)
	}
}

func TestConfigSetGlobalGitWriteCreateReleaseTagWritesUserConfig(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("git.write.create-release-tag", []string{"true"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Git.Write.Operations.CreateReleaseTag == nil || *mock.WrittenUserCfg.Git.Write.Operations.CreateReleaseTag != true {
		t.Fatalf("expected create-release-tag=true, got %v", mock.WrittenUserCfg.Git.Write.Operations.CreateReleaseTag)
	}
}

func TestConfigSetGlobalGitWritePushReleaseTagWritesUserConfig(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("git.write.push-release-tag", []string{"false"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Git.Write.Operations.PushReleaseTag == nil || *mock.WrittenUserCfg.Git.Write.Operations.PushReleaseTag != false {
		t.Fatalf("expected push-release-tag=false, got %v", mock.WrittenUserCfg.Git.Write.Operations.PushReleaseTag)
	}
}

func TestConfigSetGlobalPreservesExistingUserConfig(t *testing.T) {
	existingRank := 3
	mock := &changeentry.MockConfigIO{
		UserCfg: &changeentry.RawConfig{
			Defaults: changeentry.RawDefaults{
				Rank: &existingRank,
			},
		},
	}
	err := configSet("defaults.audience", []string{"public"}, true, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	// Rank should be preserved.
	if mock.WrittenUserCfg.Defaults.Rank == nil || *mock.WrittenUserCfg.Defaults.Rank != 3 {
		t.Fatalf("expected rank to be preserved as 3, got %v", mock.WrittenUserCfg.Defaults.Rank)
	}
	// Audience should be set.
	if len(mock.WrittenUserCfg.Defaults.Audience) != 1 || string(mock.WrittenUserCfg.Defaults.Audience[0]) != "public" {
		t.Fatalf("expected audience [public], got %v", mock.WrittenUserCfg.Defaults.Audience)
	}
}

func TestConfigSetUnknownKeyReturnsError(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("unknown.key", []string{"value"}, true, mock)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown config key") {
		t.Fatalf("expected 'unknown config key' in error, got %q", err.Error())
	}
}

// ── configUnset (global scope) ────────────────────────────────────────────────

func TestConfigUnsetGlobalDefaultsAudienceClearsField(t *testing.T) {
	audience := changeentry.StringListConfig([]string{"public"})
	mock := &changeentry.MockConfigIO{
		UserCfg: &changeentry.RawConfig{
			Defaults: changeentry.RawDefaults{
				Audience: audience,
			},
		},
	}
	err := configUnset("defaults.audience", true, mock)
	if err != nil {
		t.Fatalf("configUnset returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Defaults.Audience != nil {
		t.Fatalf("expected audience to be nil after unset, got %v", mock.WrittenUserCfg.Defaults.Audience)
	}
}

func TestConfigUnsetGlobalDefaultsRankClearsField(t *testing.T) {
	rank := 5
	mock := &changeentry.MockConfigIO{
		UserCfg: &changeentry.RawConfig{
			Defaults: changeentry.RawDefaults{
				Rank: &rank,
			},
		},
	}
	err := configUnset("defaults.rank", true, mock)
	if err != nil {
		t.Fatalf("configUnset returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Defaults.Rank != nil {
		t.Fatalf("expected rank to be nil after unset, got %v", mock.WrittenUserCfg.Defaults.Rank)
	}
}

func TestConfigUnsetGlobalGitWriteAllowClearsField(t *testing.T) {
	allow := false
	mock := &changeentry.MockConfigIO{
		UserCfg: &changeentry.RawConfig{
			Git: changeentry.RawGit{
				Write: changeentry.RawGitWrite{
					Allow: &allow,
				},
			},
		},
	}
	err := configUnset("git.write.allow", true, mock)
	if err != nil {
		t.Fatalf("configUnset returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Git.Write.Allow != nil {
		t.Fatalf("expected git.write.allow to be nil after unset, got %v", mock.WrittenUserCfg.Git.Write.Allow)
	}
}

func TestConfigUnsetGlobalNilConfigCreatesEmpty(t *testing.T) {
	mock := &changeentry.MockConfigIO{UserCfg: nil}
	err := configUnset("defaults.audience", true, mock)
	if err != nil {
		t.Fatalf("configUnset returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called even when no existing config")
	}
}

func TestConfigUnsetUnknownKeyReturnsError(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configUnset("unknown.key", true, mock)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

// ── configSet (repo scope) ────────────────────────────────────────────────────

func TestConfigSetRepoDefaultsComponentWritesRepoConfig(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	err := configSet("defaults.component", []string{"api"}, false, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenRepoCfg == nil {
		t.Fatal("expected WriteRepoConfig to be called")
	}
	if len(mock.WrittenRepoCfg.Defaults.Component) != 1 || string(mock.WrittenRepoCfg.Defaults.Component[0]) != "api" {
		t.Fatalf("expected component [api], got %v", mock.WrittenRepoCfg.Defaults.Component)
	}
}

func TestConfigSetRepoDefaultsComponentPreservesExistingConfig(t *testing.T) {
	rank := 7
	mock := &changeentry.MockConfigIO{
		RepoCfg: &changeentry.RawConfig{
			Defaults: changeentry.RawDefaults{
				Rank: &rank,
			},
		},
	}
	err := configSet("defaults.component", []string{"sdk"}, false, mock)
	if err != nil {
		t.Fatalf("configSet returned error: %v", err)
	}
	if mock.WrittenRepoCfg == nil {
		t.Fatal("expected WriteRepoConfig to be called")
	}
	if mock.WrittenRepoCfg.Defaults.Rank == nil || *mock.WrittenRepoCfg.Defaults.Rank != 7 {
		t.Fatalf("expected rank 7 to be preserved, got %v", mock.WrittenRepoCfg.Defaults.Rank)
	}
}

// ── configUnset (repo scope) ──────────────────────────────────────────────────

func TestConfigUnsetRepoDefaultsAudienceClearsField(t *testing.T) {
	audience := changeentry.StringListConfig([]string{"public"})
	mock := &changeentry.MockConfigIO{
		RepoCfg: &changeentry.RawConfig{
			Defaults: changeentry.RawDefaults{
				Audience: audience,
			},
		},
	}
	err := configUnset("defaults.audience", false, mock)
	if err != nil {
		t.Fatalf("configUnset returned error: %v", err)
	}
	if mock.WrittenRepoCfg == nil {
		t.Fatal("expected WriteRepoConfig to be called")
	}
	if mock.WrittenRepoCfg.Defaults.Audience != nil {
		t.Fatalf("expected audience to be nil after unset, got %v", mock.WrittenRepoCfg.Defaults.Audience)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func TestFormatStringListEmptyReturnsEmpty(t *testing.T) {
	got := formatStringList(nil)
	if got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}

func TestFormatStringListSingleValue(t *testing.T) {
	got := formatStringList([]string{"public"})
	if got != "public" {
		t.Fatalf("expected 'public', got %q", got)
	}
}

func TestFormatStringListMultipleValues(t *testing.T) {
	got := formatStringList([]string{"public", "internal"})
	if got != "public, internal" {
		t.Fatalf("expected 'public, internal', got %q", got)
	}
}

func TestExpandCSVSingleValue(t *testing.T) {
	got := expandCSV([]string{"public"})
	if len(got) != 1 || got[0] != "public" {
		t.Fatalf("expected [public], got %v", got)
	}
}

func TestExpandCSVCommaSeparated(t *testing.T) {
	got := expandCSV([]string{"public,internal"})
	if len(got) != 2 || got[0] != "public" || got[1] != "internal" {
		t.Fatalf("expected [public internal], got %v", got)
	}
}

func TestExpandCSVMultipleArgs(t *testing.T) {
	got := expandCSV([]string{"public", "internal"})
	if len(got) != 2 || got[0] != "public" || got[1] != "internal" {
		t.Fatalf("expected [public internal], got %v", got)
	}
}

func TestExpandCSVTrimsSpaces(t *testing.T) {
	got := expandCSV([]string{" public , internal "})
	if len(got) != 2 || got[0] != "public" || got[1] != "internal" {
		t.Fatalf("expected [public internal], got %v", got)
	}
}

func TestParseBoolTrue(t *testing.T) {
	b, err := parseBool([]string{"true"})
	if err != nil || !b {
		t.Fatalf("expected true, got %v %v", b, err)
	}
}

func TestParseBoolFalse(t *testing.T) {
	b, err := parseBool([]string{"false"})
	if err != nil || b {
		t.Fatalf("expected false, got %v %v", b, err)
	}
}

func TestParseBoolRejectsInvalid(t *testing.T) {
	_, err := parseBool([]string{"yes"})
	if err == nil {
		t.Fatal("expected error for 'yes'")
	}
}

func TestParseBoolRejectsMultipleValues(t *testing.T) {
	_, err := parseBool([]string{"true", "false"})
	if err == nil {
		t.Fatal("expected error for multiple values")
	}
}

// ── renderTypes ───────────────────────────────────────────────────────────────

func TestRenderTypesOutputsHeaders(t *testing.T) {
	defs := changeentry.DefaultTypeRegistry().Definitions()
	var buf bytes.Buffer
	if err := renderTypes(defs, &buf); err != nil {
		t.Fatalf("renderTypes returned error: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "ID") {
		t.Fatalf("expected 'ID' header in output, got:\n%s", output)
	}
	if !strings.Contains(output, "BUMP") {
		t.Fatalf("expected 'BUMP' header in output, got:\n%s", output)
	}
	if !strings.Contains(output, "TITLE") {
		t.Fatalf("expected 'TITLE' header in output, got:\n%s", output)
	}
}

func TestRenderTypesOutputsBuiltinTypes(t *testing.T) {
	defs := changeentry.DefaultTypeRegistry().Definitions()
	var buf bytes.Buffer
	if err := renderTypes(defs, &buf); err != nil {
		t.Fatalf("renderTypes returned error: %v", err)
	}
	output := buf.String()
	for _, expected := range []string{"feature", "fix", "removal", "security", "docs"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected type %q in output, got:\n%s", expected, output)
		}
	}
}

// ── configList output ─────────────────────────────────────────────────────────

func TestConfigListOutputsDefaultsSection(t *testing.T) {
	// configList needs a real git root, so we just test that it produces
	// sensible output when called with no error.
	var buf bytes.Buffer
	// We call it directly; since we're running tests inside the chagg repo,
	// resolveModuleOrDefault will succeed.
	mock := &changeentry.MockConfigIO{}
	err := configList(mock, &buf)
	if err != nil {
		t.Skipf("configList requires a git root, skipping: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "Defaults:") {
		t.Fatalf("expected 'Defaults:' section in output, got:\n%s", output)
	}
	if !strings.Contains(output, "Git write policy:") {
		t.Fatalf("expected 'Git write policy:' section in output, got:\n%s", output)
	}
}

// ── configGet output ──────────────────────────────────────────────────────────

func TestConfigGetUnknownKeyReturnsError(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	var buf bytes.Buffer
	err := configGet("no.such.key", mock, &buf)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "unknown config key") {
		t.Fatalf("expected 'unknown config key' in error, got %q", err.Error())
	}
}

// ── configAction dispatch ─────────────────────────────────────────────────────

func TestConfigActionDispatchesToSetOnKeyAndValue(t *testing.T) {
	mock := &changeentry.MockConfigIO{}
	cmd := ConfigCommand(mock)

	// Run: chagg config defaults.audience public --global
	err := cmd.Run(context.Background(), []string{"config", "--global", "defaults.audience", "public"})
	if err != nil {
		t.Fatalf("config command returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if len(mock.WrittenUserCfg.Defaults.Audience) != 1 || string(mock.WrittenUserCfg.Defaults.Audience[0]) != "public" {
		t.Fatalf("expected audience [public], got %v", mock.WrittenUserCfg.Defaults.Audience)
	}
}

func TestConfigActionDispatchesToUnsetWhenFlagSet(t *testing.T) {
	audience := changeentry.StringListConfig([]string{"public"})
	mock := &changeentry.MockConfigIO{
		UserCfg: &changeentry.RawConfig{
			Defaults: changeentry.RawDefaults{
				Audience: audience,
			},
		},
	}
	cmd := ConfigCommand(mock)

	err := cmd.Run(context.Background(), []string{"config", "--global", "--unset", "defaults.audience"})
	if err != nil {
		t.Fatalf("config command returned error: %v", err)
	}
	if mock.WrittenUserCfg == nil {
		t.Fatal("expected WriteUserConfig to be called")
	}
	if mock.WrittenUserCfg.Defaults.Audience != nil {
		t.Fatalf("expected audience to be nil after unset, got %v", mock.WrittenUserCfg.Defaults.Audience)
	}
}
