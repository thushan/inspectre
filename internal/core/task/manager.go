// internal/core/task/manager.go - Fixed version
package task

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
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

// Task status constants
const (
	StatusCreated   = "Created"
	StatusQueued    = "Queued"
	StatusRunning   = "Running"
	StatusCompleted = "Completed"
	StatusFailed    = "Failed"
	StatusCancelled = "Cancelled"
)

// Default concurrency settings
const (
	DefaultWorkerCount = 2
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskAlreadyRunning = errors.New("task already running")
	ErrInvalidTaskState   = errors.New("invalid task state")
	ErrTaskCancelled      = errors.New("task was cancelled")
	ErrManagerClosed      = errors.New("task manager is closed")
)

// UIEvent represents an event related to task execution that needs UI attention
type UIEvent struct {
	TaskID    string
	EventType string
	Message   string
	Data      interface{}
}

// TaskResult contains the output of a task run
type TaskResult struct {
	TaskID      string
	Repository  string
	Results     []*analysis.Result
	Error       error
	CompletedAt time.Time
}

// Manager handles analysis tasks
type Manager struct {
	repoManager      *repository.Manager
	storageManager   *storage.Manager
	extensionManager *extensions.Manager
	tasks            map[string]*types.Task
	tasksMu          sync.RWMutex
	ctx              context.Context
	cancelFunc       context.CancelFunc
	logger           *logging.Logger
	display          types.DisplayProvider

	// Worker pool related fields
	taskQueue   chan *types.Task
	taskResults chan TaskResult
	uiEvents    chan UIEvent
	workers     []context.CancelFunc

	// Shutdown coordination
	shutdownOnce sync.Once
	wg           sync.WaitGroup

	// Configuration
	workerCount int
	closed      bool
	closedMu    sync.RWMutex
}

// NewManager creates a new task manager
func NewManager(repoManager *repository.Manager, storageManager *storage.Manager, extensionManager *extensions.Manager, appCtx *appctx.AppContext) *Manager {
	// Create contexts for the manager and worker pool
	ctx, cancel := context.WithCancel(context.Background())

	// Set default worker count based on number of CPU cores, but not less than 1
	workerCount := runtime.NumCPU()
	if workerCount > DefaultWorkerCount {
		workerCount = DefaultWorkerCount
	}
	if workerCount < 1 {
		workerCount = 1
	}

	manager := &Manager{
		repoManager:      repoManager,
		storageManager:   storageManager,
		extensionManager: extensionManager,
		tasks:            make(map[string]*types.Task),
		ctx:              ctx,
		cancelFunc:       cancel,
		logger:           logging.GetLogger(),

		// Channels sized to avoid blocking in normal scenarios
		taskQueue:   make(chan *types.Task, 100),
		taskResults: make(chan TaskResult, 100),
		uiEvents:    make(chan UIEvent, 100),

		workers:     make([]context.CancelFunc, 0, workerCount),
		workerCount: workerCount,
		closed:      false,
	}

	// Start background workers
	manager.startWorkers()

	// Handle task results and UI events
	manager.startEventHandlers()

	// Register shutdown hook if app context is provided
	if appCtx != nil {
		appCtx.AddShutdownHook(manager.Shutdown)
	}

	return manager
}

// startWorkers initializes and starts the worker pool
func (m *Manager) startWorkers() {
	// Start worker goroutines
	for i := 0; i < m.workerCount; i++ {
		workerCtx, workerCancel := context.WithCancel(m.ctx)
		m.workers = append(m.workers, workerCancel)

		workerId := i
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.workerLoop(workerCtx, workerId)
		}()
	}
}

// startEventHandlers starts background goroutines to process task results and UI events
func (m *Manager) startEventHandlers() {
	// Process task results
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.ctx.Done():
				return
			case result, ok := <-m.taskResults:
				if !ok {
					return
				}
				m.processTaskResult(result)
			}
		}
	}()

	// Process UI events if display is available
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for {
			select {
			case <-m.ctx.Done():
				return
			case event, ok := <-m.uiEvents:
				if !ok {
					return
				}
				m.handleUIEvent(event)
			}
		}
	}()
}

