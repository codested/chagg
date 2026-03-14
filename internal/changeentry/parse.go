package changeentry

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// frontMatter holds the raw YAML fields from a change entry file header.
type frontMatter struct {
	Type      string       `yaml:"type"`
	Breaking  bool         `yaml:"breaking"`
	Component stringOrList `yaml:"component"`
	Audience  stringOrList `yaml:"audience"`
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
		return frontMatterParts{}, NewValidationError("format", "missing YAML front-matter (file must start with ---)")
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

// ParseEntry parses the content of a change entry file.
// It returns the parsed Entry and a slice of validation errors encountered.
// If the YAML structure is invalid, a single error is returned.
func ParseEntry(content string) (Entry, []error) {
	parts, err := splitFrontMatter(content)
	if err != nil {
		return Entry{}, []error{err}
	}

	var fm frontMatter
	if yamlErr := yaml.Unmarshal([]byte(parts.header), &fm); yamlErr != nil {
		return Entry{}, []error{NewValidationError("format", fmt.Sprintf("invalid YAML front-matter: %s", yamlErr))}
	}

	var errs []error

	changeType, typeErr := NormalizeType(fm.Type)
	if typeErr != nil {
		errs = append(errs, typeErr)
	}

	if len(errs) > 0 {
		return Entry{}, errs
	}

	audience := []string(fm.Audience)
	if len(audience) == 0 {
		audience = []string{DefaultAudience}
	}

	return Entry{
		Type:      changeType,
		Breaking:  fm.Breaking,
		Component: []string(fm.Component),
		Audience:  audience,
		Priority:  fm.Priority,
		Issue:     []string(fm.Issue),
		Release:   strings.TrimSpace(fm.Release),
		Body:      strings.TrimSpace(parts.body),
	}, nil
}
