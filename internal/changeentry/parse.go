package changeentry

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontMatter holds the raw YAML fields from a change entry file header.
type frontMatter struct {
	Breaking  bool         `yaml:"breaking"`
	Component stringOrList `yaml:"component"`
	Audience  stringOrList `yaml:"audience"`
	Rank      int          `yaml:"rank"`
	Priority  int          `yaml:"priority"`
	Issue     stringOrList `yaml:"issue"`
	Release   string       `yaml:"release"`
}

// stringOrList is a YAML type that accepts either a scalar string or a sequence of strings.
type stringOrList []string

func (s *stringOrList) UnmarshalYAML(value *yaml.Node) error {
	switch value.Kind {
	case yaml.ScalarNode:
		trimmed := strings.TrimSpace(value.Value)
		if trimmed == "" {
			*s = nil
			return nil
		}
		*s = stringOrList{trimmed}
		return nil
	case yaml.SequenceNode:
		var items []string
		if err := value.Decode(&items); err != nil {
			return err
		}
		result := make(stringOrList, 0, len(items))
		for _, item := range items {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		*s = result
		return nil
	default:
		return fmt.Errorf("expected string or sequence, got %v", value.Tag)
	}
}

type frontMatterParts struct {
	header string
	body   string
}

func splitFrontMatter(content string) (frontMatterParts, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")

	const delimiter = "---"

	if !strings.HasPrefix(content, delimiter+"\n") {
		return frontMatterParts{body: content}, nil
	}

	rest := content[len(delimiter)+1:] // skip "---\n"

	closingIdx := strings.Index(rest, "\n"+delimiter)
	if closingIdx == -1 {
		return frontMatterParts{}, NewValidationError("format", "unclosed YAML front-matter (missing closing ---)")
	}

	header := rest[:closingIdx]
	body := rest[closingIdx+len("\n"+delimiter):]
	body = strings.TrimPrefix(body, "\n")

	return frontMatterParts{header: header, body: body}, nil
}

// InferTypeFromFilename resolves a change type from the filename prefix.
// Accepted patterns are case-insensitive and include one or two underscores,
// e.g. feat__login.md, FEAT__login.md, Feat_login.md.
func InferTypeFromFilename(path string) (ChangeType, error) {
	base := strings.ToLower(strings.TrimSpace(filepath.Base(path)))
	if !strings.HasSuffix(base, ".md") {
		return "", NewValidationError("path", fmt.Sprintf("change filename must end with .md: %s", filepath.Base(path)))
	}

	name := strings.TrimSuffix(base, ".md")
	sep := strings.Index(name, "_")
	if sep <= 0 {
		return "", NewValidationError("path", fmt.Sprintf("change filename must start with <type>__ or <type>_: %s", filepath.Base(path)))
	}

	typePrefix := strings.TrimSpace(name[:sep])
	if typePrefix == "" {
		return "", NewValidationError("path", fmt.Sprintf("change filename must include a type prefix: %s", filepath.Base(path)))
	}

	changeType, err := NormalizeType(typePrefix)
	if err != nil {
		return "", NewValidationError("path", fmt.Sprintf("unsupported filename type prefix %q in %s", typePrefix, filepath.Base(path)))
	}

	return changeType, nil
}

// ParseEntry parses the content of a change entry file.
// It returns the parsed Entry and a slice of validation errors encountered.
// If the YAML structure is invalid, a single error is returned.
func ParseEntry(content string, path string) (Entry, []error) {
	return ParseEntryWithDefaults(content, path, nil)
}

func ParseEntryWithDefaults(content string, path string, defaultAudience []string) (Entry, []error) {
	changeType, typeErr := InferTypeFromFilename(path)
	if typeErr != nil {
		return Entry{}, []error{typeErr}
	}

	parts, err := splitFrontMatter(content)
	if err != nil {
		return Entry{}, []error{err}
	}

	var fm frontMatter
	if strings.TrimSpace(parts.header) != "" {
		if yamlErr := yaml.Unmarshal([]byte(parts.header), &fm); yamlErr != nil {
			return Entry{}, []error{NewValidationError("format", fmt.Sprintf("invalid YAML front-matter: %s", yamlErr))}
		}
	}

	audience := []string(fm.Audience)
	if len(audience) == 0 {
		audience = append([]string(nil), defaultAudience...)
	}

	rank := fm.Rank
	if rank == 0 {
		rank = fm.Priority
	}

	return Entry{
		Type:      changeType,
		Breaking:  fm.Breaking,
		Component: fm.Component,
		Audience:  audience,
		Priority:  rank,
		Issue:     fm.Issue,
		Release:   strings.TrimSpace(fm.Release),
		Body:      strings.TrimSpace(parts.body),
	}, nil
}
