package changelog

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// RenderLog writes a human-readable, columnar overview of the changelog
// groups to w.  baseDir is used to compute display-friendly relative paths
// (pass the repository root or working directory).
func RenderLog(cl *ChangeLog, baseDir string, w io.Writer) error {
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

		for _, tg := range group.TypeGroups {
			for _, e := range tg.Entries {
				preview := e.Preview()
				if preview == "" {
					relPath, relErr := filepath.Rel(baseDir, e.Path)
					if relErr != nil {
						relPath = filepath.Base(e.Path)
					}
					preview = relPath
				}

				breaking := ""
				if e.Entry.Breaking {
					breaking = "  [breaking]"
				}

				fmt.Fprintf(w, "  %-10s  %s%s\n",
					string(e.Entry.Type), preview, breaking)
			}
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
