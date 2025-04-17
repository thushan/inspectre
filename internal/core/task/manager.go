package task

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/thushan/inspectre/internal/core/analysis"
	"github.com/thushan/inspectre/internal/core/repository"
	"github.com/thushan/inspectre/internal/extensions"
	"github.com/thushan/inspectre/internal/storage"
)

var (
	ErrTaskNotFound       = errors.New("task not found")
	ErrTaskAlreadyRunning = errors.New("task already running")
	ErrInvalidTaskState   = errors.New("invalid task state")
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
	tasks            map[string]*repository.Task
	tasksMu          sync.RWMutex
	logWriters       map[string]io.Writer
	logMu            sync.RWMutex
}

// NewManager creates a new task manager
func NewManager(repoManager *repository.Manager, storageManager *storage.Manager, extensionManager *extensions.Manager) *Manager {
	return &Manager{
		repoManager:      repoManager,
		storageManager:   storageManager,
		extensionManager: extensionManager,
		tasks:            make(map[string]*repository.Task),
		logWriters:       make(map[string]io.Writer),
	}
}

// CreateTask creates a new analysis task
func (m *Manager) CreateTask(nameOrURL string) (*repository.Task, error) {
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

	logFile, err := os.OpenFile(task.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}

	m.logMu.Lock()
	m.logWriters[taskID] = logFile
	m.logMu.Unlock()

	go func() {
		m.runTask(task)
	}()

	return nil
}

