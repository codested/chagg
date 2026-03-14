package changeentry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var ConfigFileNames = []string{".chagg.yaml", ".chagg.yml", "chagg.yml"}

type ModuleConfig struct {
	Name       string
	ChangesDir string
	TagPrefix  string
	GitWrite   GitWritePolicy
}

type GitWritePolicy struct {
	Enabled     bool
	Add         bool
	ReleaseTag  bool
	ReleasePush bool
}

func defaultGitWritePolicy() GitWritePolicy {
	return GitWritePolicy{
		Enabled:     true,
		Add:         true,
		ReleaseTag:  true,
		ReleasePush: true,
	}
}

func (p GitWritePolicy) AllowsAdd() bool {
	return p.Enabled && p.Add
}

func (p GitWritePolicy) AllowsReleaseTag() bool {
	return p.Enabled && p.ReleaseTag
}

func (p GitWritePolicy) AllowsReleasePush() bool {
	return p.Enabled && p.ReleasePush
}

type configFile struct {
	GitWrite GitWriteConfig `yaml:"git-write"`
	Modules  []configModule `yaml:"modules"`
}

type configModule struct {
	Name       string `yaml:"name"`
	ChangesDir string `yaml:"changes-dir"`
	TagPrefix  string `yaml:"tag-prefix"`
}

type GitWriteConfig struct {
	Allow GitWriteAllowConfig `yaml:"allow"`
}

// GitWriteAllowConfig accepts either:
//   - allow: true|false
//   - allow:
//     add-change: true|false
//     push-release-tag: true|false
type GitWriteAllowConfig struct {
	IsSet          bool
	Value          bool
	AddChange      *bool
	PushReleaseTag *bool
}

func (c *GitWriteAllowConfig) UnmarshalYAML(node *yaml.Node) error {
	c.IsSet = true

	if node.Kind == yaml.ScalarNode {
		var value bool
		if err := node.Decode(&value); err != nil {
			return err
		}
		c.Value = value
		return nil
	}

	if node.Kind == yaml.MappingNode {
		var value struct {
			AddChange      *bool `yaml:"add-change"`
			PushReleaseTag *bool `yaml:"push-release-tag"`
		}
		if err := node.Decode(&value); err != nil {
			return err
		}

		c.Value = true
		c.AddChange = value.AddChange
		c.PushReleaseTag = value.PushReleaseTag
		return nil
	}

	return fmt.Errorf("allow must be a boolean or object")
}

// ResolveModuleForChangesDir returns module configuration for the target changes directory.
// When .chagg.yaml is missing, a sensible single-module default is returned.
func ResolveModuleForChangesDir(repoRoot string, changesDir string) (ModuleConfig, error) {
	modules, hasConfig, configName, err := loadModules(repoRoot)
	if err != nil {
		return ModuleConfig{}, err
	}

	absChangesDir, err := filepath.Abs(changesDir)
	if err != nil {
		return ModuleConfig{}, err
	}

	for _, module := range modules {
		if samePath(module.ChangesDir, absChangesDir) {
			return module, nil
		}
	}

	if hasConfig {
		return ModuleConfig{}, NewValidationError("config", fmt.Sprintf("no module in %s matches changes directory %s", configName, absChangesDir))
	}

	discoveredDirs, discoverErr := FindAllChangesDirs(repoRoot)
	if discoverErr != nil {
		return ModuleConfig{}, discoverErr
	}
	if !containsSamePath(discoveredDirs, absChangesDir) {
		discoveredDirs = append(discoveredDirs, absChangesDir)
	}
	if collisionErr := validateInferredModuleNames(repoRoot, discoveredDirs); collisionErr != nil {
		return ModuleConfig{}, collisionErr
	}

	return defaultModuleForChangesDir(repoRoot, absChangesDir), nil
}

// ResolveModulesForChangesDirs maps every discovered changes directory to a module.
// If .chagg.yaml exists, each directory must be explicitly configured.
func ResolveModulesForChangesDirs(repoRoot string, changesDirs []string) (map[string]ModuleConfig, error) {
	modules, hasConfig, configName, err := loadModules(repoRoot)
	if err != nil {
		return nil, err
	}

	result := make(map[string]ModuleConfig, len(changesDirs))
	for _, changesDir := range changesDirs {
		absChangesDir, absErr := filepath.Abs(changesDir)
		if absErr != nil {
			return nil, absErr
		}

		matched := false
		for _, module := range modules {
			if samePath(module.ChangesDir, absChangesDir) {
				result[changesDir] = module
				matched = true
				break
			}
		}

		if matched {
			continue
		}

		if hasConfig {
			return nil, NewValidationError("config", fmt.Sprintf("changes directory %s is not declared in %s", absChangesDir, configName))
		}

		result[changesDir] = defaultModuleForChangesDir(repoRoot, absChangesDir)
	}

	if !hasConfig {
		if collisionErr := validateInferredModuleNames(repoRoot, changesDirs); collisionErr != nil {
			return nil, collisionErr
		}
	}

	return result, nil
}

