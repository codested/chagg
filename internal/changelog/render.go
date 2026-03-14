package changelog

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

const defaultLogPreviewMaxLen = 80

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
	fmt.Fprintln(w, "# Changelog")

	for _, group := range cl.Groups {
		if group.TotalEntries() == 0 {
			continue
		}

		heading := "## " + group.VersionTitle()

		fmt.Fprintln(w)
		fmt.Fprintln(w, heading)

		for _, tg := range group.TypeGroups {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "### "+tg.Title)
			fmt.Fprintln(w)

			for _, e := range tg.Entries {
				lines := bodyBulletLines(e.Entry.Body)
				if len(lines) == 0 {
					lines = []string{filepath.Base(e.Path)}
				}

				var firstLine strings.Builder
				if e.Entry.Breaking {
					firstLine.WriteString("**Breaking** – ")
				}
				firstLine.WriteString(lines[0])
				if len(e.Entry.Component) > 0 {
					firstLine.WriteString(" *(")
					firstLine.WriteString(strings.Join(e.Entry.Component, ", "))
					firstLine.WriteString(")*")
				}

				fmt.Fprintf(w, "- %s\n", firstLine.String())
				for _, line := range lines[1:] {
					fmt.Fprintf(w, "  %s\n", line)
				}
			}
		}
	}

	return nil
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
