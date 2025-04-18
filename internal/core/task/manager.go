package task

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/thushan/inspectre/internal/core/analysis"
	appctx "github.com/thushan/inspectre/internal/core/context"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/repository"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/extensions"
	"github.com/thushan/inspectre/internal/storage"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskAlreadyRunning = errors.New("task already running")
	ErrInvalidTaskState   = errors.New("invalid task state")
	ErrTaskCancelled      = errors.New("task was cancelled")
)

const (
	StatusCreated   = "Created"
	StatusRunning   = "Running"
	StatusCompleted = "Completed"
	StatusFailed    = "Failed"
	StatusCancelled = "Cancelled"
)

// Manager handles analysis tasks
type Manager struct {
	repoManager      *repository.Manager
	storageManager   *storage.Manager
	extensionManager *extensions.Manager
	tasks            map[string]*types.Task
	tasksMu          sync.RWMutex
	ctx              *appctx.AppContext
	logger           *logging.Logger
	display          types.DisplayProvider
	taskCancels      map[string]context.CancelFunc
	taskCancelsMu    sync.Mutex
}

// NewManager creates a new task manager
func NewManager(repoManager *repository.Manager, storageManager *storage.Manager, extensionManager *extensions.Manager, appCtx *appctx.AppContext) *Manager {
	manager := &Manager{
		repoManager:      repoManager,
		storageManager:   storageManager,
		extensionManager: extensionManager,
		tasks:            make(map[string]*types.Task),
		ctx:              appCtx,
		logger:           logging.GetLogger(),
		taskCancels:      make(map[string]context.CancelFunc),
	}

	// Register shutdown hook
	if appCtx != nil {
		appCtx.AddShutdownHook(manager.Shutdown)
	}

	return manager
}

// SetDisplay sets the display manager
func (m *Manager) SetDisplay(display types.DisplayProvider) {
	m.display = display
}

// CreateTask creates a new analysis task
func (m *Manager) CreateTask(nameOrURL string) (*types.Task, error) {
	task, err := m.repoManager.CreateTask(nameOrURL)
	if err != nil {
		return nil, err
	}

	m.tasksMu.Lock()
	m.tasks[task.ID] = task
	m.tasksMu.Unlock()

	return task, nil
}

// StartTask begins a task's execution
func (m *Manager) StartTask(taskID string) error {
	m.tasksMu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.tasksMu.Unlock()
		return ErrTaskNotFound
	}

	if task.Status != StatusCreated {
		m.tasksMu.Unlock()
		return ErrInvalidTaskState
	}

	task.Status = StatusRunning
	m.tasksMu.Unlock()

	// Create required directories
	if err := os.MkdirAll(task.BaseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory: %w", err)
	}

	if err := os.MkdirAll(task.RepoDir, 0755); err != nil {
		return fmt.Errorf("failed to create repository directory: %w", err)
	}

	if err := os.MkdirAll(task.AssetsDir, 0755); err != nil {
		return fmt.Errorf("failed to create assets directory: %w", err)
	}

	// Register task log file
	if err := m.logger.RegisterTaskLog(task.ID, task.LogFile); err != nil {
		return fmt.Errorf("failed to register task log: %w", err)
	}

	// Create a context for this task
	taskCtx, taskCancel := context.WithCancel(context.Background())

	m.taskCancelsMu.Lock()
	m.taskCancels[task.ID] = taskCancel
	m.taskCancelsMu.Unlock()

	// Increment wait group for graceful shutdown
	if m.ctx != nil {
		m.ctx.IncrementWaitGroup()
	}

	go func() {
		m.RunTask(taskCtx, task)

		// Clean up after task completes
		m.taskCancelsMu.Lock()
		delete(m.taskCancels, task.ID)
		m.taskCancelsMu.Unlock()

		// Decrement wait group for graceful shutdown
		if m.ctx != nil {
			m.ctx.DecrementWaitGroup()
		}
	}()

	return nil
}

// CancelTask cancels a running task
func (m *Manager) CancelTask(taskID string) error {
	m.tasksMu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.tasksMu.Unlock()
		return ErrTaskNotFound
	}

	if task.Status != StatusRunning {
		m.tasksMu.Unlock()
		return ErrInvalidTaskState
	}

	task.Status = StatusCancelled
	m.tasksMu.Unlock()

	// Call the cancel function
	m.taskCancelsMu.Lock()
	if cancel, exists := m.taskCancels[taskID]; exists {
		cancel()
	}
	m.taskCancelsMu.Unlock()

	m.logger.TaskInfo(taskID, "Task cancelled by user")

	return nil
}

