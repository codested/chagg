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
		if date := group.FormattedDate(); date != "" {
			heading += " – " + date
		}

		fmt.Fprintln(w)
		fmt.Fprintln(w, heading)

		for _, tg := range group.TypeGroups {
			fmt.Fprintln(w)
			fmt.Fprintln(w, "### "+tg.Title)
			fmt.Fprintln(w)

			for _, e := range tg.Entries {
				var sb strings.Builder
				sb.WriteString("- ")

				if e.Entry.Breaking {
					sb.WriteString("**Breaking** – ")
				}

				preview := e.Preview()
				if preview == "" {
					preview = filepath.Base(e.Path)
				}
				sb.WriteString(preview)

				if len(e.Entry.Component) > 0 {
					sb.WriteString(" *(")
					sb.WriteString(strings.Join(e.Entry.Component, ", "))
					sb.WriteString(")*")
				}

				fmt.Fprintln(w, sb.String())
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
