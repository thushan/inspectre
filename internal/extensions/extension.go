package extensions

import (
	"errors"
	"github.com/thushan/inspectre/internal/extensions/cli"
	"github.com/thushan/inspectre/internal/extensions/python"
	"os/exec"
	"path/filepath"
	"plugin"

	"github.com/thushan/inspectre/internal/core/analysis"
)

var (
	ErrUnsupportedExtension = errors.New("unsupported extension type")
	ErrExtensionNotFound    = errors.New("extension not found")
	ErrInvalidExtension     = errors.New("invalid extension")
)

// Type represents the extension type
type Type string

const (
	TypeCLI    Type = "cli"
	TypePython Type = "python"
	TypeGolang Type = "golang"
)

// Extension represents a loadable extension
type Extension struct {
	Name        string            `json:"name"`
	Type        Type              `json:"type"`
	Path        string            `json:"path"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version,omitempty"`
	Author      string            `json:"author,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	Enabled     bool              `json:"enabled"`
}

// Manager handles loading and running extensions
type Manager struct {
	extensionsDir string
	extensions    map[string]*Extension
}

// NewManager creates a new extension manager
func NewManager(extensionsDir string) *Manager {
	return &Manager{
		extensionsDir: extensionsDir,
		extensions:    make(map[string]*Extension),
	}
}

// LoadExtension loads an extension by name
func (m *Manager) LoadExtension(name string) (analysis.Analyser, error) {
	ext, exists := m.extensions[name]
	if !exists {
		return nil, ErrExtensionNotFound
	}

	if !ext.Enabled {
		return nil, errors.New("extension is disabled")
	}

	switch ext.Type {
	case TypeCLI:
		return cli.NewCLIAnalyser(ext.Name, ext.Path, ext.Config), nil
	case TypePython:
		return python.NewPythonAnalyser(ext.Name, ext.Path, ext.Config), nil
	case TypeGolang:
		return m.loadGolangPlugin(ext)
	default:
		return nil, ErrUnsupportedExtension
	}
}

// LoadAllEnabled loads all enabled extensions
func (m *Manager) LoadAllEnabled() ([]analysis.Analyser, error) {
	var analysers []analysis.Analyser

	for name, ext := range m.extensions {
		if ext.Enabled {
			analyser, err := m.LoadExtension(name)
			if err != nil {
				// Log the error but continue with other extensions
				continue
			}
			analysers = append(analysers, analyser)
		}
	}

	return analysers, nil
}

// RegisterExtension adds or updates an extension in the registry
func (m *Manager) RegisterExtension(ext *Extension) {
	m.extensions[ext.Name] = ext
}

// GetExtension retrieves an extension by name
func (m *Manager) GetExtension(name string) (*Extension, error) {
	ext, exists := m.extensions[name]
	if !exists {
		return nil, ErrExtensionNotFound
	}
	return ext, nil
}

// ListExtensions returns all registered extensions
func (m *Manager) ListExtensions() []*Extension {
	var result []*Extension
	for _, ext := range m.extensions {
		result = append(result, ext)
	}
	return result
}

// loadGolangPlugin loads a Go plugin
func (m *Manager) loadGolangPlugin(ext *Extension) (analysis.Analyser, error) {
	// Resolve path
	path := ext.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(m.extensionsDir, path)
	}

	// Load the plugin
	p, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}

	// Look up the analyser symbol
	sym, err := p.Lookup("Analyser")
	if err != nil {
		return nil, err
	}

	// Assert that the symbol is an Analyser
	analyser, ok := sym.(analysis.Analyser)
	if !ok {
		return nil, ErrInvalidExtension
	}

	return analyser, nil
}

// CheckCommand verifies if a command is available in PATH
func CheckCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
