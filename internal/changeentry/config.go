package changeentry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/codested/chagg/internal/gitutil"
	"gopkg.in/yaml.v3"
)

var ConfigFileNames = []string{".chagg.yaml", ".chagg.yml", "chagg.yml"}

const UserConfigEnvVar = "CHAGG_USER_CONFIG"

// Defaults holds resolved entry-field defaults for a module.
// A nil slice means "no default configured at this level".
// Rank defaults to 0 when not configured.
type Defaults struct {
	Audience  []string // applied when an entry omits the audience: field
	Rank      int      // default rank for new entries
	Component []string // applied when an entry omits the component: field
}

// ModuleConfig is the fully resolved configuration for a single changes module.
type ModuleConfig struct {
	Name       string
	ChangesDir string
	TagPrefix  string
	Defaults   Defaults
	Types      TypeRegistry
	GitWrite   GitWritePolicy
}

// GitWritePolicy controls which git write operations chagg is allowed to perform.
type GitWritePolicy struct {
	Enabled     bool
	Add         bool
	ReleaseTag  bool
	ReleasePush bool
}

func defaultGitWritePolicy() GitWritePolicy {
	// ReleasePush defaults to false: tags are created locally by default.
	// Set git.write.push-release-tag = true in config to push automatically.
	return GitWritePolicy{Enabled: true, Add: true, ReleaseTag: true, ReleasePush: false}
}

func (p GitWritePolicy) AllowsAdd() bool         { return p.Enabled && p.Add }
func (p GitWritePolicy) AllowsReleaseTag() bool  { return p.Enabled && p.ReleaseTag }
func (p GitWritePolicy) AllowsReleasePush() bool { return p.Enabled && p.ReleasePush }

// ── YAML raw structs ──────────────────────────────────────────────────────────

// RawConfig is the YAML schema shared by both the user config and the repo
// config.  The Modules field is only meaningful in the repo config.
type RawConfig struct {
	Defaults RawDefaults    `yaml:"defaults,omitempty"`
	Git      RawGit         `yaml:"git,omitempty"`
	Types    []rawTypeEntry `yaml:"types,omitempty"`
	Modules  []RawModule    `yaml:"modules,omitempty"`
}

// RawModule is a single entry in the repo config's modules list.
type RawModule struct {
	Name       string         `yaml:"name,omitempty"`
	ChangesDir string         `yaml:"changes-dir,omitempty"`
	TagPrefix  string         `yaml:"tag-prefix,omitempty"`
	Defaults   RawDefaults    `yaml:"defaults,omitempty"`
	Types      []rawTypeEntry `yaml:"types,omitempty"`
}

type RawDefaults struct {
	Audience  StringListConfig `yaml:"audience,omitempty"`
	Rank      *int             `yaml:"rank,omitempty"`
	Component StringListConfig `yaml:"component,omitempty"`
}

type rawTypeEntry struct {
	ID          string   `yaml:"id,omitempty"`
	Aliases     []string `yaml:"aliases,omitempty"`
	Title       string   `yaml:"title,omitempty"`
	DefaultBump string   `yaml:"default-bump,omitempty"`
	Order       *int     `yaml:"order,omitempty"`
}

type RawGit struct {
	Write RawGitWrite `yaml:"write,omitempty"`
}

type RawGitWrite struct {
	Allow      *bool          `yaml:"allow,omitempty"`
	Operations RawGitWriteOps `yaml:"operations,omitempty"`
}

type RawGitWriteOps struct {
	AddChange        *bool `yaml:"add-change,omitempty"`
	CreateReleaseTag *bool `yaml:"create-release-tag,omitempty"`
	PushReleaseTag   *bool `yaml:"push-release-tag,omitempty"`
}

// StringListConfig is a YAML type that accepts either a scalar string or a
// sequence of strings, and stays nil when the field is absent.
type StringListConfig []string

