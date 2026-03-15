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

// BumpLevel represents an explicit semver bump level override for a change entry.
// An empty BumpLevel means "use the type-based default".
type BumpLevel string

const (
	BumpLevelMajor BumpLevel = "major"
	BumpLevelMinor BumpLevel = "minor"
	BumpLevelPatch BumpLevel = "patch"
)

var bumpLevelAliases = map[string]BumpLevel{
	"major": BumpLevelMajor,
	"minor": BumpLevelMinor,
	"patch": BumpLevelPatch,
}

// defaultBumpLevels maps each change type to its default semver bump level.
var defaultBumpLevels = map[ChangeType]BumpLevel{
	ChangeTypeFeature:  BumpLevelMinor,
	ChangeTypeFix:      BumpLevelPatch,
	ChangeTypeRemoval:  BumpLevelMinor,
	ChangeTypeSecurity: BumpLevelPatch,
	ChangeTypeDocs:     BumpLevelPatch,
}

// DefaultBumpLevel returns the default semver bump level for the given change type.
func DefaultBumpLevel(ct ChangeType) BumpLevel {
	if level, ok := defaultBumpLevels[ct]; ok {
		return level
	}
	return BumpLevelPatch
}

// NormalizeBumpLevel validates and normalises a bump level string.
// An empty string is accepted and returns an empty BumpLevel (use type default).
func NormalizeBumpLevel(value string) (BumpLevel, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "", nil
	}
	level, ok := bumpLevelAliases[trimmed]
	if !ok {
		return "", NewValidationError("bump", "unsupported bump level \""+value+"\", expected one of: major, minor, patch")
	}
	return level, nil
}

// BumpFlagUsage returns the usage string for the --bump CLI flag.
func BumpFlagUsage() string {
	return "Override version bump level (major, minor, patch); defaults to type-based level"
}

// BumpPrompt returns the interactive prompt string for the bump override field.
func BumpPrompt() string {
	return "Bump override (major/minor/patch, leave empty to use type default): "
}
