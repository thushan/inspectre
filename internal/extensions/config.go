package extensions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the extensions configuration file
type Config struct {
	Version    int         `json:"version"`
	Extensions []Extension `json:"extensions"`
}

// LoadConfig reads the extensions configuration from a file
func LoadConfig(path string) (*Config, error) {
	// If path is empty, use default
	if path == "" {
		path = "configs/extensions.json"
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read extensions config: %w", err)
	}

	// Parse JSON
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse extensions config: %w", err)
	}

	return &config, nil
}

// LoadExtensions loads all extensions from the configuration file
func (m *Manager) LoadExtensionsFromConfig(configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	// Register each extension
	for _, ext := range config.Extensions {
		// If path is relative, make it relative to the extensions directory
		if !filepath.IsAbs(ext.Path) {
			ext.Path = filepath.Join(m.extensionsDir, ext.Path)
		}

		// Add to registry
		m.RegisterExtension(&ext)
	}

	return nil
}

// SaveConfig writes the extensions configuration to a file
func SaveConfig(config *Config, path string) error {
	// If path is empty, use default
	if path == "" {
		path = "configs/extensions.json"
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Marshal JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal extensions config: %w", err)
	}

	// Write file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write extensions config: %w", err)
	}

	return nil
}
