package changelog

import (
	"bytes"
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
