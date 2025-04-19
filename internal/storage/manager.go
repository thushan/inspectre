package storage

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/thushan/inspectre/internal/core/analyser"
)

var (
	ErrStorageNotInitialized = errors.New("storage not initialized")
	ErrInvalidStorageType    = errors.New("invalid storage type")
)

// Manager handles storage operations
type Manager struct {
	storage  Storage
	basePath string
	mu       sync.RWMutex
}

// NewManager creates a new storage manager
func NewManager(storageType, basePath string) (*Manager, error) {
	m := &Manager{
		basePath: basePath,
	}

	if err := m.SetStorageType(storageType); err != nil {
		return nil, err
	}

	return m, nil
}

// SetStorageType changes the storage backend
func (m *Manager) SetStorageType(storageType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close existing storage if it exists
	if m.storage != nil {
		if err := m.storage.Close(); err != nil {
			return fmt.Errorf("failed to close existing storage: %w", err)
		}
	}

	// Create new storage
	switch storageType {
	case "duckdb":
		m.storage = NewDuckDBStorage(m.basePath)
	case "file":
		m.storage = NewFileStorage(m.basePath)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStorageType, storageType)
	}

	// Initialize the storage
	if err := m.storage.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	return nil
}

// StoreResults saves analyser results
func (m *Manager) StoreResults(taskID, repository string, results []*analyser.Result) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.storage == nil {
		return ErrStorageNotInitialized
	}

	return m.storage.StoreResults(taskID, repository, results)
}

// QueryMetrics executes a query against the storage
func (m *Manager) QueryMetrics(query string) ([]map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.storage == nil {
		return nil, ErrStorageNotInitialized
	}

	return m.storage.QueryMetrics(query)
}

// GetStorageFilePath returns the path to a storage file
func (m *Manager) GetStorageFilePath(filename string) string {
	return filepath.Join(m.basePath, filename)
}

// Close closes the storage
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.storage == nil {
		return nil
	}

	return m.storage.Close()
}
