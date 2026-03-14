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
	normalized, err := NormalizeType("feat")
	if err != nil {
		t.Fatalf("NormalizeType returned error: %v", err)
	}

	if normalized != ChangeTypeFeature {
		t.Fatalf("expected %q, got %q", ChangeTypeFeature, normalized)
	}
}

func TestNormalizeTypeReturnsTypedValidationError(t *testing.T) {
	_, err := NormalizeType("unsupported")
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

func TestRenderEntryOmitsDefaultFields(t *testing.T) {
	entry := Entry{
		Type:     ChangeTypeFeature,
		Audience: []string{DefaultAudience},
		Priority: 0,
		Body:     "Test.",
	}

	rendered, err := RenderEntry(entry)
	if err != nil {
		t.Fatalf("RenderEntry returned error: %v", err)
	}

	if strings.Contains(rendered, "breaking:") {
		t.Fatalf("expected default breaking to be omitted, got:\n%s", rendered)
	}

	if strings.Contains(rendered, "audience:") {
		t.Fatalf("expected default audience to be omitted, got:\n%s", rendered)
	}

	if strings.Contains(rendered, "priority:") {
		t.Fatalf("expected default priority to be omitted, got:\n%s", rendered)
	}
}

func TestRenderEntryIncludesNonDefaultFields(t *testing.T) {
	entry := Entry{
		Type:     ChangeTypeFix,
		Breaking: true,
		Audience: []string{"internal"},
		Priority: 10,
		Body:     "Fix.",
	}

	rendered, err := RenderEntry(entry)
	if err != nil {
		t.Fatalf("RenderEntry returned error: %v", err)
	}

	if !strings.Contains(rendered, "breaking: true") {
		t.Fatalf("expected non-default breaking to be included, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "audience: internal") {
		t.Fatalf("expected non-default audience to be included, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "priority: 10") {
		t.Fatalf("expected non-default priority to be included, got:\n%s", rendered)
	}
}

func TestRenderEntryKeepsBlankLineBetweenHeaderAndBody(t *testing.T) {
	entry := Entry{
		Type: ChangeTypeDocs,
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
		Body: "Body line",
	}

	rendered, err := RenderEntry(entry)
	if err != nil {
		t.Fatalf("RenderEntry returned error: %v", err)
	}

	if strings.Contains(rendered, "type: fix---") {
		t.Fatalf("expected closing delimiter not to be concatenated with type, got:\n%s", rendered)
	}

	if !strings.Contains(rendered, "---\n\nBody line") {
		t.Fatalf("expected body to start after closing delimiter and one blank line, got:\n%s", rendered)
	}
}

func TestCreateChangeCreatesTargetFileUnderChangesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")
	workingDir := filepath.Join(repoDir, "pkg", "api")

	if err := os.MkdirAll(workingDir, 0o755); err != nil {
		t.Fatalf("mkdir working dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	params := Params{
		Type:    "fix",
		TypeSet: true,
	}

	path, err := CreateChange(workingDir, "auth/token", params, strings.NewReader(""), bytes.NewBuffer(nil), false)
	if err != nil {
		t.Fatalf("CreateChange returned error: %v", err)
	}

	expectedPath := filepath.Join(repoDir, ".changes", "auth", "token.md")
	if path != expectedPath {
		t.Fatalf("expected %q, got %q", expectedPath, path)
	}

	contentBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}

	content := string(contentBytes)
	if !strings.Contains(content, "type: fix") {
		t.Fatalf("expected rendered entry to contain type, got:\n%s", content)
	}
}

func TestCreateChangeReturnsValidationErrorWithoutPromptInNonInteractiveMode(t *testing.T) {
	tempDir := t.TempDir()
	repoDir := filepath.Join(tempDir, "repo")

	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}

	output := bytes.NewBuffer(nil)
	_, err := CreateChange(repoDir, "auth/token", Params{}, strings.NewReader(""), output, false)
	if err == nil {
		t.Fatalf("expected error")
	}

	var validationErr *ValidationError
	if !errors.As(err, &validationErr) {
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

	path, err := CreateChange(repoDir, "", params, strings.NewReader("auth/token\n"), output, true)
	if err != nil {
		t.Fatalf("CreateChange returned error: %v", err)
	}

	expected := filepath.Join(repoDir, ".changes", "auth", "token.md")
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

	_, err := CreateChange(repoDir, "", Params{Type: "fix", TypeSet: true}, strings.NewReader(""), bytes.NewBuffer(nil), false)
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