// workerLoop is the main execution loop for a worker
func (m *Manager) workerLoop(ctx context.Context, workerID int) {
	m.logger.Debug("Worker %d started", workerID)

	for {
		select {
		case <-ctx.Done():
			m.logger.Debug("Worker %d shutting down: %v", workerID, ctx.Err())
			return

		case task, ok := <-m.taskQueue:
			if !ok {
				// Channel closed, exit worker
				m.logger.Debug("Worker %d exiting: task queue closed", workerID)
				return
			}

			// Check if this worker should handle this task
			m.tasksMu.Lock()
			if task.Status != StatusQueued {
				// Task was cancelled or already processed
				m.tasksMu.Unlock()
				continue
			}

			// Mark as running
			task.Status = StatusRunning
			m.tasksMu.Unlock()

			// Log start of processing
			m.logger.TaskInfo(task.ID, "Worker %d processing task %s for repository %s",
				workerID, task.ID, task.Repository)

			// Send UI event
			m.sendUIEvent(UIEvent{
				TaskID:    task.ID,
				EventType: "started",
				Message:   fmt.Sprintf("Processing repository %s", task.Repository),
			})

			// Execute the task
			result := m.executeTask(task)

			// Send result for processing
			select {
			case m.taskResults <- result:
				// Result sent successfully
			case <-ctx.Done():
				// Context cancelled, exit worker
				return
			}
		}
	}
}

// executeTask performs the actual repository analysis
func (m *Manager) executeTask(task *types.Task) TaskResult {
	// Create task-specific context that can be cancelled
	taskCtx, taskCancel := context.WithCancel(m.ctx)
	defer taskCancel()

	result := TaskResult{
		TaskID:      task.ID,
		Repository:  task.Repository,
		CompletedAt: time.Now(),
	}

	// Get repository details
	repo, err := m.getRepositoryDetails(task.Repository)
	if err != nil {
		result.Error = err
		return result
	}

	// Check if task context is cancelled
	if taskCtx.Err() != nil {
		result.Error = ErrTaskCancelled
		return result
	}

	// Ensure directories exist
	if err := m.ensureTaskDirectories(task); err != nil {
		result.Error = err
		return result
	}

	// Send UI event
	m.sendUIEvent(UIEvent{
		TaskID:    task.ID,
		EventType: "cloning",
		Message:   fmt.Sprintf("Cloning repository %s", repo.URL),
	})

	// Clone the repository
	m.logger.TaskInfo(task.ID, "Cloning repository %s to %s", repo.URL, task.RepoDir)
	if err := m.repoManager.Clone(repo, task.RepoDir); err != nil {
		result.Error = fmt.Errorf("failed to clone repository: %w", err)
		return result
	}

	// Check if task context is cancelled
	if taskCtx.Err() != nil {
		result.Error = ErrTaskCancelled
		return result
	}

	// Run analysis
	m.logger.TaskInfo(task.ID, "Repository cloned successfully. Beginning analysis...")

	// Send UI event
	m.sendUIEvent(UIEvent{
		TaskID:    task.ID,
		EventType: "analysing",
		Message:   "Running analysers...",
	})

	// Create analyser list starting with built-in analysers
	analysers := m.prepareAnalysers(task)

	// Create logging function for analyser manager
	loggerFn := func(format string, args ...interface{}) {
		m.logger.TaskInfo(task.ID, format, args...)
	}

	// Set up analyser manager
	analyserManager := analysis.NewManager(analysers, loggerFn)

	// Prepare environment variables for analysers
	env := m.createAnalyserEnvironment(task, repo)

	// Check context again
	if taskCtx.Err() != nil {
		result.Error = ErrTaskCancelled
		return result
	}

	// Run the analysers
	m.logger.TaskInfo(task.ID, "Running analysers on repository...")

	results, err := analyserManager.AnalyseRepository(task.RepoDir, env)
	if err != nil {
		result.Error = fmt.Errorf("analysis failed: %w", err)
		return result
	}

	// Log results
	m.logger.TaskInfo(task.ID, "Analysis completed with %d result sets", len(results))

	// Store results in output
	result.Results = results

	return result
}

// getRepositoryDetails retrieves or creates repository details
func (m *Manager) getRepositoryDetails(repoNameOrURL string) (*types.Repository, error) {
	repo, err := m.repoManager.GetRepository(repoNameOrURL)
	if err != nil {
		// Handle URLs that aren't in the config
		if !isURLString(repoNameOrURL) {
			return nil, fmt.Errorf("repository not found: %w", err)
		}

		// Create temporary repository object for direct URLs
		repo = &types.Repository{
			URL:  repoNameOrURL,
			Type: repository.GuessRepoType(repoNameOrURL),
			Auth: types.Auth{}, // Empty Auth struct
		}
	}

	return repo, nil
}

