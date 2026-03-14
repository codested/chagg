package changelog

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codested/chagg/internal/changeentry"
)

func TestRenderMarkdownOmitsDateAndIncludesIndentedFullBody(t *testing.T) {
	changeLog := &ChangeLog{
		Groups: []VersionGroup{{
			Version: "v1.2.3",
			Tag: &Tag{
				Name:       "v1.2.3",
				CommitDate: time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
			},
			TypeGroups: []TypeGroup{{
				Title: "Features",
				Entries: []EntryWithMeta{{
					Entry: changeentry.Entry{
						Type:      changeentry.ChangeTypeFeature,
						Component: []string{"api"},
						Body:      "First line.\n\nSecond line.\n  \nThird line.",
					},
					Path: "entry.md",
				}},
			}},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderMarkdown(changeLog, buffer); err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}

	output := buffer.String()
	if strings.Contains(output, "## v1.2.3 –") {
		t.Fatalf("expected heading without date, got:\n%s", output)
	}

	expected := "- First line. *(api)*\n  Second line.\n  Third line."
	if !strings.Contains(output, expected) {
		t.Fatalf("expected full indented body bullet, got:\n%s", output)
	}
}

func TestBodyBulletLinesTrimsAndRemovesBlankLines(t *testing.T) {
	lines := bodyBulletLines("\n  Line one\n\n  \nLine two  \n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}

	if lines[0] != "Line one" || lines[1] != "Line two" {
		t.Fatalf("unexpected lines: %#v", lines)
	}
}

func TestRenderMarkdownRendersDocsAsUnformattedBodyBeforeSections(t *testing.T) {
	changeLog := &ChangeLog{
		Groups: []VersionGroup{{
			Version: "v1.2.3",
			TypeGroups: []TypeGroup{
				{
					ChangeType: changeentry.ChangeTypeDocs,
					Title:      "Documentation",
					Entries: []EntryWithMeta{{
						Entry: changeentry.Entry{Type: changeentry.ChangeTypeDocs, Body: "Imported notes from release API."},
						Path:  ".changes/docs/imported.md",
					}},
				},
				{
					ChangeType: changeentry.ChangeTypeFeature,
					Title:      "Features",
					Entries: []EntryWithMeta{{
						Entry: changeentry.Entry{Type: changeentry.ChangeTypeFeature, Body: "Add OAuth support."},
						Path:  ".changes/api/oauth.md",
					}},
				},
			},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderMarkdown(changeLog, buffer); err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}

	output := buffer.String()
	if strings.Contains(output, "### Documentation") {
		t.Fatalf("expected docs not to render in their own section, got:\n%s", output)
	}

	headingIndex := strings.Index(output, "## v1.2.3")
	docIndex := strings.Index(output, "Imported notes from release API.")
	featureIndex := strings.Index(output, "### Features")
	if headingIndex == -1 || docIndex == -1 || featureIndex == -1 || !(headingIndex < docIndex && docIndex < featureIndex) {
		t.Fatalf("expected docs body between version heading and feature section, got:\n%s", output)
	}
}

func TestRenderMarkdownSeparatesMultipleDocsWithBlankLine(t *testing.T) {
	changeLog := &ChangeLog{
		Groups: []VersionGroup{{
			Version: "v2.0.0",
			TypeGroups: []TypeGroup{{
				ChangeType: changeentry.ChangeTypeDocs,
				Title:      "Documentation",
				Entries: []EntryWithMeta{
					{Entry: changeentry.Entry{Type: changeentry.ChangeTypeDocs, Body: "First imported release note."}, Path: ".changes/docs/one.md"},
					{Entry: changeentry.Entry{Type: changeentry.ChangeTypeDocs, Body: "Second imported release note."}, Path: ".changes/docs/two.md"},
				},
			}},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderMarkdown(changeLog, buffer); err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "First imported release note.\n\nSecond imported release note.") {
		t.Fatalf("expected blank line separation between docs entries, got:\n%s", output)
	}
}

func TestRenderLogIncludesRelativePathForEachEntry(t *testing.T) {
	baseDir := "/repo"
	entryPath := filepath.Join(baseDir, ".changes", "api", "oauth.md")

	changeLog := &ChangeLog{
		Groups: []VersionGroup{{
			Version: "staging",
			TypeGroups: []TypeGroup{{
				Title: "Features",
				Entries: []EntryWithMeta{{
					Entry: changeentry.Entry{
						Type: changeentry.ChangeTypeFeature,
						Body: "Add OAuth support.",
					},
					Path: entryPath,
				}},
			}},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderLog(changeLog, baseDir, 80, buffer); err != nil {
		t.Fatalf("RenderLog returned error: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "[.changes/api/oauth.md]") {
		t.Fatalf("expected log output to contain relative path, got:\n%s", output)
	}

	pathIndex := strings.Index(output, "[.changes/api/oauth.md]")
	typeIndex := strings.Index(output, "feature")
	if pathIndex == -1 || typeIndex == -1 || pathIndex >= typeIndex {
		t.Fatalf("expected log output to put path before type, got:\n%s", output)
	}
}

func TestTruncateLogPreviewAddsEllipsisWhenPreviewTooLong(t *testing.T) {
	input := strings.Repeat("a", 120)
	result := truncateLogPreview(input, 80)

	if len(result) != 80 {
		t.Fatalf("expected truncated length 80, got %d", len(result))
	}

	if !strings.HasSuffix(result, "...") {
		t.Fatalf("expected truncated preview to end with ellipsis, got %q", result)
	}
}

func TestRenderJSONProducesStructuredOutput(t *testing.T) {
	changeLog := &ChangeLog{
		Module: changeentry.ModuleConfig{Name: "default"},
		Groups: []VersionGroup{{
			Version: "staging",
			TypeGroups: []TypeGroup{{
				ChangeType: changeentry.ChangeTypeFix,
				Title:      "Bug Fixes",
				Entries: []EntryWithMeta{{
					Entry: changeentry.Entry{Type: changeentry.ChangeTypeFix, Body: "Fix bug."},
					Path:  ".changes/fix.md",
				}},
			}},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderJSON(changeLog, buffer); err != nil {
		t.Fatalf("RenderJSON returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if payload["module"] != "default" {
		t.Fatalf("expected module default, got %#v", payload["module"])
	}
}

func TestRenderHTMLProducesExpectedSections(t *testing.T) {
	changeLog := &ChangeLog{
		Groups: []VersionGroup{{
			Version: "v1.0.0",
			TypeGroups: []TypeGroup{{
				ChangeType: changeentry.ChangeTypeFeature,
				Title:      "Features",
				Entries: []EntryWithMeta{{
					Entry: changeentry.Entry{Type: changeentry.ChangeTypeFeature, Body: "Add thing."},
					Path:  ".changes/add.md",
				}},
			}},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderHTML(changeLog, buffer); err != nil {
		t.Fatalf("RenderHTML returned error: %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "<h1>Changelog</h1>") {
		t.Fatalf("expected html title, got:\n%s", output)
	}
	if !strings.Contains(output, "<h2>v1.0.0</h2>") {
		t.Fatalf("expected version section, got:\n%s", output)
	}
	if !strings.Contains(output, "<h3>Features</h3>") {
		t.Fatalf("expected type section, got:\n%s", output)
	}
}