func (s *StringListConfig) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		trimmed := strings.TrimSpace(node.Value)
		if trimmed == "" {
			*s = []string{}
			return nil
		}
		*s = StringListConfig{trimmed}
		return nil
	case yaml.SequenceNode:
		var values []string
		if err := node.Decode(&values); err != nil {
			return err
		}
		result := make(StringListConfig, 0, len(values))
		for _, v := range values {
			if t := strings.TrimSpace(v); t != "" {
				result = append(result, t)
			}
		}
		*s = result
		return nil
	default:
		return fmt.Errorf("expected string or sequence, got %v", node.Tag)
	}
}

// ── Accumulated config state ──────────────────────────────────────────────────

// resolvedLayer accumulates settings as config layers are applied.
type resolvedLayer struct {
	types     []TypeDefinition
	defaults  Defaults
	gitPolicy GitWritePolicy
}

func newResolvedLayer() resolvedLayer {
	cp := make([]TypeDefinition, len(builtinTypes))
	copy(cp, builtinTypes)
	return resolvedLayer{types: cp, gitPolicy: defaultGitWritePolicy()}
}

func (l *resolvedLayer) applyGit(raw RawGitWrite) {
	l.gitPolicy = applyGitWrite(l.gitPolicy, raw)
}

func (l *resolvedLayer) applyDefaults(raw RawDefaults) {
	l.defaults = mergeDefaults(l.defaults, raw)
}

func (l *resolvedLayer) applyTypes(raw []rawTypeEntry) error {
	merged, err := mergeTypeList(l.types, raw)
	if err != nil {
		return err
	}
	l.types = merged
	return nil
}

// ── Public resolution API ─────────────────────────────────────────────────────

// ResolveModuleForChangesDir returns the fully resolved ModuleConfig for the
// target changes directory by merging: code defaults → user config → repo
// config → module-level config.
func ResolveModuleForChangesDir(repoRoot string, changesDir string) (ModuleConfig, error) {
	layer, repoCfg, configName, err := buildBaseLayer(repoRoot)
	if err != nil {
		return ModuleConfig{}, err
	}

	absChangesDir, err := filepath.Abs(changesDir)
	if err != nil {
		return ModuleConfig{}, err
	}

	name, tagPrefix := inferModuleIdentity(repoRoot, absChangesDir)

	if repoCfg != nil {
		rawMod, findErr := findRawModule(repoRoot, repoCfg.Modules, absChangesDir, configName)
		if findErr != nil {
			return ModuleConfig{}, findErr
		}

		if rawMod != nil {
			if err := layer.applyTypes(rawMod.Types); err != nil {
				return ModuleConfig{}, fmt.Errorf("%s module %q types: %w", configName, rawMod.Name, err)
			}
			layer.applyDefaults(rawMod.Defaults)
			if strings.TrimSpace(rawMod.Name) != "" {
				name = strings.TrimSpace(rawMod.Name)
			}
			if strings.TrimSpace(rawMod.TagPrefix) != "" {
				tagPrefix = strings.TrimSpace(rawMod.TagPrefix)
			}
		} else if hasExplicitModules(repoCfg) {
			return ModuleConfig{}, NewValidationError("config",
				fmt.Sprintf("no module in %s matches changes directory %s", configName, absChangesDir))
		} else {
			// No modules declared: validate auto-inferred names don't collide.
			discoveredDirs, discoverErr := gitutil.FindAllChangesDirs(repoRoot)
			if discoverErr != nil {
				return ModuleConfig{}, discoverErr
			}
			if !containsSamePath(discoveredDirs, absChangesDir) {
				discoveredDirs = append(discoveredDirs, absChangesDir)
			}
			if collisionErr := validateInferredModuleNames(repoRoot, discoveredDirs); collisionErr != nil {
				return ModuleConfig{}, collisionErr
			}
		}
	} else {
		// No repo config: validate auto-inferred names don't collide.
		discoveredDirs, discoverErr := gitutil.FindAllChangesDirs(repoRoot)
		if discoverErr != nil {
			return ModuleConfig{}, discoverErr
		}
		if !containsSamePath(discoveredDirs, absChangesDir) {
			discoveredDirs = append(discoveredDirs, absChangesDir)
		}
		if collisionErr := validateInferredModuleNames(repoRoot, discoveredDirs); collisionErr != nil {
			return ModuleConfig{}, collisionErr
		}
	}

	return ModuleConfig{
		Name:       name,
		ChangesDir: absChangesDir,
		TagPrefix:  tagPrefix,
		Defaults:   layer.defaults,
		Types:      buildTypeRegistry(layer.types),
		GitWrite:   layer.gitPolicy,
	}, nil
}

