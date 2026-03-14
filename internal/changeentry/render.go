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
	Type         string
	Breaking     bool
	Component    []string
	Audience     []string
	ShowAudience bool
	Priority     int
	ShowPriority bool
	Issue        []string
	Release      string
	Body         string
}

var entryTemplate = template.Must(template.New("entry.md.tmpl").Funcs(template.FuncMap{
	"yamlField": yamlField,
}).Parse(entryTemplateSource))

func RenderEntry(entry Entry) (string, error) {
	data := entryTemplateData{
		Type:         string(entry.Type),
		Breaking:     entry.Breaking,
		Component:    entry.Component,
		Audience:     entry.Audience,
		ShowAudience: !isDefaultAudience(entry.Audience),
		Priority:     entry.Priority,
		ShowPriority: entry.Priority != 0,
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
	if len(values) != 1 {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(values[0]), DefaultAudience)
}