// ensureTaskDirectories creates required directories for a task
func (m *Manager) ensureTaskDirectories(task *types.Task) error {
	dirs := []string{task.BaseDir, task.RepoDir, task.AssetsDir}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Register task log file
	if err := m.logger.RegisterTaskLog(task.ID, task.LogFile); err != nil {
		return fmt.Errorf("failed to register task log: %w", err)
	}

	return nil
}

// prepareAnalysers creates the list of analysers to run
func (m *Manager) prepareAnalysers(task *types.Task) []analysis.Analyser {
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

	return analysers
}

// createAnalyserEnvironment prepares environment variables for analysers
func (m *Manager) createAnalyserEnvironment(task *types.Task, repo *types.Repository) map[string]string {
	return map[string]string{
		"REPOSITORY_NAME": repo.Name,
		"REPOSITORY_URL":  repo.URL,
		"REPOSITORY_TYPE": repo.Type,
		"TASK_ID":         task.ID,
		"ASSETS_DIR":      task.AssetsDir,
	}
}

// processTaskResult handles task completion, updates task status, and stores results
func (m *Manager) processTaskResult(result TaskResult) {
	m.tasksMu.Lock()
	task, exists := m.tasks[result.TaskID]
	if !exists {
		m.tasksMu.Unlock()
		m.logger.Warning("Received result for unknown task %s", result.TaskID)
		return
	}

	// Update task status and end time
	task.EndTime = result.CompletedAt

	if result.Error != nil {
		task.Status = StatusFailed
		task.Error = result.Error.Error()
		m.logger.TaskError(task.ID, "Task failed: %v", result.Error)
	} else {
		task.Status = StatusCompleted
		m.logger.TaskInfo(task.ID, "Task completed successfully")
	}
	m.tasksMu.Unlock()

	// Send completion UI event
	eventType := "completed"
	if result.Error != nil {
		eventType = "failed"
	}

	m.sendUIEvent(UIEvent{
		TaskID:    task.ID,
		EventType: eventType,
		Message:   task.Status,
		Data:      result.Results,
	})

	// Store results if successful and storage manager is available
	if result.Error == nil && m.storageManager != nil && result.Results != nil {
		m.logger.TaskInfo(task.ID, "Storing analysis results...")

		if err := m.storageManager.StoreResults(task.ID, result.Repository, result.Results); err != nil {
			m.logger.TaskWarning(task.ID, "Warning: failed to store results: %v", err)
		} else {
			m.logger.TaskInfo(task.ID, "Results stored successfully")
		}
	}

	// Close task log
	m.logger.CloseTaskLog(task.ID)

	// Log completion
	m.logger.TaskInfo(task.ID, "Task ended at %s", task.EndTime.Format(time.RFC3339))
}

// handleUIEvent processes events that need UI updates
func (m *Manager) handleUIEvent(event UIEvent) {
	// Skip if no display is configured
	if m.display == nil {
		return
	}

	switch event.EventType {
	case "started":
		m.display.StartSpinner(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))

	case "cloning":
		m.display.UpdateSpinnerText(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))

	case "analysing":
		m.display.UpdateSpinnerText(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))

	case "completed":
		m.display.StopSpinner(fmt.Sprintf("[%s] Analysis completed successfully", event.TaskID))
		if results, ok := event.Data.([]*analysis.Result); ok {
			m.display.ShowResults(results)
		}

	case "failed":
		m.display.ShowError(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))

	case "cancelled":
		m.display.ShowWarning(fmt.Sprintf("[%s] Task was cancelled: %s", event.TaskID, event.Message))

	case "progress":
		m.display.UpdateSpinnerText(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))
	}
}

// sendUIEvent sends an event to the UI event channel
func (m *Manager) sendUIEvent(event UIEvent) {
	select {
	case m.uiEvents <- event:
		// Event sent successfully
	case <-m.ctx.Done():
		// Context cancelled, don't send event
	default:
		// Channel full, log and continue (non-blocking)
		m.logger.Warning("UI event channel full, event %s for task %s dropped",
			event.EventType, event.TaskID)
	}
}