// ResolveModulesForChangesDirs maps every discovered changes directory to a
// fully resolved ModuleConfig.
func ResolveModulesForChangesDirs(repoRoot string, changesDirs []string) (map[string]ModuleConfig, error) {
	layer, repoCfg, configName, err := buildBaseLayer(repoRoot)
	if err != nil {
		return nil, err
	}

	hasConfig := repoCfg != nil && hasExplicitModules(repoCfg)

	result := make(map[string]ModuleConfig, len(changesDirs))
	for _, changesDir := range changesDirs {
		absChangesDir, absErr := filepath.Abs(changesDir)
		if absErr != nil {
			return nil, absErr
		}

		// Start from the base layer (copy so modules don't bleed into each other).
		ml := layer

		name, tagPrefix := inferModuleIdentity(repoRoot, absChangesDir)

		if repoCfg != nil {
			rawMod, findErr := findRawModule(repoRoot, repoCfg.Modules, absChangesDir, configName)
			if findErr != nil {
				return nil, findErr
			}

			if rawMod != nil {
				if err := ml.applyTypes(rawMod.Types); err != nil {
					return nil, fmt.Errorf("%s module %q types: %w", configName, rawMod.Name, err)
				}
				ml.applyDefaults(rawMod.Defaults)
				if strings.TrimSpace(rawMod.Name) != "" {
					name = strings.TrimSpace(rawMod.Name)
				}
				if strings.TrimSpace(rawMod.TagPrefix) != "" {
					tagPrefix = strings.TrimSpace(rawMod.TagPrefix)
				}
			} else if hasConfig {
				return nil, NewValidationError("config",
					fmt.Sprintf("changes directory %s is not declared in %s", absChangesDir, configName))
			}
		}

		result[changesDir] = ModuleConfig{
			Name:       name,
			ChangesDir: absChangesDir,
			TagPrefix:  tagPrefix,
			Defaults:   ml.defaults,
			Types:      buildTypeRegistry(ml.types),
			GitWrite:   ml.gitPolicy,
		}
	}

	if !hasConfig {
		if collisionErr := validateInferredModuleNames(repoRoot, changesDirs); collisionErr != nil {
			return nil, collisionErr
		}
	}

	return result, nil
}

// ── Layer construction ────────────────────────────────────────────────────────

// buildBaseLayer creates the accumulated config from code defaults → user
// config → repo root config, returning the layer and the raw repo config
// (nil when absent) for subsequent module-level processing.
func buildBaseLayer(repoRoot string) (resolvedLayer, *RawConfig, string, error) {
	layer := newResolvedLayer()

	// User config.
	userRaw, err := loadRawUserConfig()
	if err != nil {
		return resolvedLayer{}, nil, "", err
	}
	if userRaw != nil {
		layer.applyGit(userRaw.Git.Write)
		layer.applyDefaults(userRaw.Defaults)
		if err := layer.applyTypes(userRaw.Types); err != nil {
			return resolvedLayer{}, nil, "", fmt.Errorf("user config types: %w", err)
		}
	}

	// Repo config.
	repoCfg, configName, err := loadRawRepoConfig(repoRoot)
	if err != nil {
		return resolvedLayer{}, nil, "", err
	}
	if repoCfg != nil {
		layer.applyGit(repoCfg.Git.Write)
		layer.applyDefaults(repoCfg.Defaults)
		if err := layer.applyTypes(repoCfg.Types); err != nil {
			return resolvedLayer{}, nil, "", fmt.Errorf("%s types: %w", configName, err)
		}
	}

	return layer, repoCfg, configName, nil
}

