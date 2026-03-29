package changeentry

import (
	"fmt"
	"sort"
	"strings"
)

// ChangeType is the canonical identifier of a change type (e.g. "feature", "fix").
type ChangeType string

// Well-known built-in type IDs.
const (
	ChangeTypeFeature  ChangeType = "feature"
	ChangeTypeFix      ChangeType = "fix"
	ChangeTypeRemoval  ChangeType = "removal"
	ChangeTypeSecurity ChangeType = "security"
	ChangeTypeDocs     ChangeType = "docs"
	ChangeTypeChore    ChangeType = "chore"
)

// TypeDefinition describes a change type: its canonical ID, recognised
// case-insensitive aliases, the section title in generated changelogs,
// the default SemVer bump level, and the display order (lower = earlier).
type TypeDefinition struct {
	ID          ChangeType
	Aliases     []string // lower-cased; does NOT include the ID itself
	Title       string
	DefaultBump BumpLevel
	Order       int
}

// builtinTypes is the ordered list of types shipped with chagg.
var builtinTypes = []TypeDefinition{
	{ID: ChangeTypeFeature, Aliases: []string{"feat", "enhancement"}, Title: "Features", DefaultBump: BumpLevelMinor, Order: 0},
	{ID: ChangeTypeFix, Aliases: []string{"bugfix", "patch"}, Title: "Bug Fixes", DefaultBump: BumpLevelPatch, Order: 1},
	{ID: ChangeTypeRemoval, Aliases: []string{"remove"}, Title: "Removals", DefaultBump: BumpLevelMinor, Order: 2},
	{ID: ChangeTypeSecurity, Aliases: []string{}, Title: "Security", DefaultBump: BumpLevelPatch, Order: 3},
	{ID: ChangeTypeDocs, Aliases: []string{"doc"}, Title: "Documentation", DefaultBump: BumpLevelPatch, Order: 4},
	{ID: ChangeTypeChore, Aliases: []string{"misc"}, Title: "Chores", DefaultBump: BumpLevelPatch, Order: 5},
}

// TypeRegistry is an immutable, resolved set of change types built from
// the layered configuration. Always create via DefaultTypeRegistry() or
// buildTypeRegistry(); the zero value has an empty lookup map and will
// only find types via the built-in fallback in DefaultBumpLevel.
type TypeRegistry struct {
	defs    []TypeDefinition
	byAlias map[string]ChangeType // key: lower-cased alias or id
}

// DefaultTypeRegistry returns a registry containing only the built-in types.
func DefaultTypeRegistry() TypeRegistry {
	return buildTypeRegistry(builtinTypes)
}

func buildTypeRegistry(defs []TypeDefinition) TypeRegistry {
	sorted := make([]TypeDefinition, len(defs))
	copy(sorted, defs)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Order < sorted[j].Order
	})

	byAlias := make(map[string]ChangeType, len(sorted)*3)
	for _, d := range sorted {
		byAlias[strings.ToLower(string(d.ID))] = d.ID
		for _, a := range d.Aliases {
			byAlias[strings.ToLower(a)] = d.ID
		}
	}

	return TypeRegistry{defs: sorted, byAlias: byAlias}
}

// NormalizeType resolves a raw string (alias or ID) to the canonical ChangeType.
// When the registry is uninitialized (zero value), it falls back to the built-in types.
func (r TypeRegistry) NormalizeType(value string) (ChangeType, error) {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return "", NewValidationError("type", "type is required")
	}
	effective := r
	if len(effective.byAlias) == 0 {
		effective = DefaultTypeRegistry()
	}
	ct, ok := effective.byAlias[trimmed]
	if !ok {
		return "", NewValidationError("type", fmt.Sprintf("unsupported type %q, expected one of: %s", value, strings.Join(effective.TypeNames(), ", ")))
	}
	return ct, nil
}

// DefaultBumpLevel returns the default bump level for the given change type.
// Falls back to the built-in definitions when the type is not in this registry,
// so that a zero-value registry still returns sensible defaults for built-in types.
func (r TypeRegistry) DefaultBumpLevel(ct ChangeType) BumpLevel {
	for _, d := range r.defs {
		if d.ID == ct {
			return d.DefaultBump
		}
	}
	// Fallback: consult built-ins for types not present in the registry.
	for _, d := range builtinTypes {
		if d.ID == ct {
			return d.DefaultBump
		}
	}
	return BumpLevelPatch
}

// TypeNames returns the canonical IDs of all registered types in display order.
// Falls back to built-in type names when the registry is uninitialized.
func (r TypeRegistry) TypeNames() []string {
	src := r.defs
	if len(src) == 0 {
		src = builtinTypes
	}
	names := make([]string, 0, len(src))
	for _, d := range src {
		names = append(names, string(d.ID))
	}
	return names
}

// Definitions returns the type definitions in display order.
// Falls back to built-in definitions when the registry is uninitialized.
func (r TypeRegistry) Definitions() []TypeDefinition {
	if len(r.defs) == 0 {
		return builtinTypes
	}
	return r.defs
}

// TypeFlagUsage returns the --type flag usage string.
func (r TypeRegistry) TypeFlagUsage() string {
	return "Change type (" + strings.Join(r.TypeNames(), ", ") + ")"
}

// TypePrompt returns the interactive prompt string for the type field.
func (r TypeRegistry) TypePrompt() string {
	return "Type (" + strings.Join(r.TypeNames(), "/") + "): "
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