// SetDisplay sets the display manager
func (m *Manager) SetDisplay(display types.DisplayProvider) {
	m.display = display
}

// CreateTask creates a new analysis task
func (m *Manager) CreateTask(nameOrURL string) (*types.Task, error) {
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return nil, ErrManagerClosed
	}
	m.closedMu.RUnlock()

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
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return ErrManagerClosed
	}
	m.closedMu.RUnlock()

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

	// Change status to queued
	task.Status = StatusQueued
	m.tasksMu.Unlock()

	// Ensure task log file is registered
	if err := m.logger.RegisterTaskLog(task.ID, task.LogFile); err != nil {
		m.logger.Warning("Failed to register task log: %v", err)
	}

	// Log task start
	m.logger.TaskInfo(task.ID, "Task queued at %s", time.Now().Format(time.RFC3339))

	// Send to worker pool
	select {
	case m.taskQueue <- task:
		// Successfully queued task
		return nil
	case <-m.ctx.Done():
		// Manager is shutting down
		return fmt.Errorf("task manager shutting down: %w", m.ctx.Err())
	default:
		// Queue full (should not happen with properly sized queue)
		m.tasksMu.Lock()
		task.Status = StatusCreated // Reset status
		m.tasksMu.Unlock()
		return fmt.Errorf("task queue is full")
	}
}

// CancelTask cancels a running task
func (m *Manager) CancelTask(taskID string) error {
	m.tasksMu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.tasksMu.Unlock()
		return ErrTaskNotFound
	}

	// Only queued or running tasks can be cancelled
	if task.Status != StatusQueued && task.Status != StatusRunning {
		m.tasksMu.Unlock()
		return ErrInvalidTaskState
	}

	// Update status
	prevStatus := task.Status
	task.Status = StatusCancelled
	task.EndTime = time.Now()
	m.tasksMu.Unlock()

	m.logger.TaskInfo(taskID, "Task cancelled by user (previous status: %s)", prevStatus)

	// Send UI event
	m.sendUIEvent(UIEvent{
		TaskID:    taskID,
		EventType: "cancelled",
		Message:   "Task cancelled by user",
	})

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
			// Make a copy to avoid concurrent modification
			taskCopy := *task
			result = append(result, &taskCopy)
		} else if showFailed && task.Status == StatusFailed {
			taskCopy := *task
			result = append(result, &taskCopy)
		} else if task.Status == StatusRunning || task.Status == StatusQueued || task.Status == StatusCreated {
			taskCopy := *task
			result = append(result, &taskCopy)
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
	// Only shut down once
	var alreadyClosed bool
	m.shutdownOnce.Do(func() {
		m.logger.Info("Shutting down task manager...")

		// Mark as closed to prevent new tasks
		m.closedMu.Lock()
		m.closed = true
		m.closedMu.Unlock()

		// Cancel all workers
		for _, cancel := range m.workers {
			cancel()
		}

		// Close the task queue to signal workers to exit after processing current tasks
		close(m.taskQueue)

		// Cancel main context after a short delay to allow for cleanup
		time.AfterFunc(100*time.Millisecond, func() {
			m.cancelFunc()
		})

		// Cancel any running tasks
		m.tasksMu.Lock()
		for id, task := range m.tasks {
			if task.Status == StatusRunning || task.Status == StatusQueued {
				task.Status = StatusCancelled
				task.EndTime = time.Now()
				m.logger.TaskInfo(id, "Task cancelled during shutdown")
			}
		}
		m.tasksMu.Unlock()

		// Close UI events channel after a delay
		time.AfterFunc(200*time.Millisecond, func() {
			close(m.uiEvents)
		})
	})

	if alreadyClosed {
		return nil
	}

	// Wait for context timeout or all goroutines to finish
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(50 * time.Millisecond):
		// Give a little time for everything to process
	}

	// Wait for all workers to complete with timeout
	waitCh := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(waitCh)
	}()

	select {
	case <-waitCh:
		// All workers completed
		m.logger.Info("Task manager shutdown completed")
	case <-ctx.Done():
		// Timeout or cancellation
		m.logger.Warning("Task manager shutdown timed out or was cancelled")
		return ctx.Err()
	}

	return nil
}

// isURLString checks if a string is a URL
func isURLString(s string) bool {
	return s != "" && (len(s) > 4) && (strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@"))
}