// ── Raw config loaders ────────────────────────────────────────────────────────

func loadRawUserConfig() (*RawConfig, error) {
	path, hasConfig, err := resolveUserConfigPath()
	if err != nil {
		return nil, err
	}
	if !hasConfig {
		return nil, nil
	}
	return loadRawConfig(path, "user config")
}

func loadRawRepoConfig(repoRoot string) (*RawConfig, string, error) {
	configPath, hasConfig, err := resolveConfigPath(repoRoot)
	if err != nil {
		return nil, "", err
	}
	if !hasConfig {
		return nil, "", nil
	}

	configName := filepath.Base(configPath)
	cfg, err := loadRawConfig(configPath, configName)
	if err != nil {
		return nil, configName, err
	}
	return cfg, configName, nil
}

func loadRawConfig(path, label string) (*RawConfig, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg RawConfig
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, NewValidationError("config", fmt.Sprintf("invalid %s: %s", label, err))
	}
	return &cfg, nil
}

func resolveConfigPath(repoRoot string) (string, bool, error) {
	found := make([]string, 0, len(ConfigFileNames))
	for _, name := range ConfigFileNames {
		path := filepath.Join(repoRoot, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
	}
	if len(found) == 0 {
		return "", false, nil
	}
	if len(found) > 1 {
		names := make([]string, 0, len(found))
		for _, p := range found {
			names = append(names, filepath.Base(p))
		}
		return "", false, NewValidationError("config",
			fmt.Sprintf("multiple config files found (%s); keep only one of %s",
				strings.Join(names, ", "), strings.Join(ConfigFileNames, ", ")))
	}
	return found[0], true, nil
}

func resolveUserConfigPath() (string, bool, error) {
	if explicit := strings.TrimSpace(os.Getenv(UserConfigEnvVar)); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, true, nil
		} else if errors.Is(err, os.ErrNotExist) {
			return explicit, false, nil
		} else {
			return "", false, err
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", false, err
	}

	var path string
	if runtime.GOOS == "windows" {
		path = filepath.Join(homeDir, "AppData", "Roaming", "chagg", "config.yaml")
	} else {
		path = filepath.Join(homeDir, ".config", "chagg", "config.yaml")
	}

	if _, err := os.Stat(path); err == nil {
		return path, true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return path, false, nil
	} else {
		return "", false, err
	}
}

// ── Merge helpers ─────────────────────────────────────────────────────────────

func mergeDefaults(base Defaults, raw RawDefaults) Defaults {
	result := base
	if raw.Audience != nil {
		result.Audience = normalizeStringList([]string(raw.Audience))
	}
	if raw.Rank != nil {
		result.Rank = *raw.Rank
	}
	if raw.Component != nil {
		result.Component = normalizeStringList([]string(raw.Component))
	}
	return result
}

