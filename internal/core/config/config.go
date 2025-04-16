package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

var (
	ErrConfigNotFound       = errors.New("configuration file not found")
	ErrInvalidConfig        = errors.New("invalid configuration")
	ErrDirectoryCreation    = errors.New("failed to create directory")
	DefaultConfigFolder     = "configs"
	DefaultRepositoriesFile = "repositories.json"
	DefaultPluginsFolder    = "plugins"
	DefaultTempFolder       = filepath.Join(os.TempDir(), "inspectre")
)

// Config represents the global configuration
type Config struct {
	// Base directories
	ConfigDir  string `json:"config_dir"`
	TempDir    string `json:"temp_dir"`
	PluginsDir string `json:"plugins_dir"`

	// Repository settings
	RepositoriesFile string `json:"repositories_file"`

	// Analysis settings
	Parallelism    int  `json:"parallelism"`
	CleanupOnExit  bool `json:"cleanup_on_exit"`
	VerboseLogging bool `json:"verbose_logging"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		ConfigDir:        DefaultConfigFolder,
		TempDir:          DefaultTempFolder,
		PluginsDir:       DefaultPluginsFolder,
		RepositoriesFile: filepath.Join(DefaultConfigFolder, DefaultRepositoriesFile),
		Parallelism:      runtime.NumCPU(),
		CleanupOnExit:    true,
		VerboseLogging:   false,
	}
}

// LoadConfig reads configuration from a file
func LoadConfig(path string) (*Config, error) {
	if path == "" {
		path = filepath.Join(DefaultConfigFolder, "config.json")
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Return default config if file doesn't exist
		return DefaultConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrConfigNotFound, err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	// Ensure minimal configuration
	if config.ConfigDir == "" {
		config.ConfigDir = DefaultConfigFolder
	}
	if config.TempDir == "" {
		config.TempDir = DefaultTempFolder
	}
	if config.PluginsDir == "" {
		config.PluginsDir = DefaultPluginsFolder
	}
	if config.RepositoriesFile == "" {
		config.RepositoriesFile = filepath.Join(DefaultConfigFolder, DefaultRepositoriesFile)
	}
	if config.Parallelism <= 0 {
		config.Parallelism = runtime.NumCPU()
	}

	return &config, nil
}

// SaveConfig writes configuration to a file
func SaveConfig(config *Config, path string) error {
	if path == "" {
		path = filepath.Join(DefaultConfigFolder, "config.json")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("%w: %v", ErrDirectoryCreation, err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// EnsureDirectories creates necessary directories if they don't exist
func EnsureDirectories(config *Config) error {
	dirs := []string{
		config.ConfigDir,
		config.TempDir,
		config.PluginsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("%w: %s - %v", ErrDirectoryCreation, dir, err)
		}
	}

	return nil
}
