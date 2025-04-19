package task

import (
	"context"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
	"time"

	"github.com/thushan/inspectre/internal/core/analyser"
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
	TaskExecutionTimeout = 30 * time.Minute
	CloneTimeout         = 10 * time.Minute

	// Worker scaling
	WorkerScaleInterval = 5 * time.Second
	IdleWorkerTimeout   = 30 * time.Second
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
	Results     []*analyser.Result
	Error       error
	CompletedAt time.Time
}

// WorkerState tracks the state of a worker
type WorkerState struct {
	ID        int
	IsIdle    bool
	LastUsed  time.Time
	TaskCount int
	Cancel    context.CancelFunc
}

// Manager handles analyser tasks
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
	taskQueue    chan *types.Task
	taskResults  chan TaskResult
	uiEvents     chan UIEvent
	workerStates map[int]*WorkerState
	workersMu    sync.RWMutex

	// Shutdown coordination
	shutdownOnce sync.Once
	wg           sync.WaitGroup

	// Configuration
	minWorkerCount int
	maxWorkerCount int
	currentWorkers int
	closed         bool
	closedMu       sync.RWMutex
}

// NewManager creates a new task manager
func NewManager(repoManager *repository.Manager, storageManager *storage.Manager, extensionManager *extensions.Manager, appCtx *appctx.AppContext) *Manager {
	// Create contexts for the manager and worker pool
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

	manager := &Manager{
		repoManager:      repoManager,
		storageManager:   storageManager,
		extensionManager: extensionManager,
		tasks:            make(map[string]*types.Task),
		ctx:              ctx,
		cancelFunc:       cancel,
		logger:           logging.GetLogger(),

		// Channels sized to avoid blocking in normal scenarios
		taskQueue:    make(chan *types.Task, TaskQueueSize),
		taskResults:  make(chan TaskResult, TaskResultSize),
		uiEvents:     make(chan UIEvent, UIEventSize),
		workerStates: make(map[int]*WorkerState),

		minWorkerCount: minWorkers,
		maxWorkerCount: maxWorkers,
		currentWorkers: 0,
		closed:         false,
	}

	// Start background workers
	manager.startWorkers()

	// Start worker scaling goroutine
	manager.startWorkerScaling()

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
		m.workersMu.Lock()
		for _, worker := range m.workerStates {
			worker.Cancel()
		}
		m.workersMu.Unlock()

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

		// Close channels in the correct order
		// First stop accepting new tasks
		close(m.taskQueue)

		// Give time for any pending results to be processed
		time.AfterFunc(200*time.Millisecond, func() {
			close(m.taskResults)
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