// GetTask retrieves a task by ID
func (m *Manager) GetTask(taskID string) (*repository.Task, error) {
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
func (m *Manager) ListTasks(showAll, showFailed bool) []*repository.Task {
	m.tasksMu.RLock()
	defer m.tasksMu.RUnlock()

	var result []*repository.Task

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

// WriteLog writes a message to the task's log
func (m *Manager) WriteLog(taskID, message string) error {
	m.logMu.RLock()
	writer, exists := m.logWriters[taskID]
	m.logMu.RUnlock()

	if !exists {
		return ErrTaskNotFound
	}

	_, err := fmt.Fprintln(writer, message)
	return err
}

// GetLogReader returns a reader for the task's log
func (m *Manager) GetLogReader(taskID string) (io.ReadCloser, error) {
	m.tasksMu.RLock()
	task, exists := m.tasks[taskID]
	m.tasksMu.RUnlock()

	if !exists {
		return nil, ErrTaskNotFound
	}

	file, err := os.OpenFile(task.LogFile, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return file, nil
}
func (m *Manager) CleanupTask(taskID string) error {
	m.tasksMu.RLock()
	task, exists := m.tasks[taskID]
	m.tasksMu.RUnlock()

	if !exists {
		return ErrTaskNotFound
	}

	// Make sure any log files are closed first
	m.CloseTaskLog(taskID)

	// Wait a bit to ensure all file handles are released
	time.Sleep(100 * time.Millisecond)

	// Use repo manager to clean up
	return m.repoManager.CleanUp(task)
}

// CloseTaskLog closes the log file for a task
func (m *Manager) CloseTaskLog(taskID string) {
	m.logMu.Lock()
	defer m.logMu.Unlock()

	if writer, exists := m.logWriters[taskID]; exists {
		if closer, ok := writer.(io.Closer); ok {
			_ = closer.Close() // Ignore error on close
		}
		delete(m.logWriters, taskID)
	}
}

// runTask performs the actual repository analysis
func (m *Manager) runTask(task *repository.Task) {
	logger := func(format string, args ...interface{}) {
		message := fmt.Sprintf(format, args...)
		m.WriteLog(task.ID, message)
	}

	// Set the task to running state
	logger("Task started at %s", time.Now().Format(time.RFC3339))

	// Get repository details
	repo, err := m.repoManager.GetRepository(task.Repository)
	if err != nil {
		// Handle URLs that aren't in the config
		if !isURL(task.Repository) {
			m.markTaskFailed(task, fmt.Sprintf("Repository not found: %v", err))
			return
		}

		// Create temporary repository object for direct URLs
		repo = &repository.Repository{
			URL:  task.Repository,
			Type: repository.GuessRepoType(task.Repository),
			Auth: repository.Auth{}, // Empty Auth struct
		}
	}

	// Clone the repository to the repo directory
	logger("Cloning repository %s to %s", repo.URL, task.RepoDir)
	err = m.repoManager.Clone(repo, task.RepoDir)
	if err != nil {
		m.markTaskFailed(task, fmt.Sprintf("Failed to clone repository: %v", err))
		return
	}

	// Run analysis
	logger("Repository cloned successfully. Beginning analysis...")

	// Create analyser list starting with built-in analysers
	analysers := []analysis.Analyser{
		analysis.NewFileAnalyser(),
		analysis.NewGitAnalyser(),
	}

	// Add extension analysers if extension manager is available
	if m.extensionManager != nil {
		logger("Loading extensions...")
		extAnalysers, err := m.extensionManager.LoadAllEnabled()
		if err != nil {
			logger("Warning: failed to load some extensions: %v", err)
		}

		if len(extAnalysers) > 0 {
			analysers = append(analysers, extAnalysers...)
			logger("Loaded %d extension analysers", len(extAnalysers))
		}
	}

	analyserManager := analysis.NewManager(analysers, logger)

	// Prepare environment variables for analysers
	env := map[string]string{
		"REPOSITORY_NAME": repo.Name,
		"REPOSITORY_URL":  repo.URL,
		"REPOSITORY_TYPE": repo.Type,
		"TASK_ID":         task.ID,
		"ASSETS_DIR":      task.AssetsDir, // Add assets dir to environment
	}

	logger("Running analysers on repository...")
	// Use repo directory instead of work directory
	results, err := analyserManager.AnalyseRepository(task.RepoDir, env)
	if err != nil {
		m.markTaskFailed(task, fmt.Sprintf("Analysis failed: %v", err))
		return
	}

	logger("Analysis completed with %d result sets", len(results))
	for _, result := range results {
		logger("Analyser %s: %d metrics collected (success=%v)",
			result.AnalyserName, len(result.Metrics), result.Success)

		if !result.Success {
			logger("Analyser %s failed: %s", result.AnalyserName, result.Error)
			continue
		}

		for _, metric := range result.Metrics {
			if metric.Key == "" {
				logger("Metric: %s = %v", metric.Name, metric.Value)
			} else {
				logger("Metric: %s [%s] = %v", metric.Name, metric.Key, metric.Value)
			}
		}
	}

	// Store results if storage manager is available
	if m.storageManager != nil {
		logger("Storing analysis results...")
		if err := m.storageManager.StoreResults(task.ID, repo.URL, results); err != nil {
			logger("Warning: failed to store results: %v", err)
		} else {
			logger("Results stored successfully")
		}
	}

	// Mark as completed
	m.tasksMu.Lock()
	task.Status = StatusCompleted
	task.EndTime = time.Now()
	m.tasksMu.Unlock()

	logger("Task completed at %s", task.EndTime.Format(time.RFC3339))

	m.CloseTaskLog(task.ID)
}

// markTaskFailed updates a task's status to failed
func (m *Manager) markTaskFailed(task *repository.Task, errorMsg string) {
	m.tasksMu.Lock()
	task.Status = StatusFailed
	task.Error = errorMsg
	task.EndTime = time.Now()
	m.tasksMu.Unlock()

	m.WriteLog(task.ID, fmt.Sprintf("Task failed: %s", errorMsg))
	m.WriteLog(task.ID, fmt.Sprintf("Task ended at %s", task.EndTime.Format(time.RFC3339)))

	m.CloseTaskLog(task.ID)
}

// isURL checks if a string is a URL
func isURL(s string) bool {
	return s != "" && (s[:7] == "http://" || s[:8] == "https://" || s[:4] == "git@")
}
