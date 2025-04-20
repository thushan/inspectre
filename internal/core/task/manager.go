package task

import (
	"context"
	"errors"
	"fmt"
	"github.com/thushan/inspectre/internal/core/ui/theme"
	"io"
	"runtime"
	"time"

	"github.com/panjf2000/ants/v2"
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

	// Concurrency settings
	MinWorkerCount = 2
	MaxWorkerCount = 8

	// Channel buffer sizes
	TaskQueueSize  = 100
	TaskResultSize = 100
	UIEventSize    = 100

	// Timeouts
	ShutdownTimeout      = 10 * time.Second
	TaskExecutionTimeout = 30 * time.Minute
	CloneTimeout         = 10 * time.Minute

	// Worker pool configuration
	IdleWorkerTimeout  = 30 * time.Second
	NonBlockingSubmit  = false
	PreAllocateWorkers = true
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskAlreadyRunning = errors.New("task already running")
	ErrInvalidTaskState   = errors.New("invalid task state")
	ErrTaskCancelled      = errors.New("task was cancelled")
	ErrManagerClosed      = errors.New("task manager is closed")
	ErrTaskQueueFull      = errors.New("task queue is full")
)

// NewManager creates a new task manager
func NewManager(repoManager *repository.Manager, storageManager *storage.Manager, extensionManager *extensions.Manager, appCtx *appctx.AppContext) *Manager {
	// Create contexts for the manager
	ctx, cancel := context.WithCancel(context.Background())

	// Set worker count based on number of CPU cores, with reasonable limits
	cpuCount := runtime.NumCPU()
	minWorkers := MinWorkerCount
	if cpuCount < minWorkers {
		minWorkers = cpuCount
	}

	maxWorkers := MaxWorkerCount
	if cpuCount < maxWorkers {
		maxWorkers = cpuCount
	}

	if minWorkers < 1 {
		minWorkers = 1
	}

	logger := logging.GetLogger()

	// Initialize the ants worker pool
	pool, err := ants.NewPool(maxWorkers,
		ants.WithExpiryDuration(IdleWorkerTimeout),
		ants.WithPreAlloc(PreAllocateWorkers),
		ants.WithNonblocking(NonBlockingSubmit),
		ants.WithLogger(antsLogger{logger: logger}))

	if err != nil {
		logger.Error("Failed to create worker pool: %v", err)
		// Fallback to a minimal pool size if creation fails
		pool, _ = ants.NewPool(minWorkers)
	}

	manager := &Manager{
		repoManager:      repoManager,
		storageManager:   storageManager,
		extensionManager: extensionManager,
		tasks:            make(map[string]*types.Task),
		ctx:              ctx,
		cancelFunc:       cancel,
		logger:           logger,

		// Channels sized to avoid blocking in normal scenarios
		taskResults: make(chan TaskResult, TaskResultSize),
		uiEvents:    make(chan UIEvent, UIEventSize),
		workerPool:  pool,
		closed:      false,
	}

	// Handle task results and UI events
	manager.startEventHandlers()

	// Register shutdown hook if app context is provided
	if appCtx != nil {
		appCtx.AddShutdownHookWithPriority(
			"taskmanager.shutdown",
			appctx.PriorityNormal,
			manager.Shutdown,
		)
	}

	return manager
}

// SetDisplay sets the display manager
func (m *Manager) SetDisplay(display types.DisplayProvider) {
	m.display = display
}

// CreateTask creates a new analyser task
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

	// Create a copy of the task for the worker
	taskCopy := *task

	// Submit task to the worker pool
	err := m.workerPool.Submit(func() {
		m.processTask(&taskCopy)
	})

	if err != nil {
		// If submission fails, reset the task status
		m.tasksMu.Lock()
		task.Status = StatusCreated
		m.tasksMu.Unlock()

		m.logger.Error("Failed to submit task to worker pool: %v", err)
		return fmt.Errorf("failed to queue task: %w", err)
	}

	m.logger.Info("Task %s submitted to worker pool", theme.ColourTaskId(task.ID))
	return nil
}

