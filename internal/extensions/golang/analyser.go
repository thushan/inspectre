package golang

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"

	"github.com/thushan/inspectre/internal/core/analysis"
)

// PluginAnalyser wraps a Go plugin implementing the Analyser interface
type PluginAnalyser struct {
	name     string
	path     string
	analyser analysis.Analyser
	config   map[string]string
}

// NewPluginAnalyser creates a new Go plugin analyser
func NewPluginAnalyser(name, path string, config map[string]string) (*PluginAnalyser, error) {
	if config == nil {
		config = make(map[string]string)
	}

	analyser := &PluginAnalyser{
		name:   name,
		path:   path,
		config: config,
	}

	if err := analyser.loadPlugin(); err != nil {
		return nil, err
	}

	return analyser, nil
}

// Name returns the analyser's identifier
func (a *PluginAnalyser) Name() string {
	return a.name
}

// Initialize prepares the analyser
func (a *PluginAnalyser) Initialize(repoPath string, env map[string]string) error {
	// Add config to env
	for k, v := range a.config {
		env[fmt.Sprintf("INSPECTRE_CONFIG_%s", k)] = v
	}

	return a.analyser.Initialize(repoPath, env)
}

// Run executes the analyser plugin
func (a *PluginAnalyser) Run() ([]analysis.Metric, error) {
	return a.analyser.Run()
}

// Cleanup performs any necessary cleanup
func (a *PluginAnalyser) Cleanup() error {
	return a.analyser.Cleanup()
}

// loadPlugin loads the Go plugin
func (a *PluginAnalyser) loadPlugin() error {
	// Check if file exists
	if _, err := os.Stat(a.path); os.IsNotExist(err) {
		return fmt.Errorf("plugin file not found: %s", a.path)
	}

	// Use absolute path for plugin
	absPath, err := filepath.Abs(a.path)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Load the plugin
	plug, err := plugin.Open(absPath)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	// Look up the analyser symbol
	sym, err := plug.Lookup("Analyser")
	if err != nil {
		return fmt.Errorf("plugin does not export 'Analyser': %w", err)
	}

	// Assert that the symbol is an Analyser
	analyser, ok := sym.(analysis.Analyser)
	if !ok {
		return fmt.Errorf("exported 'Analyser' is not an Analyser interface")
	}

	a.analyser = analyser
	return nil
}
