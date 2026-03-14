package changeentry

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const ConfigFileName = ".chagg.yaml"

type ModuleConfig struct {
	Name       string
	ChangesDir string
	TagPrefix  string
}

type configFile struct {
	Modules []configModule `yaml:"modules"`
}

type configModule struct {
	Name       string `yaml:"name"`
	ChangesDir string `yaml:"changesDir"`
	TagPrefix  string `yaml:"tagPrefix"`
}

// ResolveModuleForChangesDir returns module configuration for the target changes directory.
// When .chagg.yaml is missing, a sensible single-module default is returned.
func ResolveModuleForChangesDir(repoRoot string, changesDir string) (ModuleConfig, error) {
	modules, hasConfig, err := loadModules(repoRoot)
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
		return ModuleConfig{}, NewValidationError("config", fmt.Sprintf("no module in %s matches changes directory %s", ConfigFileName, absChangesDir))
	}

	return defaultModuleForChangesDir(repoRoot, absChangesDir), nil
}

// ResolveModulesForChangesDirs maps every discovered changes directory to a module.
// If .chagg.yaml exists, each directory must be explicitly configured.
func ResolveModulesForChangesDirs(repoRoot string, changesDirs []string) (map[string]ModuleConfig, error) {
	modules, hasConfig, err := loadModules(repoRoot)
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
			return nil, NewValidationError("config", fmt.Sprintf("changes directory %s is not declared in %s", absChangesDir, ConfigFileName))
		}

		result[changesDir] = defaultModuleForChangesDir(repoRoot, absChangesDir)
	}

	return result, nil
}

func loadModules(repoRoot string) ([]ModuleConfig, bool, error) {
	configPath := filepath.Join(repoRoot, ConfigFileName)
	content, err := os.ReadFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var file configFile
	if err := yaml.Unmarshal(content, &file); err != nil {
		return nil, true, NewValidationError("config", fmt.Sprintf("invalid %s: %s", ConfigFileName, err))
	}

	modules := make([]ModuleConfig, 0, len(file.Modules))
	seenNames := map[string]bool{}
	seenDirs := map[string]bool{}

	for index, module := range file.Modules {
		name := strings.TrimSpace(module.Name)
		if name == "" {
			return nil, true, NewValidationError("config", fmt.Sprintf("modules[%d].name is required", index))
		}
		if seenNames[strings.ToLower(name)] {
			return nil, true, NewValidationError("config", fmt.Sprintf("duplicate module name %q", name))
		}

		changesDirRaw := strings.TrimSpace(module.ChangesDir)
		if changesDirRaw == "" {
			return nil, true, NewValidationError("config", fmt.Sprintf("modules[%d].changesDir is required", index))
		}

		cleanChangesDir := filepath.Clean(changesDirRaw)
		if filepath.IsAbs(cleanChangesDir) {
			return nil, true, NewValidationError("config", fmt.Sprintf("modules[%d].changesDir must be relative", index))
		}

		changesDirPath := filepath.Join(repoRoot, cleanChangesDir)
		if seenDirs[strings.ToLower(changesDirPath)] {
			return nil, true, NewValidationError("config", fmt.Sprintf("duplicate module changesDir %q", changesDirRaw))
		}

		tagPrefix := strings.TrimSpace(module.TagPrefix)
		modules = append(modules, ModuleConfig{
			Name:       name,
			ChangesDir: changesDirPath,
			TagPrefix:  tagPrefix,
		})
		seenNames[strings.ToLower(name)] = true
		seenDirs[strings.ToLower(changesDirPath)] = true
	}

	return modules, true, nil
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

	return ModuleConfig{Name: name, ChangesDir: changesDir}
}

func samePath(a string, b string) bool {
	cleanA := filepath.Clean(a)
	cleanB := filepath.Clean(b)
	return strings.EqualFold(cleanA, cleanB)
}