// processTask is executed by the worker pool to handle a task
func (m *Manager) processTask(task *types.Task) {
	// Update task status to Running
	m.tasksMu.Lock()
	storedTask, exists := m.tasks[task.ID]
	if !exists {
		m.tasksMu.Unlock()
		m.logger.Warning("Task %s not found in task map", theme.ColourTaskId(task.ID))
		return
	}

	// Skip if task was cancelled while queued
	if storedTask.Status == StatusCancelled {
		m.tasksMu.Unlock()
		m.logger.TaskInfo(task.ID, "Task was cancelled before execution")
		return
	}

	storedTask.Status = StatusRunning
	m.tasksMu.Unlock()

	// Log start of processing
	m.logger.TaskInfo(task.ID, "Processing task %s for repository %s", theme.ColourTaskId(task.ID), theme.ColourRepository(task.Repository))

	// Send UI event
	m.sendUIEvent(UIEvent{
		TaskID:    task.ID,
		EventType: "started",
		Message:   fmt.Sprintf("Processing repository %s", theme.ColourRepository(task.Repository)),
	})

	// Execute the task
	m.logger.Debug("Executing task %s", theme.ColourTaskId(task.ID))
	result := m.executeTask(task)
	m.logger.Debug("Completed execution of task %s", theme.ColourTaskId(task.ID))

	// Send result for processing with timeout to prevent deadlocks
	m.logger.Debug("Sending results for task %s", theme.ColourTaskId(task.ID))
	select {
	case m.taskResults <- result:
		m.logger.Debug("Results for task %s sent successfully", theme.ColourTaskId(task.ID))
	case <-m.ctx.Done():
		// Context cancelled, exit worker
		m.logger.Warning("Context cancelled during result send for task %s", theme.ColourTaskId(task.ID))
		return
	case <-time.After(5 * time.Second):
		// Couldn't send results after timeout, log and continue
		m.logger.Warning("Timeout sending results for task %s, dropping results", theme.ColourTaskId(task.ID))
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
	var err error

	m.shutdownOnce.Do(func() {
		m.logger.Info("Shutting down task manager...")

		// Mark as closed first to prevent new operations
		m.closedMu.Lock()
		m.closed = true
		m.closedMu.Unlock()

		// Create a child context with timeout if parent doesn't have one
		shutdownCtx, cancel := context.WithTimeout(ctx, ShutdownTimeout)
		defer cancel()

		// Update status of any running tasks
		m.tasksMu.Lock()
		for id, task := range m.tasks {
			if task.Status == StatusRunning || task.Status == StatusQueued {
				task.Status = StatusCancelled
				task.EndTime = time.Now()
				m.logger.TaskInfo(id, "Task cancelled during shutdown")
			}
		}
		m.tasksMu.Unlock()
		m.logger.Info("All running tasks marked as cancelled")

		// Release the worker pool
		m.workerPool.Release()
		m.logger.Info("Worker pool released")

		// Wait briefly to allow any in-progress tasks to complete
		select {
		case <-time.After(200 * time.Millisecond):
		case <-shutdownCtx.Done():
			err = fmt.Errorf("shutdown timed out while waiting for workers: %w", shutdownCtx.Err())
			m.logger.Warning("Shutdown timeout while waiting for workers")
		}

		// Close result and event channels
		close(m.taskResults)
		close(m.uiEvents)
		m.logger.Info("All channels closed")

		// Finally, cancel the main context
		m.cancelFunc()
		m.logger.Info("Main context cancelled")

		// Wait for all goroutines to complete with timeout
		waitCh := make(chan struct{})
		go func() {
			m.wg.Wait()
			close(waitCh)
		}()

		select {
		case <-waitCh:
			m.logger.Info("All goroutines successfully terminated")
		case <-shutdownCtx.Done():
			err = fmt.Errorf("shutdown timed out waiting for goroutines: %w", shutdownCtx.Err())
			m.logger.Warning("Timeout waiting for goroutines to terminate, some resources may leak")
		}

		// Close task logs
		for id := range m.tasks {
			m.logger.CloseTaskLog(id)
		}
		m.logger.Info("All task logs closed")
	})

	return err
}

// Custom logger implementation for ants
type antsLogger struct {
	logger *logging.Logger
}

func (l antsLogger) Printf(format string, args ...interface{}) {
	l.logger.Debug("ants: "+format, args...)
}
