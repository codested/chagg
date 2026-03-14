package commands

import "testing"

func TestNormalizeGenerateFormat(t *testing.T) {
	if value := normalizeGenerateFormat("md"); value != "markdown" {
		t.Fatalf("expected markdown, got %q", value)
	}
	if value := normalizeGenerateFormat("HTML"); value != "html" {
		t.Fatalf("expected html, got %q", value)
	}
	if value := normalizeGenerateFormat("json"); value != "json" {
		t.Fatalf("expected json, got %q", value)
	}
}
