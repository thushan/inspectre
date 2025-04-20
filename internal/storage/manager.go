package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
)

// Storage types
const (
	TypeDuckDB = "duckdb"
	TypeFile   = "file"
)

// Error definitions
var (
	ErrStorageNotInitialized = errors.New("storage not initialized")
	ErrInvalidStorageType    = errors.New("invalid storage type")
	ErrOperationTimeout      = errors.New("operation timed out")
)

// Default timeouts
const (
	DefaultQueryTimeout = 30 * time.Second
	DefaultStoreTimeout = 60 * time.Second
	DefaultCloseTimeout = 10 * time.Second
	DefaultInitTimeout  = 30 * time.Second
)

// Manager handles storage operations
type Manager struct {
	storage     Storage
	basePath    string
	mu          sync.RWMutex
	logger      *logging.Logger
	ctx         context.Context
	cancelFn    context.CancelFunc
	storageType string
}

// NewManager creates a new storage manager
func NewManager(storageType, basePath string) (*Manager, error) {
	if storageType == "" {
		storageType = TypeDuckDB // Default to DuckDB
	}

	// Create context for manager operations
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		basePath:    basePath,
		ctx:         ctx,
		cancelFn:    cancel,
		logger:      logging.GetLogger(),
		storageType: storageType,
	}

	// Ensure the base directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Initialize the storage backend
	if err := m.initializeStorage(storageType); err != nil {
		return nil, err
	}

	return m, nil
}

// initializeStorage sets up the storage backend with timeout
func (m *Manager) initializeStorage(storageType string) error {
	// Create a context with timeout for initialization
	ctx, cancel := context.WithTimeout(m.ctx, DefaultInitTimeout)
	defer cancel()

	// Create storage based on type
	var storage Storage
	var err error

	switch storageType {
	case TypeDuckDB:
		storage = NewDuckDBStorage(m.basePath)
	case TypeFile:
		storage = NewFileStorage(m.basePath)
	default:
		return fmt.Errorf("%w: %s", ErrInvalidStorageType, storageType)
	}

	// Initialize with context if supported
	if withCtx, ok := storage.(WithContext); ok {
		storage = withCtx.WithContext(ctx)
	}

	// Initialize the storage
	initDone := make(chan error, 1)
	go func() {
		initDone <- storage.Initialize()
	}()

	// Wait for initialization or timeout
	select {
	case err = <-initDone:
		if err != nil {
			return fmt.Errorf("failed to initialize %s storage: %w", storageType, err)
		}
	case <-ctx.Done():
		return fmt.Errorf("%w: storage initialization", ErrOperationTimeout)
	}

	m.mu.Lock()
	m.storage = storage
	m.storageType = storageType
	m.mu.Unlock()

	m.logger.Info("Initialized %s storage at %s", storageType, m.basePath)
	return nil
}

// SetStorageType changes the storage backend with proper cleanup
func (m *Manager) SetStorageType(storageType string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Skip if already using this storage type
	if m.storageType == storageType && m.storage != nil {
		return nil
	}

	// Close existing storage if it exists
	if m.storage != nil {
		// Create context with timeout for closing
		ctx, cancel := context.WithTimeout(m.ctx, DefaultCloseTimeout)
		defer cancel()

		closeDone := make(chan error, 1)
		go func() {
			closeDone <- m.storage.Close()
		}()

		// Wait for close or timeout
		select {
		case err := <-closeDone:
			if err != nil {
				m.logger.Warning("Error closing storage: %v", err)
			}
		case <-ctx.Done():
			m.logger.Warning("Timeout while closing storage")
		}

		m.storage = nil
	}

	return m.initializeStorage(storageType)
}

// StoreResults saves analyser results with timeout
func (m *Manager) StoreResults(taskID, repository string, results []*analyser.Result) error {
	m.mu.RLock()
	storage := m.storage
	m.mu.RUnlock()

	if storage == nil {
		return ErrStorageNotInitialized
	}

	// Create context with timeout for store operation
	ctx, cancel := context.WithTimeout(m.ctx, DefaultStoreTimeout)
	defer cancel()

	// Use context-aware storage if available
	if withCtx, ok := storage.(WithContext); ok {
		storage = withCtx.WithContext(ctx)
	}

	// Execute store operation with timeout
	storeDone := make(chan error, 1)
	go func() {
		storeDone <- storage.StoreResults(taskID, repository, results)
	}()

	// Wait for completion or timeout
	select {
	case err := <-storeDone:
		return err
	case <-ctx.Done():
		return fmt.Errorf("%w: store results", ErrOperationTimeout)
	}
}

