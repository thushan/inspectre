package repository

import (
	"context"
	"errors"
	"fmt"
	"github.com/sony/sonyflake"
	"os"
	"path/filepath"
	"strconv"
	"time"

	appctx "github.com/thushan/inspectre/internal/core/context"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/core/utils"
)

// Repository manager constants
const (
	// Permissions
	PublicReadFilePerm = 0o644
	DirectoryPerm      = 0o755

	// Clone operation settings
	CloneTimeout      = 10 * time.Minute
	CleanupRetryCount = 3
	CloneProgressSize = 10

	// Cache settings
	RepoCacheTTL     = 1 * time.Hour
	MetadataCacheTTL = 30 * time.Minute

	// URI prefixes
	HTTPPrefix  = "http://"
	HTTPSPrefix = "https://"
	SSHPrefix   = "git@"

	// URL validation
	MinURLLength = 5
)

// Default paths
var (
	// Default task directory paths
	DefaultTempDir = filepath.Join(os.TempDir(), "inspectre")

	// Generates unique IDs for tasks
	SonyFlake = sonyflake.NewSonyflake(sonyflake.Settings{})

	// Error definitions
	ErrRepositoryNotFound    = errors.New("repository not found")
	ErrInvalidConfiguration  = errors.New("invalid configuration")
	ErrCloneFailure          = errors.New("failed to clone repository")
	ErrUnsupportedRepoType   = errors.New("unsupported repository type")
	ErrMissingAuthentication = errors.New("missing authentication details")
	ErrManagerClosed         = errors.New("repository manager is closed")
)

// NewManager creates a new repository manager
func NewManager(configPath string, appCtx *appctx.AppContext) (*Manager, error) {
	if configPath == "" {
		// Default config path
		configPath = "configs/repositories.json"
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		configPath: configPath,
		ctx:        ctx,
		cancelFunc: cancel,
		logger:     logging.GetLogger(),
		tempDir:    DefaultTempDir,
		closed:     false,
		repoCache:  make(map[string]*RepoCache),
	}

	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	// Register shutdown hook
	if appCtx != nil {
		appCtx.AddShutdownHookWithPriority("repomanager.shutdown",
			appctx.PriorityNormal, m.shutdown)
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(m.tempDir, DirectoryPerm); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	return m, nil
}

// SetDisplay sets the display manager
func (m *Manager) SetDisplay(display types.DisplayProvider) {
	m.display = display
}

// SetTempDir sets the temporary directory for repositories
func (m *Manager) SetTempDir(dir string) {
	m.tempDir = dir
}

// CleanUp removes the temporary directory
func (m *Manager) CleanUp(task *Task) error {
	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return ErrManagerClosed
	}
	m.closedMu.RUnlock()

	if task == nil {
		return errors.New("task cannot be nil")
	}

	m.logger.Info("Cleaning up task directory: %s", task.BaseDir)

	// Try to clean up with retries (but without sleeps - use context pattern)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return utils.RetryWithContext(ctx, CleanupRetryCount, func() error {
		return os.RemoveAll(task.BaseDir)
	})
}

// shutdown performs a graceful shutdown
func (m *Manager) shutdown(ctx context.Context) error {
	var err error
	m.shutdownOnce.Do(func() {
		m.logger.Info("Shutting down repository manager...")

		// Mark as closed first to prevent new operations
		m.closedMu.Lock()
		m.closed = true
		m.closedMu.Unlock()

		// Cancel context to signal shutdown to ongoing operations
		m.cancelFunc()

		// Set up a timeout for cleanup operations
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 5*time.Second)
		defer cleanupCancel()

		// Wait for all goroutines to complete or timeout
		waitCh := make(chan struct{})
		go func() {
			m.wg.Wait()
			close(waitCh)
		}()

		select {
		case <-waitCh:
			// All goroutines exited cleanly
			m.logger.Info("Repository manager shutdown completed successfully")
		case <-cleanupCtx.Done():
			// Timeout - some goroutines didn't exit
			err = fmt.Errorf("repository manager shutdown timed out: %w", cleanupCtx.Err())
			m.logger.Warning("Repository manager shutdown timed out, some operations may not have completed")
		}

		// Clear caches to help with garbage collection
		m.repoCacheMu.Lock()
		m.repoCache = nil
		m.repoCacheMu.Unlock()
	})

	return err
}

// CreateTask creates a new analyser task
func (m *Manager) CreateTask(nameOrURL string) (*Task, error) {
	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return nil, ErrManagerClosed
	}
	m.closedMu.RUnlock()

	repo, err := m.GetRepository(nameOrURL)
	if err != nil {
		// Handle case when URL is provided directly
		if utils.IsURLString(nameOrURL) {
			// Create a temporary repository entry
			repo = &Repository{
				Name: utils.ExtractRepoName(nameOrURL),
				URL:  nameOrURL,
				Type: utils.GuessRepoType(nameOrURL),
				Auth: Auth{}, // No auth provided
			}
		} else {
			return nil, err
		}
	}

	// Generate task ID with retry mechanism
	var rawId uint64
	var taskID string

	// Retry a few times if ID generation fails
	for attempts := 0; attempts < 3; attempts++ {
		var err error
		rawId, err = SonyFlake.NextID()
		if err == nil {
			taskID = strconv.FormatUint(rawId, 36)
			break
		}

		if attempts == 2 {
			return nil, fmt.Errorf("failed to generate task ID: %w", err)
		}

		// Brief delay before retry
		time.Sleep(10 * time.Millisecond)
	}

	// Create base task directory
	baseDir := filepath.Join(m.tempDir, taskID)

	// Create separate subdirectories
	repoDir := filepath.Join(baseDir, "repo")     // Repository clone directory
	assetsDir := filepath.Join(baseDir, "assets") // Assets and plugin data
	logFile := filepath.Join(baseDir, "task.log") // Main log file

	m.logger.Info("Creating task %s for repository %s", taskID, repo.URL)

	return &Task{
		ID:         taskID,
		Repository: repo.URL,
		Status:     "Created",
		StartTime:  timeNow(),
		BaseDir:    baseDir,
		RepoDir:    repoDir,
		AssetsDir:  assetsDir,
		LogFile:    logFile,
	}, nil
}