// GetTask retrieves a task by ID
func (m *Manager) GetTask(taskID string) (*types.Task, error) {
	m.tasksMu.RLock()
	defer m.tasksMu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}

	// Create a copy of the task to avoid concurrent modification issues
	taskCopy := *task
	return &taskCopy, nil
}

// ListTasks returns all tasks
func (m *Manager) ListTasks(showAll, showFailed bool) []*types.Task {
	m.tasksMu.RLock()
	defer m.tasksMu.RUnlock()

	var result []*types.Task

	for _, task := range m.tasks {
		if showAll {
			result = append(result, task)
		} else if showFailed && task.Status == StatusFailed {
			result = append(result, task)
		} else if task.Status == StatusRunning || task.Status == StatusCreated {
			result = append(result, task)
		}
	}

	return result
}

// GetLogReader returns a reader for the task's log
func (m *Manager) GetLogReader(taskID string) (io.ReadCloser, error) {
	m.tasksMu.RLock()
	task, exists := m.tasks[taskID]
	m.tasksMu.RUnlock()

	if !exists {
		return nil, ErrTaskNotFound
	}

	return m.logger.GetTaskLogReader(taskID, task.LogFile)
}

// CleanupTask removes all files associated with a task
func (m *Manager) CleanupTask(taskID string) error {
	m.tasksMu.RLock()
	task, exists := m.tasks[taskID]
	m.tasksMu.RUnlock()

	if !exists {
		return ErrTaskNotFound
	}

	// Make sure any log files are closed first
	m.logger.CloseTaskLog(taskID)

	// Use repo manager to clean up
	return m.repoManager.CleanUp(task)
}

// Shutdown performs a graceful shutdown
func (m *Manager) Shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down task manager...")

	// Cancel all running tasks
	m.taskCancelsMu.Lock()
	for taskID, cancel := range m.taskCancels {
		m.logger.Info("Cancelling task: %s", taskID)
		cancel()
	}
	m.taskCancelsMu.Unlock()

	// Wait for context to be done or timeout
	<-ctx.Done()

	return nil
}