func loadModules(repoRoot string) ([]ModuleConfig, bool, string, error) {
	configPath, hasConfig, err := resolveConfigPath(repoRoot)
	if err != nil {
		return nil, false, "", err
	}
	if !hasConfig {
		return nil, false, "", nil
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, false, "", err
	}

	var file configFile
	if err := yaml.Unmarshal(content, &file); err != nil {
		return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("invalid %s: %s", filepath.Base(configPath), err))
	}

	globalGitWrite := applyGitWriteConfig(defaultGitWritePolicy(), file.GitWrite)

	modules := make([]ModuleConfig, 0, len(file.Modules))
	seenNames := map[string]bool{}
	seenDirs := map[string]bool{}

	for index, module := range file.Modules {
		changesDirRaw := strings.TrimSpace(module.ChangesDir)
		if changesDirRaw == "" {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("modules[%d].changes-dir is required", index))
		}

		cleanChangesDir := filepath.Clean(changesDirRaw)
		if filepath.IsAbs(cleanChangesDir) {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("modules[%d].changes-dir must be relative", index))
		}

		changesDirPath := filepath.Join(repoRoot, cleanChangesDir)
		if seenDirs[strings.ToLower(changesDirPath)] {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("duplicate module changes-dir %q", changesDirRaw))
		}

		inferredName, inferredTagPrefix := inferModuleIdentity(repoRoot, changesDirPath)
		name := strings.TrimSpace(module.Name)
		if name == "" {
			name = inferredName
		}
		if seenNames[strings.ToLower(name)] {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("duplicate module name %q", name))
		}

		tagPrefix := strings.TrimSpace(module.TagPrefix)
		if tagPrefix == "" {
			tagPrefix = inferredTagPrefix
		}
		modules = append(modules, ModuleConfig{
			Name:       name,
			ChangesDir: changesDirPath,
			TagPrefix:  tagPrefix,
			GitWrite:   globalGitWrite,
		})
		seenNames[strings.ToLower(name)] = true
		seenDirs[strings.ToLower(changesDirPath)] = true
	}

	return modules, true, filepath.Base(configPath), nil
}

func resolveConfigPath(repoRoot string) (string, bool, error) {
	found := make([]string, 0, len(ConfigFileNames))
	for _, name := range ConfigFileNames {
		path := filepath.Join(repoRoot, name)
		if _, err := os.Stat(path); err == nil {
			found = append(found, path)
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", false, err
		}
	}

	if len(found) == 0 {
		return "", false, nil
	}

	if len(found) > 1 {
		names := make([]string, 0, len(found))
		for _, path := range found {
			names = append(names, filepath.Base(path))
		}
		return "", false, NewValidationError("config", fmt.Sprintf("multiple config files found (%s); keep only one of %s", strings.Join(names, ", "), strings.Join(ConfigFileNames, ", ")))
	}

	return found[0], true, nil
}

func applyGitWriteConfig(base GitWritePolicy, overrides GitWriteConfig) GitWritePolicy {
	result := base
	if !overrides.Allow.IsSet {
		return result
	}

	if overrides.Allow.AddChange == nil && overrides.Allow.PushReleaseTag == nil {
		result.Enabled = overrides.Allow.Value
		result.Add = overrides.Allow.Value
		result.ReleaseTag = overrides.Allow.Value
		result.ReleasePush = overrides.Allow.Value
		return result
	}

	result.Enabled = true
	result.ReleaseTag = true
	if overrides.Allow.AddChange != nil {
		result.Add = *overrides.Allow.AddChange
	}
	if overrides.Allow.PushReleaseTag != nil {
		result.ReleasePush = *overrides.Allow.PushReleaseTag
	}

	return result
}

func defaultModuleForChangesDir(repoRoot string, changesDir string) ModuleConfig {
	name, tagPrefix := inferModuleIdentity(repoRoot, changesDir)

	return ModuleConfig{
		Name:       name,
		ChangesDir: changesDir,
		TagPrefix:  tagPrefix,
		GitWrite:   defaultGitWritePolicy(),
	}
}

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
	return NewValidationError(
		"config",
		fmt.Sprintf("inferred module name collision detected: %s. Define explicit modules in config with unique names/tag-prefixes", strings.Join(collisions, "; ")),
	)
}

func containsSamePath(paths []string, target string) bool {
	for _, path := range paths {
		if samePath(path, target) {
			return true
		}
	}

	return false
}

func samePath(a string, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	return strings.EqualFold(cleanA, cleanB)
}