// mergeTypeList applies rawTypeEntry overrides onto an existing type list.
// Entries whose ID matches an existing type perform a partial update;
// entries with a new ID are appended.  Duplicate IDs within the overrides
// list, or aliases that conflict with another type's ID or aliases, are
// rejected.
func mergeTypeList(base []TypeDefinition, overrides []rawTypeEntry) ([]TypeDefinition, error) {
	result := make([]TypeDefinition, len(base))
	copy(result, base)

	idxByID := make(map[ChangeType]int, len(result))
	for i, d := range result {
		idxByID[d.ID] = i
	}

	seenOverrideIDs := make(map[ChangeType]bool, len(overrides))

	for _, ov := range overrides {
		id := ChangeType(strings.ToLower(strings.TrimSpace(ov.ID)))
		if id == "" {
			return nil, NewValidationError("config", "type entry is missing required 'id' field")
		}
		if seenOverrideIDs[id] {
			return nil, NewValidationError("config", fmt.Sprintf("duplicate type id %q in config", id))
		}
		seenOverrideIDs[id] = true

		if idx, exists := idxByID[id]; exists {
			// Partial override of an existing type.
			if len(ov.Aliases) > 0 {
				result[idx].Aliases = normalizeAliases(ov.Aliases)
			}
			if ov.Title != "" {
				result[idx].Title = ov.Title
			}
			if ov.DefaultBump != "" {
				bump, err := NormalizeBumpLevel(ov.DefaultBump)
				if err != nil {
					return nil, fmt.Errorf("type %q default-bump: %w", id, err)
				}
				result[idx].DefaultBump = bump
			}
			if ov.Order != nil {
				result[idx].Order = *ov.Order
			}
		} else {
			// New type.
			bump := BumpLevelPatch
			if ov.DefaultBump != "" {
				var err error
				bump, err = NormalizeBumpLevel(ov.DefaultBump)
				if err != nil {
					return nil, fmt.Errorf("type %q default-bump: %w", id, err)
				}
			}
			order := len(result) // default: append after existing
			if ov.Order != nil {
				order = *ov.Order
			}
			title := ov.Title
			if title == "" {
				title = capitalizeFirst(string(id))
			}
			result = append(result, TypeDefinition{
				ID:          id,
				Aliases:     normalizeAliases(ov.Aliases),
				Title:       title,
				DefaultBump: bump,
				Order:       order,
			})
			idxByID[id] = len(result) - 1
		}
	}

	// Validate that no two types share an alias.
	if err := validateTypeAliasConflicts(result); err != nil {
		return nil, err
	}

	return result, nil
}

func validateTypeAliasConflicts(defs []TypeDefinition) error {
	seen := make(map[string]ChangeType, len(defs)*3)
	for _, d := range defs {
		key := strings.ToLower(string(d.ID))
		if other, exists := seen[key]; exists && other != d.ID {
			return NewValidationError("config",
				fmt.Sprintf("type alias %q is claimed by both %q and %q", key, other, d.ID))
		}
		seen[key] = d.ID

		for _, a := range d.Aliases {
			lower := strings.ToLower(a)
			if other, exists := seen[lower]; exists && other != d.ID {
				return NewValidationError("config",
					fmt.Sprintf("type alias %q is claimed by both %q and %q", lower, other, d.ID))
			}
			seen[lower] = d.ID
		}
	}
	return nil
}

func applyGitWrite(base GitWritePolicy, raw RawGitWrite) GitWritePolicy {
	result := base
	if raw.Allow != nil {
		result.Enabled = *raw.Allow
		result.Add = *raw.Allow
		result.ReleaseTag = *raw.Allow
		result.ReleasePush = *raw.Allow
	}
	if raw.Operations.AddChange != nil {
		result.Add = *raw.Operations.AddChange
	}
	if raw.Operations.CreateReleaseTag != nil {
		result.ReleaseTag = *raw.Operations.CreateReleaseTag
	}
	if raw.Operations.PushReleaseTag != nil {
		result.ReleasePush = *raw.Operations.PushReleaseTag
	}
	return result
}

// ── Module lookup helpers ─────────────────────────────────────────────────────

