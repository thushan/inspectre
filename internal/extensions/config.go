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
		// First try current working directory
		cwdPath := "configs/extensions.json"
		if _, err := os.Stat(cwdPath); err == nil {
			path = cwdPath
		} else {
			// Try executable directory as second option
			execPath, err := os.Executable()
			if err == nil {
				execDir := filepath.Dir(execPath)
				path = filepath.Join(execDir, "configs", "extensions.json")
			} else {
				// Fall back to current directory regardless
				path = cwdPath
			}
		}
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

// LoadExtensionsFromConfig loads all extensions from the configuration file
func (m *Manager) LoadExtensionsFromConfig(configPath string) error {
	config, err := LoadConfig(configPath)
	if err != nil {
		return err
	}

	// Register each extension
	for _, ext := range config.Extensions {
		extCopy := ext // Create a copy to avoid modifying the original

		// Store original path for reference
		originalPath := extCopy.Path

		// If path is relative, make it relative to the extensions directory
		if !filepath.IsAbs(extCopy.Path) {
			extCopy.Path = filepath.Join(m.extensionsDir, extCopy.Path)
		}

		// Verify the extension exists
		if _, err := os.Stat(extCopy.Path); os.IsNotExist(err) {
			// Try executable directory as fallback for plugins
			if execPath, err := os.Executable(); err == nil {
				execDir := filepath.Dir(execPath)
				altPath := filepath.Join(execDir, originalPath)

				if _, err := os.Stat(altPath); err == nil {
					extCopy.Path = altPath
				} else {
					// Also try with plugins dir
					altPath = filepath.Join(execDir, "plugins", filepath.Base(originalPath))
					if _, err := os.Stat(altPath); err == nil {
						extCopy.Path = altPath
					}
				}
			}
		}

		// Add to registry
		m.RegisterExtension(&extCopy)
	}

	return nil
}

// ResolvePath attempts to find the absolute path for a plugin
// This checks multiple locations to account for different runtime environments
func ResolvePath(basePath, pluginPath string) string {
	// If already absolute, return as is
	if filepath.IsAbs(pluginPath) {
		return pluginPath
	}

	// Try relative to specified base path
	fullPath := filepath.Join(basePath, pluginPath)
	if _, err := os.Stat(fullPath); err == nil {
		return fullPath
	}

	// Try relative to executable
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)

		// Try executable dir + plugins
		pluginsPath := filepath.Join(execDir, "plugins", filepath.Base(pluginPath))
		if _, err := os.Stat(pluginsPath); err == nil {
			return pluginsPath
		}

		// Try direct path under executable dir
		directPath := filepath.Join(execDir, pluginPath)
		if _, err := os.Stat(directPath); err == nil {
			return directPath
		}
	}

	// Try current working directory
	cwd, err := os.Getwd()
	if err == nil {
		cwdPath := filepath.Join(cwd, pluginPath)
		if _, err := os.Stat(cwdPath); err == nil {
			return cwdPath
		}

		// Try plugins subdirectory
		cwdPluginsPath := filepath.Join(cwd, "plugins", filepath.Base(pluginPath))
		if _, err := os.Stat(cwdPluginsPath); err == nil {
			return cwdPluginsPath
		}
	}

	// If all fails, return the original with base path
	return fullPath
}

// SaveConfig writes the extensions configuration to a file
func SaveConfig(config *Config, path string) error {
	// If path is empty, use default
	if path == "" {
		// First try current working directory
		cwdPath := "configs/extensions.json"
		if _, err := os.Stat("configs"); err == nil || os.IsNotExist(err) {
			// Directory exists or can be created
			path = cwdPath
		} else {
			// Try executable directory as second option
			execPath, err := os.Executable()
			if err == nil {
				execDir := filepath.Dir(execPath)
				path = filepath.Join(execDir, "configs", "extensions.json")
			} else {
				// Fall back to current directory regardless
				path = cwdPath
			}
		}
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
