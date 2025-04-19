package extensions

import (
	"context"
	"errors"
	"fmt"
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
	ErrLoadTimeout          = errors.New("extension load timeout")
	ErrContextCancelled     = errors.New("operation cancelled")

	// Extension cache TTL in seconds
	ExtensionCacheTTL = int64(300) // 5 minutes

	// Timeouts
	ExtensionLoadTimeout = 30 * time.Second
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
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
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
	// Create context for extension operations
	ctx, cancel := context.WithCancel(context.Background())

	manager := &Manager{
		extensionsDir: extensionsDir,
		extensions:    make(map[string]*Extension),
		factories:     make(map[Type]AnalyserFactory),
		analyserCache: make(map[string]*CachedAnalyser),
		ctx:           ctx,
		cancel:        cancel,
	}

	// Register factories for different extension types
	manager.factories[TypeCLI] = &CLIAnalyserFactory{}
	manager.factories[TypePython] = &PythonAnalyserFactory{}
	manager.factories[TypeGolang] = &GolangAnalyserFactory{ExtensionsDir: extensionsDir}

	return manager
}

// Shutdown performs cleanup operations for the extension manager
func (m *Manager) Shutdown() error {
	// Signal cancellation to all operations
	m.cancel()

	// Wait for operations to complete with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All operations completed
	case <-time.After(5 * time.Second):
		return errors.New("timeout waiting for extension operations to complete")
	}

	// Clear cache
	m.cacheMu.Lock()
	m.analyserCache = nil
	m.cacheMu.Unlock()

	return nil
}

// LoadExtension loads an extension by name
func (m *Manager) LoadExtension(name string) (analyser.Analyser, error) {
	// Create timeout context for the operation
	ctx, cancel := context.WithTimeout(m.ctx, ExtensionLoadTimeout)
	defer cancel()

	// Check cache first under read lock
	m.cacheMu.RLock()
	if cached, ok := m.analyserCache[name]; ok && isCacheValid(cached.CreatedAt) {
		analyser := cached.Analyser
		m.cacheMu.RUnlock()
		return analyser, nil
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

	// Create analyser with timeout
	var analyser analyser.Analyser
	var err error

	done := make(chan struct{})
	go func() {
		defer close(done)
		analyser, err = factory.Create(ext)
	}()

	select {
	case <-done:
		// Creation completed
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrLoadTimeout, ctx.Err())
	}

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
	// Create context for the entire operation
	ctx, cancel := context.WithTimeout(m.ctx, ExtensionLoadTimeout*2)
	defer cancel()

	var analysers []analyser.Analyser
	var loadErrors []error

	// Use a mutex to protect the results slice during concurrent access
	var resultMu sync.Mutex

	// First gather all enabled extensions under a read lock
	m.mu.RLock()
	enabledExtensions := make([]*Extension, 0)
	for _, ext := range m.extensions {
		if ext.Enabled {
			enabledExtensions = append(enabledExtensions, ext)
		}
	}
	m.mu.RUnlock()

	if len(enabledExtensions) == 0 {
		return []analyser.Analyser{}, nil
	}

	// Create a channel for results with buffer equal to number of extensions
	type loadResult struct {
		analyser analyser.Analyser
		err      error
		name     string
	}
	resultCh := make(chan loadResult, len(enabledExtensions))

	// Load extensions in parallel with proper resource tracking
	for _, ext := range enabledExtensions {
		ext := ext // Capture for goroutine
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()

			// Create a separate context for each extension load
			extCtx, extCancel := context.WithTimeout(ctx, ExtensionLoadTimeout)
			defer extCancel()

			// Track the time taken for metrics
			startTime := time.Now()

			// Load the extension with the context
			a, err := m.loadExtensionWithContext(extCtx, ext.Name)

			// Send result through channel with context awareness
			select {
			case resultCh <- loadResult{
				analyser: a,
				err:      err,
				name:     ext.Name,
			}:
				// Result sent
			case <-ctx.Done():
				// Parent context cancelled, discard result
			}

			loadTime := time.Since(startTime)
			if loadTime > 1*time.Second {
				// Log slow extension loads
				fmt.Printf("Warning: Extension %s took %v to load\n", ext.Name, loadTime)
			}
		}()
	}

	// Collect results with timeout
	for i := 0; i < len(enabledExtensions); i++ {
		select {
		case result := <-resultCh:
			if result.err != nil {
				loadErrors = append(loadErrors, fmt.Errorf("failed to load %s: %w", result.name, result.err))
			} else {
				resultMu.Lock()
				analysers = append(analysers, result.analyser)
				resultMu.Unlock()
			}
		case <-ctx.Done():
			// Overall timeout or cancellation
			return analysers, fmt.Errorf("%w: %v", ErrContextCancelled, ctx.Err())
		}
	}

	// Return what we have even if some failed
	var err error
	if len(loadErrors) > 0 {
		// Combine errors for reporting
		errMsg := fmt.Sprintf("failed to load %d extensions", len(loadErrors))
		if len(loadErrors) > 0 {
			errMsg += fmt.Sprintf(": %v", loadErrors[0])
		}
		err = errors.New(errMsg)
	}

	return analysers, err
}

// loadExtensionWithContext is a helper that respects context cancellation
func (m *Manager) loadExtensionWithContext(ctx context.Context, name string) (analyser.Analyser, error) {
	// Check if the context is already cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// Continue with loading
	}

	// Run load with cancellation support
	resultCh := make(chan struct {
		a   analyser.Analyser
		err error
	}, 1)

	go func() {
		a, err := m.LoadExtension(name)
		select {
		case resultCh <- struct {
			a   analyser.Analyser
			err error
		}{a, err}:
		case <-ctx.Done():
			// Context cancelled, discard result
		}
	}()

	select {
	case result := <-resultCh:
		return result.a, result.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
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
