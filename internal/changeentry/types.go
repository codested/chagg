package changeentry

import "strings"

type ChangeType string

const (
	ChangeTypeFeature  ChangeType = "feature"
	ChangeTypeFix      ChangeType = "fix"
	ChangeTypeRemoval  ChangeType = "removal"
	ChangeTypeSecurity ChangeType = "security"
	ChangeTypeDocs     ChangeType = "docs"
)

var changeTypeAliases = map[string]ChangeType{
	"feature":  ChangeTypeFeature,
	"feat":     ChangeTypeFeature,
	"fix":      ChangeTypeFix,
	"bugfix":   ChangeTypeFix,
	"patch":    ChangeTypeFix,
	"removal":  ChangeTypeRemoval,
	"remove":   ChangeTypeRemoval,
	"security": ChangeTypeSecurity,
	"docs":     ChangeTypeDocs,
	"doc":      ChangeTypeDocs,
}

var supportedChangeTypes = []ChangeType{
	ChangeTypeFeature,
	ChangeTypeFix,
	ChangeTypeRemoval,
	ChangeTypeSecurity,
	ChangeTypeDocs,
}

func SupportedChangeTypeNames() []string {
	result := make([]string, 0, len(supportedChangeTypes))
	for _, value := range supportedChangeTypes {
		result = append(result, string(value))
	}

	return result
}

func TypeFlagUsage() string {
	return "Change type (feature, fix, removal, security, docs)"
}

func TypePrompt() string {
	return "Type (feature/fix/removal/security/docs): "
}

func NormalizeType(value string) (ChangeType, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "", NewValidationError("type", "type is required")
	}

	normalized, ok := changeTypeAliases[trimmed]
	if !ok {
		return "", NewValidationError("type", "unsupported type \""+value+"\", expected one of: "+strings.Join(SupportedChangeTypeNames(), ", "))
	}

	return normalized, nil
}
