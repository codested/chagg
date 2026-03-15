package changeentry

import (
	"fmt"
	"os"
	"path/filepath"
)

// ConfigIO abstracts reading and writing of chagg configuration files.
// The default implementation (FileConfigIO) uses the real filesystem.
// Tests can supply a MockConfigIO to avoid filesystem I/O entirely.
type ConfigIO interface {
	// ReadUserConfig reads the user-level config. Returns nil if absent.
	ReadUserConfig() (*RawConfig, error)
	// WriteUserConfig persists cfg to the user config file, creating parent dirs as needed.
	WriteUserConfig(cfg *RawConfig) error
	// UserConfigPath returns the resolved path to the user config file (may not exist).
	UserConfigPath() (string, error)
	// ReadRepoConfig reads the repo-level config. Returns (nil, "", nil) if absent.
	ReadRepoConfig(repoRoot string) (*RawConfig, string, error)
	// WriteRepoConfig persists cfg to the repo config file (overwrites existing or creates .chagg.yaml).
	// Returns the filename (base name only) that was written.
	WriteRepoConfig(repoRoot string, cfg *RawConfig) (string, error)
}

// ── FileConfigIO ─────────────────────────────────────────────────────────────

// FileConfigIO is the production ConfigIO that reads/writes the real filesystem.
type FileConfigIO struct{}

// NewFileConfigIO returns a ConfigIO backed by the real filesystem.
func NewFileConfigIO() ConfigIO { return FileConfigIO{} }

func (FileConfigIO) ReadUserConfig() (*RawConfig, error) {
	return loadRawUserConfig()
}

func (FileConfigIO) WriteUserConfig(cfg *RawConfig) error {
	path, err := resolveUserConfigPathForWrite()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	return writeRawConfig(path, cfg)
}

func (FileConfigIO) UserConfigPath() (string, error) {
	path, _, err := resolveUserConfigPath()
	return path, err
}

func (FileConfigIO) ReadRepoConfig(repoRoot string) (*RawConfig, string, error) {
	return loadRawRepoConfig(repoRoot)
}

func (FileConfigIO) WriteRepoConfig(repoRoot string, cfg *RawConfig) (string, error) {
	configPath, hasConfig, err := resolveConfigPath(repoRoot)
	if err != nil {
		return "", err
	}
	filename := ".chagg.yaml"
	if hasConfig {
		filename = filepath.Base(configPath)
	} else {
		configPath = filepath.Join(repoRoot, filename)
	}
	return filename, writeRawConfig(configPath, cfg)
}

// ── MockConfigIO ──────────────────────────────────────────────────────────────

// MockConfigIO implements ConfigIO with in-memory storage, suitable for unit tests.
// All fields are exported so tests can set up preconditions and inspect results directly.
type MockConfigIO struct {
	// Inputs
	UserCfg    *RawConfig
	UserCfgErr error
	RepoCfg    *RawConfig
	RepoName   string // defaults to ".chagg.yaml" when empty
	RepoCfgErr error
	UserPath   string // defaults to "/mock/home/.config/chagg/config.yaml"

	// Outputs (populated by Write calls)
	WrittenUserCfg *RawConfig
	WrittenRepoCfg *RawConfig
	WriteUserErr   error
	WriteRepoErr   error
}

func (m *MockConfigIO) ReadUserConfig() (*RawConfig, error) {
	return m.UserCfg, m.UserCfgErr
}

func (m *MockConfigIO) WriteUserConfig(cfg *RawConfig) error {
	m.WrittenUserCfg = cfg
	return m.WriteUserErr
}

func (m *MockConfigIO) UserConfigPath() (string, error) {
	if m.UserPath != "" {
		return m.UserPath, nil
	}
	return "/mock/home/.config/chagg/config.yaml", nil
}

func (m *MockConfigIO) ReadRepoConfig(repoRoot string) (*RawConfig, string, error) {
	name := m.RepoName
	if name == "" {
		name = ".chagg.yaml"
	}
	return m.RepoCfg, name, m.RepoCfgErr
}

func (m *MockConfigIO) WriteRepoConfig(repoRoot string, cfg *RawConfig) (string, error) {
	m.WrittenRepoCfg = cfg
	name := m.RepoName
	if name == "" {
		name = ".chagg.yaml"
	}
	return name, m.WriteRepoErr
}
