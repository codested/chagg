package changeentry

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveChangesDirectoryPrefersExistingChanges(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	nestedDir := filepath.Join(repoDir, "nested", "deep")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.MkdirAll(changesDir, 0o755); err != nil {
		t.Fatalf("mkdir .changes: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	resolved, err := ResolveChangesDirectory(nestedDir)
	if err != nil {
		t.Fatalf("ResolveChangesDirectory returned error: %v", err)
	}

	if resolved != changesDir {
		t.Fatalf("expected %q, got %q", changesDir, resolved)
	}
}

func TestResolveChangesDirectoryFallsBackToGitDirectory(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	nestedDir := filepath.Join(repoDir, "nested", "deep")

	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	resolved, err := ResolveChangesDirectory(nestedDir)
	if err != nil {
		t.Fatalf("ResolveChangesDirectory returned error: %v", err)
	}

	expected := filepath.Join(repoDir, ".changes")
	if resolved != expected {
		t.Fatalf("expected %q, got %q", expected, resolved)
	}
}

func TestBuildChangeFilePathAppendsMarkdownExtension(t *testing.T) {
	tempDir := t.TempDir()

	resolved, err := BuildChangeFilePath(tempDir, "auth/new-login")
	if err != nil {
		t.Fatalf("BuildChangeFilePath returned error: %v", err)
	}

	expected := filepath.Join(tempDir, "auth", "new-login.md")
	if resolved != expected {
		t.Fatalf("expected %q, got %q", expected, resolved)
	}
}

func TestBuildChangeFilePathDoesNotDuplicateExtension(t *testing.T) {
	tempDir := t.TempDir()

	resolved, err := BuildChangeFilePath(tempDir, "auth/new-login.md")
	if err != nil {
		t.Fatalf("BuildChangeFilePath returned error: %v", err)
	}

	expected := filepath.Join(tempDir, "auth", "new-login.md")
	if resolved != expected {
		t.Fatalf("expected %q, got %q", expected, resolved)
	}
}

func TestNormalizeTypeAlias(t *testing.T) {
	normalized, err := DefaultTypeRegistry().NormalizeType("feat")
	if err != nil {
		t.Fatalf("NormalizeType returned error: %v", err)
	}

	if normalized != ChangeTypeFeature {
		t.Fatalf("expected %q, got %q", ChangeTypeFeature, normalized)
	}
}

func TestNormalizeTypeReturnsTypedValidationError(t *testing.T) {
	_, err := DefaultTypeRegistry().NormalizeType("unsupported")
	if err == nil {
		t.Fatalf("expected error")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if validationErr.ExitCode() != ExitCodeValidation {
		t.Fatalf("expected exit code %d, got %d", ExitCodeValidation, validationErr.ExitCode())
	}
}

func TestRenderEntryEndsWithNewline(t *testing.T) {
	cases := []Entry{
		{Type: ChangeTypeFeature, Body: "Body only, no header."},
		{Type: ChangeTypeFix, Bump: BumpLevelMajor, Body: "Body with header."},
		{Type: ChangeTypeDocs, Bump: BumpLevelMinor, Body: ""},
	}

	for _, entry := range cases {
		rendered, err := RenderEntry(entry)
		if err != nil {
			t.Fatalf("RenderEntry returned error: %v", err)
		}
		if rendered == "" {
			continue // empty output is fine
		}
		if !strings.HasSuffix(rendered, "\n") {
			t.Fatalf("expected rendered entry to end with newline, got:\n%q", rendered)
		}
	}
}

func TestRenderEntryOmitsDefaultFields(t *testing.T) {
	entry := Entry{
		Type:     ChangeTypeFeature,
		Audience: nil,
		Rank:     0,
		Body:     "Test.",
	}

	rendered, err := RenderEntry(entry)
	if err != nil {
		t.Fatalf("RenderEntry returned error: %v", err)
	}

	if strings.Contains(rendered, "bump:") {
		t.Fatalf("expected default bump to be omitted, got:\n%s", rendered)
	}

	if strings.Contains(rendered, "audience:") {
		t.Fatalf("expected default audience to be omitted, got:\n%s", rendered)
	}

	if strings.Contains(rendered, "rank:") {
		t.Fatalf("expected default rank to be omitted, got:\n%s", rendered)
	}
}

func TestRenderEntryIncludesNonDefaultFields(t *testing.T) {
	entry := Entry{
		Type:     ChangeTypeFix,
		Bump:     BumpLevelMajor,
		Audience: []string{"internal"},
		Rank:     10,
		Body:     "Fix.",
	}

	rendered, err := RenderEntry(entry)
	if err != nil {
		t.Fatalf("RenderEntry returned error: %v", err)
	}

	if !strings.Contains(rendered, "bump: major") {
		t.Fatalf("expected non-default bump to be included, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "audience: internal") {
		t.Fatalf("expected non-default audience to be included, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "rank: 10") {
		t.Fatalf("expected rank to be rendered, got:\n%s", rendered)
	}
}

func TestRenderEntryKeepsBlankLineBetweenHeaderAndBody(t *testing.T) {
	entry := Entry{
		Type: ChangeTypeDocs,
		Bump: BumpLevelMajor,
		Body: "Document usage.",
	}

	rendered, err := RenderEntry(entry)
	if err != nil {
		t.Fatalf("RenderEntry returned error: %v", err)
	}

	if !strings.Contains(rendered, "---\n\nDocument usage.") {
		t.Fatalf("expected blank line between front matter and body, got:\n%s", rendered)
	}
}

func TestRenderEntryPlacesClosingDelimiterOnSeparateLine(t *testing.T) {
	entry := Entry{
		Type: ChangeTypeFix,
		Bump: BumpLevelMajor,
		Body: "Body line",
	}

	rendered, err := RenderEntry(entry)
	if err != nil {
		t.Fatalf("RenderEntry returned error: %v", err)
	}

	if strings.Contains(rendered, "bump: major---") {
		t.Fatalf("expected closing delimiter not to be concatenated with header fields, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "---\n\nBody line") {
		t.Fatalf("expected body to start after closing delimiter and one blank line, got:\n%s", rendered)
	}
}

func TestCreateChangeInfersTypeFromFilenameWithoutDuplicatingPrefix(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	// When the target already carries a type prefix, CreateChange must not
	// prompt for a type or prepend it again (e.g. feat__feat__name).
	params := Params{} // no type flag; type should be inferred from filename
	path, err := CreateChange(ModuleConfig{ChangesDir: changesDir, Types: DefaultTypeRegistry()}, "feat__my-change", params, strings.NewReader(""), bytes.NewBuffer(nil), false)
	if err != nil {
		t.Fatalf("CreateChange returned error: %v", err)
	}

	expectedPath := filepath.Join(changesDir, "feat__my-change.md")
	if path != expectedPath {
		t.Fatalf("expected %q, got %q", expectedPath, path)
	}
}

func TestCreateChangeInfersTypeFromFilenameInSubdirectory(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	params := Params{}
	path, err := CreateChange(ModuleConfig{ChangesDir: changesDir, Types: DefaultTypeRegistry()}, "ci/fix__pipeline", params, strings.NewReader(""), bytes.NewBuffer(nil), false)
	if err != nil {
		t.Fatalf("CreateChange returned error: %v", err)
	}

	expectedPath := filepath.Join(changesDir, "ci", "fix__pipeline.md")
	if path != expectedPath {
		t.Fatalf("expected %q, got %q", expectedPath, path)
	}
}

func TestCreateChangeCreatesTargetFileUnderChangesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	changesDir := filepath.Join(repoDir, ".changes")

	if err := os.MkdirAll(filepath.Join(repoDir, "pkg", "api"), 0o755); err != nil {
		t.Fatalf("mkdir working dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	params := Params{
		Type:    "fix",
		TypeSet: true,
	}

	path, err := CreateChange(ModuleConfig{ChangesDir: changesDir, Types: DefaultTypeRegistry()}, "auth/token", params, strings.NewReader(""), bytes.NewBuffer(nil), false)
	if err != nil {
		t.Fatalf("CreateChange returned error: %v", err)
	}

	expectedPath := filepath.Join(repoDir, ".changes", "auth", "fix__token.md")
	if path != expectedPath {
		t.Fatalf("expected %q, got %q", expectedPath, path)
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}

	content := string(contentBytes)
	if strings.Contains(content, "type:") {
		t.Fatalf("expected rendered entry to omit type front matter, got:\n%s", content)
	}
}

func TestCreateChangeReturnsValidationErrorWithoutPromptInNonInteractiveMode(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")

	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	output := bytes.NewBuffer(nil)
	_, err := CreateChange(ModuleConfig{ChangesDir: filepath.Join(repoDir, ".changes"), Types: DefaultTypeRegistry()}, "auth/token", Params{}, strings.NewReader(""), output, false)
	if err == nil {
		t.Fatalf("expected error")
	}

	if _, ok := errors.AsType[*ValidationError](err); !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if strings.Contains(output.String(), "Type (") {
		t.Fatalf("expected no interactive prompt in non-interactive mode, got output: %q", output.String())
	}
}

func TestCreateChangePromptsForPathWhenMissingInInteractiveMode(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")

	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	params := Params{Type: "fix", TypeSet: true}
	output := bytes.NewBuffer(nil)

	path, err := CreateChange(ModuleConfig{ChangesDir: filepath.Join(repoDir, ".changes"), Types: DefaultTypeRegistry()}, "", params, strings.NewReader("auth/token\n"), output, true)
	if err != nil {
		t.Fatalf("CreateChange returned error: %v", err)
	}

	expected := filepath.Join(repoDir, ".changes", "auth", "fix__token.md")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}

	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatalf("expected file to exist at %q: %v", path, statErr)
	}

	if !strings.Contains(output.String(), "Path (example: auth/new-login): ") {
		t.Fatalf("expected path prompt in output, got %q", output.String())
	}
}

func TestCreateChangeReturnsValidationErrorWhenPathMissingNonInteractive(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")

	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	_, err := CreateChange(ModuleConfig{ChangesDir: filepath.Join(repoDir, ".changes"), Types: DefaultTypeRegistry()}, "", Params{Type: "fix", TypeSet: true}, strings.NewReader(""), bytes.NewBuffer(nil), false)
	if err == nil {
		t.Fatalf("expected error")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}

	if !strings.Contains(validationErr.Error(), "path") {
		t.Fatalf("expected path validation error, got %q", validationErr.Error())
	}
}
