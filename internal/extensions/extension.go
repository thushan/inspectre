package extensions

import (
	"errors"
	"os/exec"
	"path/filepath"
	"plugin"
	"sync"
	"time"

	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/extensions/cli"
	"github.com/thushan/inspectre/internal/extensions/python"
)

// Extension type constants
const (
	TypeCLI    = "cli"
	TypePython = "python"
	TypeGolang = "golang"
)

// Common error definitions
var (
	ErrUnsupportedExtension = errors.New("unsupported extension type")
	ErrExtensionNotFound    = errors.New("extension not found")
	ErrInvalidExtension     = errors.New("invalid extension")

	// Extension cache TTL in seconds
	ExtensionCacheTTL = int64(300) // 5 minutes
)

// Type represents the extension type
type Type string

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

// CachedAnalyser holds an analyser with caching information
type CachedAnalyser struct {
	Analyser  analyser.Analyser
	CreatedAt int64 // Unix timestamp
}

// AnalyserFactory creates analysers of specific types
type AnalyserFactory interface {
	Create(ext *Extension) (analyser.Analyser, error)
}

// Manager handles loading and running extensions
type Manager struct {
	extensionsDir string
	extensions    map[string]*Extension
	factories     map[Type]AnalyserFactory
	analyserCache map[string]*CachedAnalyser
	mu            sync.RWMutex
	cacheMu       sync.RWMutex
}

// CLIAnalyserFactory creates CLI analysers
type CLIAnalyserFactory struct{}

func (f *CLIAnalyserFactory) Create(ext *Extension) (analyser.Analyser, error) {
	return cli.NewCLIAnalyser(ext.Name, ext.Path, ext.Config), nil
}

// PythonAnalyserFactory creates Python analysers
type PythonAnalyserFactory struct{}

func (f *PythonAnalyserFactory) Create(ext *Extension) (analyser.Analyser, error) {
	return python.NewPythonAnalyser(ext.Name, ext.Path, ext.Config), nil
}

// GolangAnalyserFactory creates Golang plugin analysers
type GolangAnalyserFactory struct {
	ExtensionsDir string
}

func (f *GolangAnalyserFactory) Create(ext *Extension) (analyser.Analyser, error) {
	// Resolve path
	path := ext.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.ExtensionsDir, path)
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
	analyser, ok := sym.(analyser.Analyser)
	if !ok {
		return nil, ErrInvalidExtension
	}

	return analyser, nil
}

// NewManager creates a new extension manager
func NewManager(extensionsDir string) *Manager {
	manager := &Manager{
		extensionsDir: extensionsDir,
		extensions:    make(map[string]*Extension),
		factories:     make(map[Type]AnalyserFactory),
		analyserCache: make(map[string]*CachedAnalyser),
	}

	// Register factories for different extension types
	manager.factories[TypeCLI] = &CLIAnalyserFactory{}
	manager.factories[TypePython] = &PythonAnalyserFactory{}
	manager.factories[TypeGolang] = &GolangAnalyserFactory{ExtensionsDir: extensionsDir}

	return manager
}

// LoadExtension loads an extension by name
func (m *Manager) LoadExtension(name string) (analyser.Analyser, error) {
	// Check cache first
	m.cacheMu.RLock()
	if cached, ok := m.analyserCache[name]; ok {
		if isCacheValid(cached.CreatedAt) {
			analyser := cached.Analyser
			m.cacheMu.RUnlock()
			return analyser, nil
		}
	}
	m.cacheMu.RUnlock()

	// Cache miss or expired, load from scratch
	m.mu.RLock()
	ext, exists := m.extensions[name]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrExtensionNotFound
	}

	if !ext.Enabled {
		return nil, errors.New("extension is disabled")
	}

	// Get factory for this extension type
	factory, ok := m.factories[ext.Type]
	if !ok {
		return nil, ErrUnsupportedExtension
	}

	// Create analyser
	analyser, err := factory.Create(ext)
	if err != nil {
		return nil, err
	}

	// Cache the analyser
	m.cacheMu.Lock()
	m.analyserCache[name] = &CachedAnalyser{
		Analyser:  analyser,
		CreatedAt: getCurrentTimestamp(),
	}
	m.cacheMu.Unlock()

	return analyser, nil
}

// LoadAllEnabled loads all enabled extensions
func (m *Manager) LoadAllEnabled() ([]analyser.Analyser, error) {
	var analysers []analyser.Analyser
	var wg sync.WaitGroup
	var mu sync.Mutex
	var loadErrors []error

	// First, gather all enabled extensions
	m.mu.RLock()
	enabledExtensions := make([]*Extension, 0)
	for _, ext := range m.extensions {
		if ext.Enabled {
			enabledExtensions = append(enabledExtensions, ext)
		}
	}
	m.mu.RUnlock()

	// Load extensions in parallel
	for _, ext := range enabledExtensions {
		wg.Add(1)
		go func(ext *Extension) {
			defer wg.Done()

			analyser, err := m.LoadExtension(ext.Name)
			if err != nil {
				mu.Lock()
				loadErrors = append(loadErrors, err)
				mu.Unlock()
				return
			}

			mu.Lock()
			analysers = append(analysers, analyser)
			mu.Unlock()
		}(ext)
	}

	wg.Wait()

	// Return what we have even if there were errors
	return analysers, nil
}

// RegisterExtension adds or updates an extension in the registry
func (m *Manager) RegisterExtension(ext *Extension) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.extensions[ext.Name] = ext

	// Clear cache for this extension
	m.cacheMu.Lock()
	delete(m.analyserCache, ext.Name)
	m.cacheMu.Unlock()
}

// GetExtension retrieves an extension by name
func (m *Manager) GetExtension(name string) (*Extension, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ext, exists := m.extensions[name]
	if !exists {
		return nil, ErrExtensionNotFound
	}
	return ext, nil
}

// ListExtensions returns all registered extensions
func (m *Manager) ListExtensions() []*Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Extension, 0, len(m.extensions))
	for _, ext := range m.extensions {
		result = append(result, ext)
	}
	return result
}

// ClearCache clears the analyser cache
func (m *Manager) ClearCache() {
	m.cacheMu.Lock()
	defer m.cacheMu.Unlock()

	m.analyserCache = make(map[string]*CachedAnalyser)
}

// CheckCommand verifies if a command is available in PATH
func CheckCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// isCacheValid checks if a cached entry is still valid
func isCacheValid(timestamp int64) bool {
	return (getCurrentTimestamp() - timestamp) < ExtensionCacheTTL
}

// getCurrentTimestamp returns the current Unix timestamp
func getCurrentTimestamp() int64 {
	return GetTimeNow().Unix()
}

// GetTimeNow is a function that can be mocked in tests
var GetTimeNow = func() TimeProvider {
	return RealTime{}
}

// TimeProvider is an interface for time operations
type TimeProvider interface {
	Unix() int64
}

// RealTime implements TimeProvider with actual time
type RealTime struct{}

func (RealTime) Unix() int64 {
	return time.Now().Unix()
}
