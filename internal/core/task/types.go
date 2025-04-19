package task

import (
	"context"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/repository"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/extensions"
	"github.com/thushan/inspectre/internal/storage"
	"sync"
	"time"
)

// Use the common Task type from types package
type Task = types.Task

// TaskManager handles task operations
type TaskManager interface {
	types.TaskManager
}

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
