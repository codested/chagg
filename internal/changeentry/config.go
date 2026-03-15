package changeentry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var ConfigFileNames = []string{".chagg.yaml", ".chagg.yml", "chagg.yml"}

const UserConfigEnvVar = "CHAGG_USER_CONFIG"

type ModuleConfig struct {
	Name            string
	ChangesDir      string
	TagPrefix       string
	DefaultAudience []string
	GitWrite        GitWritePolicy
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
	DefaultAudience audienceConfig `yaml:"default-audience"`
	Modules         []configModule `yaml:"modules"`
}

type configModule struct {
	Name       string `yaml:"name"`
	ChangesDir string `yaml:"changes-dir"`
	TagPrefix  string `yaml:"tag-prefix"`
}

type audienceConfig []string

func (a *audienceConfig) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		var value string
		if err := node.Decode(&value); err != nil {
			return err
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			*a = nil
			return nil
		}
		*a = []string{trimmed}
		return nil
	case yaml.SequenceNode:
		var values []string
		if err := node.Decode(&values); err != nil {
			return err
		}
		result := make([]string, 0, len(values))
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		*a = result
		return nil
	default:
		return fmt.Errorf("default-audience must be a string or list")
	}
}

type userConfigFile struct {
	Git userConfigGit `yaml:"git"`
}

type userConfigGit struct {
	Write userConfigGitWrite `yaml:"write"`
}

type userConfigGitWrite struct {
	Allow      *bool                 `yaml:"allow"`
	Operations userConfigGitWriteOps `yaml:"operations"`
}

type userConfigGitWriteOps struct {
	AddChange        *bool `yaml:"add-change"`
	CreateReleaseTag *bool `yaml:"create-release-tag"`
	PushReleaseTag   *bool `yaml:"push-release-tag"`
}

// ResolveModuleForChangesDir returns module configuration for the target changes directory.
// When .chagg.yaml is missing, a sensible single-module default is returned.
func ResolveModuleForChangesDir(repoRoot string, changesDir string) (ModuleConfig, error) {
	modules, hasConfig, configName, err := loadModules(repoRoot)
	if err != nil {
		return ModuleConfig{}, err
	}

	gitWritePolicy, err := loadUserGitWritePolicy()
	if err != nil {
		return ModuleConfig{}, err
	}

	absChangesDir, err := filepath.Abs(changesDir)
	if err != nil {
		return ModuleConfig{}, err
	}

	for _, module := range modules {
		if samePath(module.ChangesDir, absChangesDir) {
			module.GitWrite = gitWritePolicy
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

	module := defaultModuleForChangesDir(repoRoot, absChangesDir)
	module.GitWrite = gitWritePolicy
	return module, nil
}

// ResolveModulesForChangesDirs maps every discovered changes directory to a module.
// If .chagg.yaml exists, each directory must be explicitly configured.
func ResolveModulesForChangesDirs(repoRoot string, changesDirs []string) (map[string]ModuleConfig, error) {
	modules, hasConfig, configName, err := loadModules(repoRoot)
	if err != nil {
		return nil, err
	}

	gitWritePolicy, err := loadUserGitWritePolicy()
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
				module.GitWrite = gitWritePolicy
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

		module := defaultModuleForChangesDir(repoRoot, absChangesDir)
		module.GitWrite = gitWritePolicy
		result[changesDir] = module
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

	defaultAudience := normalizeAudience([]string(file.DefaultAudience))

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
			Name:            name,
			ChangesDir:      changesDirPath,
			TagPrefix:       tagPrefix,
			DefaultAudience: defaultAudience,
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

func loadUserGitWritePolicy() (GitWritePolicy, error) {
	policy := defaultGitWritePolicy()

	path, hasConfig, err := resolveUserConfigPath()
	if err != nil {
		return GitWritePolicy{}, err
	}
	if !hasConfig {
		return policy, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return GitWritePolicy{}, err
	}

	var cfg userConfigFile
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return GitWritePolicy{}, NewValidationError("config", fmt.Sprintf("invalid user config %s: %s", path, err))
	}

	return applyUserGitWriteConfig(policy, cfg.Git.Write), nil
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

func applyUserGitWriteConfig(base GitWritePolicy, cfg userConfigGitWrite) GitWritePolicy {
	result := base

	if cfg.Allow != nil {
		result.Enabled = *cfg.Allow
		result.Add = *cfg.Allow
		result.ReleaseTag = *cfg.Allow
		result.ReleasePush = *cfg.Allow
	}

	if cfg.Operations.AddChange != nil {
		result.Add = *cfg.Operations.AddChange
	}
	if cfg.Operations.CreateReleaseTag != nil {
		result.ReleaseTag = *cfg.Operations.CreateReleaseTag
	}
	if cfg.Operations.PushReleaseTag != nil {
		result.ReleasePush = *cfg.Operations.PushReleaseTag
	}

	return result
}

func defaultModuleForChangesDir(repoRoot string, changesDir string) ModuleConfig {
	name, tagPrefix := inferModuleIdentity(repoRoot, changesDir)

	return ModuleConfig{
		Name:            name,
		ChangesDir:      changesDir,
		TagPrefix:       tagPrefix,
		DefaultAudience: nil,
		GitWrite:        defaultGitWritePolicy(),
	}
}

func normalizeAudience(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	if len(result) == 0 {
		return nil
	}

	return result
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
