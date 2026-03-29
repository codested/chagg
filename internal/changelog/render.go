package changelog

import (
	"bytes"
	_ "embed"
	"encoding/json"
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

				bumpSuffix := ""
				if e.Entry.Bump != "" {
					bumpSuffix = "  [bump:" + string(e.Entry.Bump) + "]"
				}

				fmt.Fprintf(tw, "  [%s]\t%s\t%s%s\n",
					relPath, string(e.Entry.Type), preview, bumpSuffix)
			}
		}

		if err := tw.Flush(); err != nil {
			return err
		}
	}

	return nil
}

// LogHints contains version hint metadata for log JSON output.
type LogHints struct {
	LatestTag string
	NextTag   string
}

// ── Shared JSON types and builder ─────────────────────────────────────────────

type jsonEntry struct {
	ID        string   `json:"id"`
	Path      string   `json:"path"`
	Type      string   `json:"type"`
	Bump      string   `json:"bump,omitempty"`
	Component []string `json:"component,omitempty"`
	Audience  []string `json:"audience,omitempty"`
	Rank      int      `json:"rank,omitempty"`
	Issue     []string `json:"issue,omitempty"`
	Release   string   `json:"release,omitempty"`
	Preview   string   `json:"preview,omitempty"`
	Body      string   `json:"body,omitempty"`
}

type jsonResource struct {
	Path    string `json:"path"`
	EntryID string `json:"entry_id"`
}

type jsonTypeGroup struct {
	Type    string      `json:"type"`
	Title   string      `json:"title"`
	Entries []jsonEntry `json:"entries"`
}

type jsonGroup struct {
	Version   string          `json:"version"`
	Title     string          `json:"title"`
	Date      string          `json:"date,omitempty"`
	IsStaging bool            `json:"isStaging"`
	Types     []jsonTypeGroup `json:"types"`
}

// jsonBuildResult holds the output of buildJSONGroups: the version groups and
// all resources (images etc.) referenced by entry bodies.
type jsonBuildResult struct {
	Groups    []jsonGroup
	Resources []jsonResource
}

// buildJSONGroups converts changelog version groups into their JSON
// representation. This is the shared core used by both RenderJSON and
// RenderLogJSON. When baseDir is non-empty, relative image paths in entry
// bodies are rewritten to be relative to baseDir, and referenced resources are
// collected.
func buildJSONGroups(cl *ChangeLog, repoRoot string, baseDir string) jsonBuildResult {
	groups := make([]jsonGroup, 0, len(cl.Groups))
	var allResources []jsonResource

	for _, group := range cl.Groups {
		if group.TotalEntries() == 0 {
			continue
		}

		typeGroups := make([]jsonTypeGroup, 0, len(group.TypeGroups))
		for _, tg := range group.TypeGroups {
			entries := make([]jsonEntry, 0, len(tg.Entries))
			for _, entry := range tg.Entries {
				idPath := jsonIDPath(cl.Module, entry.Path)
				id := idPath + "@" + jsonEntryAddHash(entry)

				fullPreview := entry.Preview()
				truncatedPreview := truncateLogPreview(fullPreview, defaultLogPreviewMaxLen)
				remainingBody := entry.BodyWithoutPreviewLine()

				// Body = content after the preview line (heading stripped).
				// Also include the full body when the preview was truncated,
				// so consumers can show the complete first line.
				var body string
				if truncatedPreview != fullPreview {
					body = strings.TrimSpace(entry.Entry.Body)
				} else if remainingBody != "" {
					body = remainingBody
				}

				// Collect resources from the full raw body and rewrite
				// image paths in the JSON body field.
				if baseDir != "" {
					for _, r := range collectResources(entry.Entry.Body, entry.Path, id, baseDir) {
						allResources = append(allResources, jsonResource{
							Path:    r.RelPath,
							EntryID: r.EntryID,
						})
					}
					if body != "" {
						body = rewriteImagePaths(body, entry.Path, baseDir)
					}
				}

				entries = append(entries, jsonEntry{
					ID:        id,
					Path:      displayPath(repoRoot, entry.Path),
					Type:      string(entry.Entry.Type),
					Bump:      string(entry.Entry.Bump),
					Component: entry.Entry.Component,
					Audience:  entry.Entry.Audience,
					Rank:      entry.Entry.Rank,
					Issue:     entry.Entry.Issue,
					Release:   entry.Entry.Release,
					Preview:   truncatedPreview,
					Body:      body,
				})
			}

			typeGroups = append(typeGroups, jsonTypeGroup{
				Type:    string(tg.ChangeType),
				Title:   tg.Title,
				Entries: entries,
			})
		}

		groups = append(groups, jsonGroup{
			Version:   group.Version,
			Title:     group.VersionTitle(),
			Date:      group.FormattedDate(),
			IsStaging: group.IsStaging(),
			Types:     typeGroups,
		})
	}
	return jsonBuildResult{Groups: groups, Resources: allResources}
}

