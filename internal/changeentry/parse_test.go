package changeentry

import "testing"

func defaultTestModule() ModuleConfig {
	return ModuleConfig{Types: DefaultTypeRegistry()}
}

func TestInferTypeFromFilenameAcceptsAliasesAndCase(t *testing.T) {
	registry := DefaultTypeRegistry()
	cases := []struct {
		path string
		want ChangeType
	}{
		{path: "feat__test.md", want: ChangeTypeFeature},
		{path: "FEAT__test.md", want: ChangeTypeFeature},
		{path: "Feat_test.md", want: ChangeTypeFeature},
		{path: "fix__test.md", want: ChangeTypeFix},
	}

	for _, tc := range cases {
		got, err := InferTypeFromFilename(tc.path, registry)
		if err != nil {
			t.Fatalf("InferTypeFromFilename(%q) returned error: %v", tc.path, err)
		}
		if got != tc.want {
			t.Fatalf("InferTypeFromFilename(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestParseEntryAllowsBodyWithoutFrontMatter(t *testing.T) {
	entry, errs := ParseEntry("Body only", "feat__body.md", defaultTestModule())
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if entry.Type != ChangeTypeFeature {
		t.Fatalf("expected feature type, got %q", entry.Type)
	}
	if entry.Body != "Body only" {
		t.Fatalf("expected body, got %q", entry.Body)
	}
}

func TestParseEntryAllowsUnknownFrontMatterFields(t *testing.T) {
	content := "---\ncustom: value\n---\n\nDocs body"
	entry, errs := ParseEntry(content, "docs__custom.md", defaultTestModule())
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if entry.Type != ChangeTypeDocs {
		t.Fatalf("expected docs type, got %q", entry.Type)
	}
	if entry.Body != "Docs body" {
		t.Fatalf("expected body Docs body, got %q", entry.Body)
	}
}

func TestParseEntryAppliesConfiguredDefaultAudienceWhenMissing(t *testing.T) {
	module := ModuleConfig{
		Types:    DefaultTypeRegistry(),
		Defaults: Defaults{Audience: []string{"public", "developer"}},
	}
	entry, errs := ParseEntry("Body", "fix__body.md", module)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(entry.Audience) != 2 || entry.Audience[0] != "public" || entry.Audience[1] != "developer" {
		t.Fatalf("expected default audience [public developer], got %#v", entry.Audience)
	}
}

func TestParseEntryExplicitEmptyAudienceOverridesConfiguredDefault(t *testing.T) {
	content := "---\naudience: []\n---\n\nBody"
	module := ModuleConfig{
		Types:    DefaultTypeRegistry(),
		Defaults: Defaults{Audience: []string{"public", "developer"}},
	}
	entry, errs := ParseEntry(content, "fix__body.md", module)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}

	if len(entry.Audience) != 0 {
		t.Fatalf("expected explicit empty audience to stay empty, got %#v", entry.Audience)
	}
}

func TestParseEntryFailsForFilenameWithoutTypePrefix(t *testing.T) {
	_, errs := ParseEntry("Body", "plain.md", defaultTestModule())
	if len(errs) == 0 {
		t.Fatalf("expected error")
	}
}