// QueryMetrics executes a query against the storage with timeout
func (m *Manager) QueryMetrics(query string) ([]map[string]interface{}, error) {
	m.mu.RLock()
	storage := m.storage
	m.mu.RUnlock()

	if storage == nil {
		return nil, ErrStorageNotInitialized
	}

	// Create context with timeout for query operation
	ctx, cancel := context.WithTimeout(m.ctx, DefaultQueryTimeout)
	defer cancel()

	// Use context-aware storage if available
	if withCtx, ok := storage.(WithContext); ok {
		storage = withCtx.WithContext(ctx)
	}

	// Execute query with timeout
	resultCh := make(chan struct {
		results []map[string]interface{}
		err     error
	}, 1)

	go func() {
		results, err := storage.QueryMetrics(query)
		resultCh <- struct {
			results []map[string]interface{}
			err     error
		}{results, err}
	}()

	// Wait for completion or timeout
	select {
	case result := <-resultCh:
		return result.results, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: query metrics", ErrOperationTimeout)
	}
}

// GetMetricsByName retrieves metrics by name with timeout
func (m *Manager) GetMetricsByName(name string, limit int) ([]map[string]interface{}, error) {
	m.mu.RLock()
	storage := m.storage
	m.mu.RUnlock()

	if storage == nil {
		return nil, ErrStorageNotInitialized
	}

	// Create context with timeout for query operation
	ctx, cancel := context.WithTimeout(m.ctx, DefaultQueryTimeout)
	defer cancel()

	// Use context-aware storage if available
	if withCtx, ok := storage.(WithContext); ok {
		storage = withCtx.WithContext(ctx)
	}

	// Execute query with timeout
	resultCh := make(chan struct {
		results []map[string]interface{}
		err     error
	}, 1)

	go func() {
		results, err := storage.GetMetricsByName(name, limit)
		resultCh <- struct {
			results []map[string]interface{}
			err     error
		}{results, err}
	}()

	// Wait for completion or timeout
	select {
	case result := <-resultCh:
		return result.results, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: get metrics by name", ErrOperationTimeout)
	}
}

// GetMetricsByTask retrieves metrics for a specific task with timeout
func (m *Manager) GetMetricsByTask(taskID string) ([]map[string]interface{}, error) {
	m.mu.RLock()
	storage := m.storage
	m.mu.RUnlock()

	if storage == nil {
		return nil, ErrStorageNotInitialized
	}

	// Create context with timeout for query operation
	ctx, cancel := context.WithTimeout(m.ctx, DefaultQueryTimeout)
	defer cancel()

	// Use context-aware storage if available
	if withCtx, ok := storage.(WithContext); ok {
		storage = withCtx.WithContext(ctx)
	}

	// Execute query with timeout
	resultCh := make(chan struct {
		results []map[string]interface{}
		err     error
	}, 1)

	go func() {
		results, err := storage.GetMetricsByTask(taskID)
		resultCh <- struct {
			results []map[string]interface{}
			err     error
		}{results, err}
	}()

	// Wait for completion or timeout
	select {
	case result := <-resultCh:
		return result.results, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: get metrics by task", ErrOperationTimeout)
	}
}

// GetRepositorySummary gets summary data for a repository with timeout
func (m *Manager) GetRepositorySummary(repository string) (map[string]interface{}, error) {
	m.mu.RLock()
	storage := m.storage
	m.mu.RUnlock()

	if storage == nil {
		return nil, ErrStorageNotInitialized
	}

	// Create context with timeout for query operation
	ctx, cancel := context.WithTimeout(m.ctx, DefaultQueryTimeout)
	defer cancel()

	// Use context-aware storage if available
	if withCtx, ok := storage.(WithContext); ok {
		storage = withCtx.WithContext(ctx)
	}

	// Execute query with timeout
	resultCh := make(chan struct {
		result map[string]interface{}
		err    error
	}, 1)

	go func() {
		result, err := storage.GetRepositorySummary(repository)
		resultCh <- struct {
			result map[string]interface{}
			err    error
		}{result, err}
	}()

	// Wait for completion or timeout
	select {
	case result := <-resultCh:
		return result.result, result.err
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: get repository summary", ErrOperationTimeout)
	}
}

// GetStorageFilePath returns the path to a storage file
func (m *Manager) GetStorageFilePath(filename string) string {
	return filepath.Join(m.basePath, filename)
}

// GetCurrentStorageType returns the current storage type
func (m *Manager) GetCurrentStorageType() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.storageType
}

// Close safely shuts down the storage manager
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Cancel the manager context
	m.cancelFn()

	if m.storage == nil {
		return nil
	}

	// Create context with timeout for close operation
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCloseTimeout)
	defer cancel()

	// Close storage with timeout
	closeDone := make(chan error, 1)
	go func() {
		closeDone <- m.storage.Close()
	}()

	// Wait for completion or timeout
	select {
	case err := <-closeDone:
		m.storage = nil
		return err
	case <-ctx.Done():
		m.storage = nil
		return fmt.Errorf("%w: close storage", ErrOperationTimeout)
	}
}
