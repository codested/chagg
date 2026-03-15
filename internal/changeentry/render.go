package changeentry

import (
	"bytes"
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/entry.md.tmpl
var entryTemplateSource string

type entryTemplateData struct {
	ShowHeader   bool
	Bump         string
	ShowBump     bool
	Component    []string
	Audience     []string
	ShowAudience bool
	Rank         int
	ShowRank     bool
	Issue        []string
	Release      string
	Body         string
}

var entryTemplate = template.Must(template.New("entry.md.tmpl").Funcs(template.FuncMap{
	"yamlField": yamlField,
}).Parse(entryTemplateSource))

func RenderEntry(entry Entry) (string, error) {
	showBump := entry.Bump != ""
	data := entryTemplateData{
		ShowHeader:   showBump || len(entry.Component) > 0 || !isDefaultAudience(entry.Audience) || entry.Rank != 0 || len(entry.Issue) > 0 || strings.TrimSpace(entry.Release) != "",
		Bump:         string(entry.Bump),
		ShowBump:     showBump,
		Component:    entry.Component,
		Audience:     entry.Audience,
		ShowAudience: !isDefaultAudience(entry.Audience),
		Rank:         entry.Rank,
		ShowRank:     entry.Rank != 0,
		Issue:        entry.Issue,
		Release:      entry.Release,
		Body:         entry.Body,
	}

	builder := bytes.NewBuffer(nil)
	if err := entryTemplate.Execute(builder, data); err != nil {
		return "", fmt.Errorf("render change entry template: %w", err)
	}

	return builder.String(), nil
}

func yamlField(name string, values []string) string {
	if len(values) == 0 {
		return ""
	}

	if len(values) == 1 {
		return fmt.Sprintf("%s: %s\n", name, values[0])
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("%s:\n", name))
	for _, value := range values {
		builder.WriteString(fmt.Sprintf("  - %s\n", value))
	}

	return builder.String()
}

func isDefaultAudience(values []string) bool {
	return len(values) == 0
}