// findRawModule locates the RawModule entry matching absChangesDir.
// Returns nil (no error) when the modules list is empty (unconfigured repo).
func findRawModule(repoRoot string, modules []RawModule, absChangesDir string, configName string) (*RawModule, error) {
	seenNames := map[string]bool{}
	seenDirs := map[string]bool{}

	for i, m := range modules {
		changesDirRaw := strings.TrimSpace(m.ChangesDir)
		if changesDirRaw == "" {
			return nil, NewValidationError("config",
				fmt.Sprintf("modules[%d].changes-dir is required", i))
		}
		clean := filepath.Clean(changesDirRaw)
		if filepath.IsAbs(clean) {
			return nil, NewValidationError("config",
				fmt.Sprintf("modules[%d].changes-dir must be relative", i))
		}
		resolved := filepath.Join(repoRoot, clean)
		lowerResolved := strings.ToLower(resolved)
		if seenDirs[lowerResolved] {
			return nil, NewValidationError("config",
				fmt.Sprintf("duplicate module changes-dir %q", changesDirRaw))
		}
		seenDirs[lowerResolved] = true

		inferredName, _ := inferModuleIdentity(repoRoot, resolved)
		name := strings.TrimSpace(m.Name)
		if name == "" {
			name = inferredName
		}
		if seenNames[strings.ToLower(name)] {
			return nil, NewValidationError("config",
				fmt.Sprintf("duplicate module name %q", name))
		}
		seenNames[strings.ToLower(name)] = true

		cp := modules[i]
		if samePath(resolved, absChangesDir) {
			return &cp, nil
		}
	}
	return nil, nil
}

func hasExplicitModules(cfg *RawConfig) bool {
	return cfg != nil && len(cfg.Modules) > 0
}

// ── Path / identity utilities ─────────────────────────────────────────────────

func inferModuleIdentity(repoRoot, changesDir string) (string, string) {
	rootChangesDir := filepath.Join(repoRoot, ".changes")
	if samePath(rootChangesDir, changesDir) {
		repoBase := filepath.Base(filepath.Clean(repoRoot))
		if repoBase == "" || repoBase == "." || repoBase == string(filepath.Separator) {
			repoBase = "default"
		}
		return repoBase, ""
	}

	parent := filepath.Base(filepath.Dir(changesDir))
	if parent == "" || parent == "." || parent == string(filepath.Separator) {
		parent = "default"
	}
	return parent, parent + "-"
}

func validateInferredModuleNames(repoRoot string, changesDirs []string) error {
	namesToDirs := map[string][]string{}
	for _, changesDir := range changesDirs {
		absChangesDir, err := filepath.Abs(changesDir)
		if err != nil {
			return err
		}
		name, _ := inferModuleIdentity(repoRoot, absChangesDir)
		key := strings.ToLower(name)
		namesToDirs[key] = append(namesToDirs[key], absChangesDir)
	}

	collisions := make([]string, 0)
	for key, dirs := range namesToDirs {
		if len(dirs) <= 1 {
			continue
		}
		sort.Strings(dirs)
		collisions = append(collisions, fmt.Sprintf("%s (%s)", key, strings.Join(dirs, ", ")))
	}
	if len(collisions) == 0 {
		return nil
	}
	sort.Strings(collisions)
	return NewValidationError("config",
		fmt.Sprintf("inferred module name collision detected: %s. Define explicit modules in config with unique names/tag-prefixes",
			strings.Join(collisions, "; ")))
}

func containsSamePath(paths []string, target string) bool {
	for _, path := range paths {
		if samePath(path, target) {
			return true
		}
	}
	return false
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

// ── Config write helpers ──────────────────────────────────────────────────────

// writeRawConfig marshals cfg to YAML and writes it to path.
func writeRawConfig(path string, cfg *RawConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}

// resolveUserConfigPathForWrite returns the user config path, always (even when
// the file does not yet exist), so callers can create it.
func resolveUserConfigPathForWrite() (string, error) {
	path, _, err := resolveUserConfigPath()
	return path, err
}

// ── String utilities ──────────────────────────────────────────────────────────

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, v := range values {
		if t := strings.TrimSpace(v); t != "" {
			result = append(result, t)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func normalizeAliases(aliases []string) []string {
	result := make([]string, 0, len(aliases))
	for _, a := range aliases {
		if t := strings.ToLower(strings.TrimSpace(a)); t != "" {
			result = append(result, t)
		}
	}
	return result
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