// RenderLogJSON writes a JSON representation of the log view (typically
// staging or a single version) to w.  hints may be nil when not applicable.
func RenderLogJSON(cl *ChangeLog, repoRoot string, hints *LogHints, w io.Writer) error {
	type jsonDocument struct {
		SchemaVersion int         `json:"schema_version"`
		Module        string      `json:"module"`
		LatestTag     string      `json:"latest_tag,omitempty"`
		NextTag       string      `json:"next_tag,omitempty"`
		Groups        []jsonGroup `json:"groups"`
	}

	result := buildJSONGroups(cl, repoRoot, "")
	doc := jsonDocument{SchemaVersion: 1, Module: cl.Module.Name, Groups: result.Groups}
	if hints != nil {
		doc.LatestTag = hints.LatestTag
		doc.NextTag = hints.NextTag
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

// RenderMarkdown writes a full changelog in standard Markdown format to w.
// Sections are grouped by version (newest first) then by change type.
// baseDir is used to rewrite relative image paths in entry bodies so they
// resolve correctly from the caller's working directory. Pass "" to skip
// image-path rewriting.
func RenderMarkdown(cl *ChangeLog, baseDir string, w io.Writer) error {
	templateData := buildChangelogTemplateData(cl, baseDir)
	b := bytes.NewBuffer(nil)
	if err := changelogTemplate.Execute(b, templateData); err != nil {
		return fmt.Errorf("render changelog template: %w", err)
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// RenderJSON writes a full changelog in JSON format to w. baseDir controls
// image-path rewriting and resource collection (pass "" to skip).
func RenderJSON(cl *ChangeLog, repoRoot string, baseDir string, w io.Writer) error {
	type jsonDocument struct {
		SchemaVersion int            `json:"schema_version"`
		Module        string         `json:"module"`
		Groups        []jsonGroup    `json:"groups"`
		Resources     []jsonResource `json:"resources,omitempty"`
	}

	result := buildJSONGroups(cl, repoRoot, baseDir)
	doc := jsonDocument{
		SchemaVersion: 1,
		Module:        cl.Module.Name,
		Groups:        result.Groups,
		Resources:     result.Resources,
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(doc)
}

func jsonIDPath(module changeentry.ModuleConfig, entryPath string) string {
	rel, err := filepath.Rel(module.ChangesDir, entryPath)
	if err != nil {
		rel = filepath.Base(entryPath)
	}

	rel = filepath.Clean(rel)
	if rel == "." || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(entryPath)
	}

	if module.Name != "" && !strings.EqualFold(module.Name, "default") {
		return filepath.ToSlash(filepath.Join(module.Name, rel))
	}

	return filepath.ToSlash(rel)
}

func jsonEntryAddHash(entry EntryWithMeta) string {
	hash := strings.TrimSpace(entry.AddedCommitHash)
	if hash == "" {
		return "untracked"
	}

	return hash
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

func buildChangelogTemplateData(cl *ChangeLog, baseDir string) changelogTemplateData {
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
					docs = append(docs, docBodyText(e, baseDir))
				}
				templateGroup.DocsBlock = strings.Join(docs, "\n\n")
				continue
			}

			templateTypeGroup := changelogTemplateTypeGroup{Title: tg.Title}
			entries := make([]string, 0, len(tg.Entries))
			for _, e := range tg.Entries {
				entries = append(entries, renderBulletEntry(e, baseDir))
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

func docBodyText(entry EntryWithMeta, baseDir string) string {
	trimmed := strings.TrimSpace(entry.Entry.Body)
	if trimmed == "" {
		return filepath.Base(entry.Path)
	}
	if baseDir != "" {
		trimmed = rewriteImagePaths(trimmed, entry.Path, baseDir)
	}
	return trimmed
}

func renderBulletEntry(entry EntryWithMeta, baseDir string) string {
	body := entry.Entry.Body
	if baseDir != "" {
		body = rewriteImagePaths(body, entry.Path, baseDir)
	}
	lines := bodyBulletLines(body)
	if len(lines) == 0 {
		lines = []string{filepath.Base(entry.Path)}
	}

	var firstLine strings.Builder
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
