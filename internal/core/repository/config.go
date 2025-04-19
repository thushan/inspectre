package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
)

// loadConfig reads and parses the repositories configuration file
func (m *Manager) loadConfig() error {
	file, err := os.Open(m.configPath)
	if os.IsNotExist(err) {
		m.logger.Warning("Config file not found at %s, creating empty configuration", m.configPath)

		// Create an empty config
		m.config = &Config{
			Version:      1,
			Repositories: []Repository{},
		}

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(m.configPath), DirectoryPerm); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Save empty config
		return m.SaveConfig()
	} else if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Process environment variables in the config
	processedData := m.processEnvVars(string(data))

	var config Config
	if err := json.Unmarshal([]byte(processedData), &config); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}

	m.config = &config
	return nil
}

// processEnvVars replaces environment variables in the format ${VAR_NAME}
func (m *Manager) processEnvVars(input string) string {
	re := regexp.MustCompile(`\${([^}]+)}`)
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := match[2 : len(match)-1]
		// Get environment variable value
		value := os.Getenv(varName)
		if value == "" {
			// If not found, keep the original placeholder
			return match
		}
		return value
	})
	return result
}

// SaveConfig saves the current configuration to disk
func (m *Manager) SaveConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return ErrManagerClosed
	}
	m.closedMu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), DirectoryPerm); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, PublicReadFilePerm); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
