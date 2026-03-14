package changelog

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"text/template"

	"github.com/codested/chagg/internal/changeentry"
)

const defaultLogPreviewMaxLen = 80

//go:embed templates/changelog.md.tmpl
var changelogTemplateSource string

type changelogTemplateData struct {
	Groups []changelogTemplateGroup
}

type changelogTemplateGroup struct {
	Heading    string
	DocsBlock  string
	TypeGroups []changelogTemplateTypeGroup
}

type changelogTemplateTypeGroup struct {
	Title        string
	EntriesBlock string
}

var changelogTemplate = template.Must(template.New("changelog.md.tmpl").Parse(changelogTemplateSource))

// RenderLog writes a human-readable, columnar overview of the changelog
// groups to w.  baseDir is used to compute display-friendly relative paths
// (pass the repository root or working directory).
func RenderLog(cl *ChangeLog, baseDir string, previewMaxLen int, w io.Writer) error {
	if previewMaxLen <= 0 {
		previewMaxLen = defaultLogPreviewMaxLen
	}

	hasAny := false
	for _, g := range cl.Groups {
		if g.TotalEntries() > 0 {
			hasAny = true
			break
		}
	}
	if !hasAny {
		_, err := fmt.Fprintln(w, "No changes found.")
		return err
	}

	for i, group := range cl.Groups {
		total := group.TotalEntries()
		if total == 0 {
			continue
		}

		if i > 0 {
			fmt.Fprintln(w)
		}

		heading := group.VersionTitle()
		if date := group.FormattedDate(); date != "" {
			heading += " – " + date
		}

		fmt.Fprintf(w, "%s  (%d %s)\n\n",
			heading, total, pluralise(total, "entry", "entries"))

		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

		for _, tg := range group.TypeGroups {
			for _, e := range tg.Entries {
				relPath := displayPath(baseDir, e.Path)

				preview := e.Preview()
				if preview == "" {
					preview = relPath
				}
				preview = truncateLogPreview(preview, previewMaxLen)

				breaking := ""
				if e.Entry.Breaking {
					breaking = "  [breaking]"
				}

				fmt.Fprintf(tw, "  [%s]\t%s\t%s%s\n",
					relPath, string(e.Entry.Type), preview, breaking)
			}
		}

		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return nil
}

// RenderMarkdown writes a full changelog in standard Markdown format to w.
// Sections are grouped by version (newest first) then by change type.
func RenderMarkdown(cl *ChangeLog, w io.Writer) error {
	templateData := buildChangelogTemplateData(cl)
	b := bytes.NewBuffer(nil)
	if err := changelogTemplate.Execute(b, templateData); err != nil {
		return fmt.Errorf("render changelog template: %w", err)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

func pluralise(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func bodyBulletLines(body string) []string {
	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return nil
	}

	rawLines := strings.Split(trimmed, "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		content := strings.TrimSpace(line)
		if content == "" {
			continue
		}
		lines = append(lines, content)
	}

	return lines
}

func displayPath(baseDir string, path string) string {
	relPath, err := filepath.Rel(baseDir, path)
	if err != nil {
		return filepath.Base(path)
	}

	return relPath
}

func truncateLogPreview(value string, maxLen int) string {
	trimmed := strings.TrimSpace(value)
	if maxLen <= 3 {
		return trimmed
	}
	if len(trimmed) <= maxLen {
		return trimmed
	}

	return strings.TrimSpace(trimmed[:maxLen-3]) + "..."
}

func buildChangelogTemplateData(cl *ChangeLog) changelogTemplateData {
	groups := make([]changelogTemplateGroup, 0, len(cl.Groups))
	for _, group := range cl.Groups {
		if group.TotalEntries() == 0 {
			continue
		}

		templateGroup := changelogTemplateGroup{
			Heading: "## " + group.VersionTitle(),
		}

		for _, tg := range group.TypeGroups {
			if tg.ChangeType == changeentry.ChangeTypeDocs {
				docs := make([]string, 0, len(tg.Entries))
				for _, e := range tg.Entries {
					docs = append(docs, docBodyText(e))
				}
				templateGroup.DocsBlock = strings.Join(docs, "\n\n")
				continue
			}

			templateTypeGroup := changelogTemplateTypeGroup{Title: tg.Title}
			entries := make([]string, 0, len(tg.Entries))
			for _, e := range tg.Entries {
				entries = append(entries, renderBulletEntry(e))
			}
			templateTypeGroup.EntriesBlock = strings.Join(entries, "\n")

			if len(entries) > 0 {
				templateGroup.TypeGroups = append(templateGroup.TypeGroups, templateTypeGroup)
			}
		}

		groups = append(groups, templateGroup)
	}

	return changelogTemplateData{Groups: groups}
}

func docBodyText(entry EntryWithMeta) string {
	trimmed := strings.TrimSpace(entry.Entry.Body)
	if trimmed != "" {
		return trimmed
	}

	return filepath.Base(entry.Path)
}

func renderBulletEntry(entry EntryWithMeta) string {
	lines := bodyBulletLines(entry.Entry.Body)
	if len(lines) == 0 {
		lines = []string{filepath.Base(entry.Path)}
	}

	var firstLine strings.Builder
	if entry.Entry.Breaking {
		firstLine.WriteString("**Breaking** – ")
	}
	firstLine.WriteString(lines[0])
	if len(entry.Entry.Component) > 0 {
		firstLine.WriteString(" *(")
		firstLine.WriteString(strings.Join(entry.Entry.Component, ", "))
		firstLine.WriteString(")*")
	}

	builder := strings.Builder{}
	builder.WriteString("- ")
	builder.WriteString(firstLine.String())
	for _, line := range lines[1:] {
		builder.WriteString("\n  ")
		builder.WriteString(line)
	}

	return builder.String()
}
