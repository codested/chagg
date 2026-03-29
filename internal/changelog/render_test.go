package changelog

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/codested/chagg/internal/changeentry"
	"github.com/codested/chagg/internal/semver"
)

func TestRenderMarkdownOmitsDateAndIncludesIndentedFullBody(t *testing.T) {
	changeLog := &ChangeLog{
		Groups: []VersionGroup{{
			Version: "v1.2.3",
			Tag: &semver.Tag{
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
	if err := RenderMarkdown(changeLog, "", buffer); err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}

	output := buffer.String()
	if strings.Contains(output, "## 1.2.3 –") {
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
	if err := RenderMarkdown(changeLog, "", buffer); err != nil {
		t.Fatalf("RenderMarkdown returned error: %v", err)
	}

	output := buffer.String()
	if strings.Contains(output, "### Documentation") {
		t.Fatalf("expected docs not to render in their own section, got:\n%s", output)
	}

	headingIndex := strings.Index(output, "## 1.2.3")
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
	if err := RenderMarkdown(changeLog, "", buffer); err != nil {
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
		Module: changeentry.ModuleConfig{Name: "default", ChangesDir: "/repo/.changes"},
		Groups: []VersionGroup{{
			Version: "staging",
			TypeGroups: []TypeGroup{{
				ChangeType: changeentry.ChangeTypeFix,
				Title:      "Bug Fixes",
				Entries: []EntryWithMeta{{
					Entry:            changeentry.Entry{Type: changeentry.ChangeTypeFix, Body: "Fix bug."},
					Path:             "/repo/.changes/feature/fix__bug.md",
					OriginalFilename: "fix__bug.md",
					AddedCommitHash:  "abc123def",
				}},
			}},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderJSON(changeLog, "/repo", "", buffer); err != nil {
		t.Fatalf("RenderJSON returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	if payload["schema_version"] != float64(1) {
		t.Fatalf("expected schema_version 1, got %#v", payload["schema_version"])
	}

	if payload["module"] != "default" {
		t.Fatalf("expected module default, got %#v", payload["module"])
	}

	groups, ok := payload["groups"].([]any)
	if !ok || len(groups) == 0 {
		t.Fatalf("expected groups in payload")
	}
	firstGroup, ok := groups[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first group object")
	}
	types, ok := firstGroup["types"].([]any)
	if !ok || len(types) == 0 {
		t.Fatalf("expected types in first group")
	}
	firstType, ok := types[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first type object")
	}
	entries, ok := firstType["entries"].([]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("expected entries in first type")
	}
	firstEntry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first entry object")
	}
	if firstEntry["id"] != "feature/fix__bug.md@abc123def" {
		t.Fatalf("expected entry id feature/fix__bug.md@abc123def, got %#v", firstEntry["id"])
	}
	if firstEntry["path"] != ".changes/feature/fix__bug.md" {
		t.Fatalf("expected repo-relative path .changes/feature/fix__bug.md, got %#v", firstEntry["path"])
	}
}

func TestRenderJSONOmitsBodyWhenRedundant(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantPreview string
		wantBody    string // empty means omitted
	}{
		{
			name:        "plain text — preview equals body",
			body:        "Fix login bug",
			wantPreview: "Fix login bug",
			wantBody:    "",
		},
		{
			name:        "heading — preview is heading text, body omitted",
			body:        "# Fix login bug",
			wantPreview: "Fix login bug",
			wantBody:    "",
		},
		{
			name:        "heading with extra content — body is remainder only",
			body:        "# Fix login bug\n\nThis was caused by a race condition.",
			wantPreview: "Fix login bug",
			wantBody:    "This was caused by a race condition.",
		},
		{
			name:        "multi-line without heading — body is remainder only",
			body:        "Fix login bug\n\nMore details here.",
			wantPreview: "Fix login bug",
			wantBody:    "More details here.",
		},
		{
			name:        "long single line — preview truncated, body is full raw text",
			body:        strings.Repeat("a", 120),
			wantPreview: strings.Repeat("a", 77) + "...",
			wantBody:    strings.Repeat("a", 120),
		},
		{
			name:        "bold text in preview — asterisks stripped",
			body:        "**Important** fix for login",
			wantPreview: "Important fix for login",
			wantBody:    "",
		},
		{
			name:        "heading with bold in body — both stripped in preview",
			body:        "## **Breaking Change**\n\nRemoved deprecated API.",
			wantPreview: "Breaking Change",
			wantBody:    "Removed deprecated API.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := &ChangeLog{
				Module: changeentry.ModuleConfig{Name: "default", ChangesDir: "/repo/.changes"},
				Groups: []VersionGroup{{
					Version: "staging",
					TypeGroups: []TypeGroup{{
						ChangeType: changeentry.ChangeTypeFix,
						Title:      "Bug Fixes",
						Entries: []EntryWithMeta{{
							Entry:           changeentry.Entry{Type: changeentry.ChangeTypeFix, Body: tt.body},
							Path:            "/repo/.changes/fix__test.md",
							AddedCommitHash: "abc123",
						}},
					}},
				}},
			}

			buf := bytes.NewBuffer(nil)
			if err := RenderJSON(cl, "/repo", "", buf); err != nil {
				t.Fatalf("RenderJSON: %v", err)
			}

			var doc struct {
				Groups []struct {
					Types []struct {
						Entries []struct {
							Preview string `json:"preview"`
							Body    string `json:"body"`
						} `json:"entries"`
					} `json:"types"`
				} `json:"groups"`
			}
			if err := json.Unmarshal(buf.Bytes(), &doc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			entry := doc.Groups[0].Types[0].Entries[0]
			if entry.Preview != tt.wantPreview {
				t.Errorf("preview = %q, want %q", entry.Preview, tt.wantPreview)
			}
			if entry.Body != tt.wantBody {
				t.Errorf("body = %q, want %q", entry.Body, tt.wantBody)
			}
		})
	}
}

func TestRenderJSONIncludesRelativeChangesPathInID(t *testing.T) {
	changeLog := &ChangeLog{
		Module: changeentry.ModuleConfig{Name: "msal-react", ChangesDir: "/repo/lib/msal-react/.changes"},
		Groups: []VersionGroup{{
			Version: "staging",
			TypeGroups: []TypeGroup{{
				ChangeType: changeentry.ChangeTypeFeature,
				Title:      "Features",
				Entries: []EntryWithMeta{{
					Entry:           changeentry.Entry{Type: changeentry.ChangeTypeFeature, Body: "Add thing."},
					Path:            "/repo/lib/msal-react/.changes/ui/feature__add-thing.md",
					AddedCommitHash: "deadbeef",
				}},
			}},
		}},
	}

	buffer := bytes.NewBuffer(nil)
	if err := RenderJSON(changeLog, "/repo", "", buffer); err != nil {
		t.Fatalf("RenderJSON returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(buffer.Bytes(), &payload); err != nil {
		t.Fatalf("failed to unmarshal output: %v", err)
	}

	groups := payload["groups"].([]any)
	firstGroup := groups[0].(map[string]any)
	types := firstGroup["types"].([]any)
	firstType := types[0].(map[string]any)
	entries := firstType["entries"].([]any)
	firstEntry := entries[0].(map[string]any)

	if firstEntry["id"] != "msal-react/ui/feature__add-thing.md@deadbeef" {
		t.Fatalf("expected id msal-react/ui/feature__add-thing.md@deadbeef, got %#v", firstEntry["id"])
	}
}