// RunTask performs the actual repository analysis
func (m *Manager) RunTask(ctx context.Context, task *types.Task) {
	// Initialize spinner if display is available
	var spinner types.SpinnerProvider
	if m.display != nil {
		spinner = m.display.StartSpinner(fmt.Sprintf("Analysing repository %s", task.Repository))
		defer spinner.Success(fmt.Sprintf("Analysis of %s completed", task.Repository))
	}

	// Log task start
	m.logger.TaskInfo(task.ID, "Task started at %s", time.Now().Format(time.RFC3339))

	// Get repository details
	repo, err := m.repoManager.GetRepository(task.Repository)
	if err != nil {
		// Handle URLs that aren't in the config
		if !isURL(task.Repository) {
			m.markTaskFailed(task, fmt.Sprintf("Repository not found: %v", err))
			return
		}

		// Create temporary repository object for direct URLs
		repo = &types.Repository{
			URL:  task.Repository,
			Type: repository.GuessRepoType(task.Repository),
			Auth: types.Auth{}, // Empty Auth struct
		}
	}

	// Check if task is cancelled
	select {
	case <-ctx.Done():
		m.markTaskCancelled(task, "Task context cancelled")
		return
	default:
	}

	// Clone the repository to the repo directory
	m.logger.TaskInfo(task.ID, "Cloning repository %s to %s", repo.URL, task.RepoDir)

	if spinner != nil {
		spinner.UpdateText(fmt.Sprintf("Cloning repository %s", repo.URL))
	}

	err = m.repoManager.Clone(repo, task.RepoDir)
	if err != nil {
		m.markTaskFailed(task, fmt.Sprintf("Failed to clone repository: %v", err))
		return
	}

	// Check if task is cancelled
	select {
	case <-ctx.Done():
		m.markTaskCancelled(task, "Cancelled during repository cloning")
		return
	default:
	}

	// Run analysis
	m.logger.TaskInfo(task.ID, "Repository cloned successfully. Beginning analysis...")

	if spinner != nil {
		spinner.UpdateText("Loading analysers...")
	}

	// Create analyser list starting with built-in analysers
	analysers := []analysis.Analyser{
		analysis.NewFileAnalyser(),
		analysis.NewGitAnalyser(),
	}

	// Add extension analysers if extension manager is available
	if m.extensionManager != nil {
		m.logger.TaskInfo(task.ID, "Loading extensions...")
		extAnalysers, err := m.extensionManager.LoadAllEnabled()
		if err != nil {
			m.logger.TaskWarning(task.ID, "Warning: failed to load some extensions: %v", err)
		}

		if len(extAnalysers) > 0 {
			analysers = append(analysers, extAnalysers...)
			m.logger.TaskInfo(task.ID, "Loaded %d extension analysers", len(extAnalysers))
		}
	}

	// Create logging function for analyser manager
	loggerFn := func(format string, args ...interface{}) {
		m.logger.TaskInfo(task.ID, format, args...)
	}

	analyserManager := analysis.NewManager(analysers, loggerFn)

	// Prepare environment variables for analysers
	env := map[string]string{
		"REPOSITORY_NAME": repo.Name,
		"REPOSITORY_URL":  repo.URL,
		"REPOSITORY_TYPE": repo.Type,
		"TASK_ID":         task.ID,
		"ASSETS_DIR":      task.AssetsDir,
	}

	// Check if task is cancelled
	select {
	case <-ctx.Done():
		m.markTaskCancelled(task, "Cancelled during analysis preparation")
		return
	default:
	}

	m.logger.TaskInfo(task.ID, "Running analysers on repository...")

	if spinner != nil {
		spinner.UpdateText("Running analysers...")
	}

	// Use repo directory instead of work directory
	results, err := analyserManager.AnalyseRepository(task.RepoDir, env)
	if err != nil {
		m.markTaskFailed(task, fmt.Sprintf("Analysis failed: %v", err))
		return
	}

	m.logger.TaskInfo(task.ID, "Analysis completed with %d result sets", len(results))

	// Format and log results
	for _, result := range results {
		m.logger.TaskInfo(task.ID, "Analyser %s: %d metrics collected (success=%v)",
			result.AnalyserName, len(result.Metrics), result.Success)

		if !result.Success {
			m.logger.TaskWarning(task.ID, "Analyser %s failed: %s", result.AnalyserName, result.Error)
			continue
		}

		for _, metric := range result.Metrics {
			formattedMetric := logging.FormatMetric(metric, false)
			m.logger.TaskInfo(task.ID, "Metric: %s", formattedMetric)
		}
	}

	// Store results if storage manager is available
	if m.storageManager != nil {
		m.logger.TaskInfo(task.ID, "Storing analysis results...")

		if spinner != nil {
			spinner.UpdateText("Storing analysis results...")
		}

		if err := m.storageManager.StoreResults(task.ID, repo.URL, results); err != nil {
			m.logger.TaskWarning(task.ID, "Warning: failed to store results: %v", err)
		} else {
			m.logger.TaskInfo(task.ID, "Results stored successfully")
		}
	}

	// Mark as completed
	m.tasksMu.Lock()
	task.Status = StatusCompleted
	task.EndTime = time.Now()
	m.tasksMu.Unlock()

	m.logger.TaskInfo(task.ID, "Task completed at %s", task.EndTime.Format(time.RFC3339))
	m.logger.CloseTaskLog(task.ID)

	// Show formatted results if display is available
	if m.display != nil {
		m.display.ShowResults(results)
	}
}

// markTaskFailed updates a task's status to failed
func (m *Manager) markTaskFailed(task *types.Task, errorMsg string) {
	m.tasksMu.Lock()
	task.Status = StatusFailed
	task.Error = errorMsg
	task.EndTime = time.Now()
	m.tasksMu.Unlock()

	m.logger.TaskError(task.ID, "Task failed: %s", errorMsg)
	m.logger.TaskInfo(task.ID, "Task ended at %s", task.EndTime.Format(time.RFC3339))
	m.logger.CloseTaskLog(task.ID)

	if m.display != nil {
		m.display.ShowError(errorMsg)
	}
}

// markTaskCancelled updates a task's status to cancelled
func (m *Manager) markTaskCancelled(task *types.Task, errorMsg string) {
	m.tasksMu.Lock()
	task.Status = StatusCancelled
	task.EndTime = time.Now()
	m.tasksMu.Unlock()

	m.logger.TaskWarning(task.ID, "Task was cancelled: %s", errorMsg)
	m.logger.TaskInfo(task.ID, "Task ended at %s", task.EndTime.Format(time.RFC3339))
	m.logger.CloseTaskLog(task.ID)

	if m.display != nil {
		m.display.ShowWarning(fmt.Sprintf("Task was cancelled: %s", errorMsg))
	}
}

// isURL checks if a string is a URL
func isURL(s string) bool {
	return s != "" && (len(s) > 4) && (strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@"))
}
