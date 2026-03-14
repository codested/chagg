package changeentry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var ConfigFileNames = []string{".chagg.yaml", ".chagg.yml", "chagg.yml"}

type ModuleConfig struct {
	Name       string
	ChangesDir string
	TagPrefix  string
	Defaults   ModuleDefaults
	GitWrite   GitWritePolicy
}

type ModuleDefaults struct {
	AutoAddToGit bool
}

type GitWritePolicy struct {
	Enabled     bool
	Add         bool
	ReleaseTag  bool
	ReleasePush bool
}

func defaultModuleDefaults() ModuleDefaults {
	return ModuleDefaults{AutoAddToGit: true}
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
	Defaults ModuleDefaultsConfig `yaml:"defaults"`
	GitWrite GitWriteConfig       `yaml:"gitWrite"`
	Modules  []configModule       `yaml:"modules"`
}

type configModule struct {
	Name       string               `yaml:"name"`
	ChangesDir string               `yaml:"changesDir"`
	TagPrefix  string               `yaml:"tagPrefix"`
	Defaults   ModuleDefaultsConfig `yaml:"defaults"`
	GitWrite   GitWriteConfig       `yaml:"gitWrite"`
}

type ModuleDefaultsConfig struct {
	AutoAddToGit *bool `yaml:"autoAddToGit"`
}

type GitWriteConfig struct {
	Enabled     *bool `yaml:"enabled"`
	Add         *bool `yaml:"add"`
	ReleaseTag  *bool `yaml:"releaseTag"`
	ReleasePush *bool `yaml:"releasePush"`
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

	globalDefaults := applyDefaultsConfig(defaultModuleDefaults(), file.Defaults)
	globalGitWrite := applyGitWriteConfig(defaultGitWritePolicy(), file.GitWrite)

	modules := make([]ModuleConfig, 0, len(file.Modules))
	seenNames := map[string]bool{}
	seenDirs := map[string]bool{}

	for index, module := range file.Modules {
		name := strings.TrimSpace(module.Name)
		if name == "" {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("modules[%d].name is required", index))
		}
		if seenNames[strings.ToLower(name)] {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("duplicate module name %q", name))
		}

		changesDirRaw := strings.TrimSpace(module.ChangesDir)
		if changesDirRaw == "" {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("modules[%d].changesDir is required", index))
		}

		cleanChangesDir := filepath.Clean(changesDirRaw)
		if filepath.IsAbs(cleanChangesDir) {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("modules[%d].changesDir must be relative", index))
		}

		changesDirPath := filepath.Join(repoRoot, cleanChangesDir)
		if seenDirs[strings.ToLower(changesDirPath)] {
			return nil, true, filepath.Base(configPath), NewValidationError("config", fmt.Sprintf("duplicate module changesDir %q", changesDirRaw))
		}

		tagPrefix := strings.TrimSpace(module.TagPrefix)
		resolvedDefaults := applyDefaultsConfig(globalDefaults, module.Defaults)
		resolvedGitWrite := applyGitWriteConfig(globalGitWrite, module.GitWrite)
		modules = append(modules, ModuleConfig{
			Name:       name,
			ChangesDir: changesDirPath,
			TagPrefix:  tagPrefix,
			Defaults:   resolvedDefaults,
			GitWrite:   resolvedGitWrite,
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

func applyDefaultsConfig(base ModuleDefaults, overrides ModuleDefaultsConfig) ModuleDefaults {
	result := base
	if overrides.AutoAddToGit != nil {
		result.AutoAddToGit = *overrides.AutoAddToGit
	}

	return result
}

func applyGitWriteConfig(base GitWritePolicy, overrides GitWriteConfig) GitWritePolicy {
	result := base
	if overrides.Enabled != nil {
		result.Enabled = *overrides.Enabled
	}
	if overrides.Add != nil {
		result.Add = *overrides.Add
	}
	if overrides.ReleaseTag != nil {
		result.ReleaseTag = *overrides.ReleaseTag
	}
	if overrides.ReleasePush != nil {
		result.ReleasePush = *overrides.ReleasePush
	}

	return result
}

func defaultModuleForChangesDir(repoRoot string, changesDir string) ModuleConfig {
	relPath, err := filepath.Rel(repoRoot, changesDir)
	if err != nil {
		relPath = ".changes"
	}

	name := strings.TrimSuffix(relPath, string(filepath.Separator)+".changes")
	name = strings.TrimSuffix(name, ".changes")
	name = strings.Trim(name, string(filepath.Separator))
	if name == "" {
		name = "default"
	}
	name = strings.ReplaceAll(name, string(filepath.Separator), "-")

	return ModuleConfig{
		Name:       name,
		ChangesDir: changesDir,
		Defaults:   defaultModuleDefaults(),
		GitWrite:   defaultGitWritePolicy(),
	}
}

func samePath(a string, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	return strings.EqualFold(cleanA, cleanB)
}
