package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/ui"
	"github.com/thushan/inspectre/internal/ui/theme"
)

type Manager struct {
	tasks           map[string]*Task
	executor        *Executor
	lock            sync.RWMutex
	taskCounter     int
	progressManager *ui.ProgressManager
	eventEmitter    *EventEmitter
}

func NewManager() *Manager {
	executor, err := NewExecutor(0)
	if err != nil {
		panic(fmt.Sprintf("Failed to create executor: %v", err))
	}

	manager := &Manager{
		tasks:           make(map[string]*Task),
		executor:        executor,
		progressManager: ui.NewProgressManager(),
		eventEmitter:    executor.eventEmitter,
	}

	executor.RegisterUIHandlers(manager.handleEvent)

	return manager
}

func (m *Manager) CreateTask(description string, repo string) *Task {
	m.lock.Lock()
	defer m.lock.Unlock()

	m.taskCounter++
	taskID := fmt.Sprintf("task_%d", m.taskCounter)

	task := &Task{
		ID:          taskID,
		Description: description,
		Status:      TaskCreated,
		Repository:  repo,
	}

	m.tasks[taskID] = task

	m.progressManager.CreateTracker(taskID, description)

	return task
}

func (m *Manager) Run(repoTarget string, outputFormat string) {
	task := m.CreateTask("Repository analysis", repoTarget)

	err := m.executor.Execute(task, func(ctx context.Context) error {
		// Here is where the actual repository analysis would occur
		// This is a placeholder for the actual implementation

		// Update progress through events
		for i := 0; i <= 100; i += 10 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				m.eventEmitter.EmitTaskProgress(task.ID, float64(i),
					fmt.Sprintf("Analysis in progress: %d%%", i))
				time.Sleep(200 * time.Millisecond)
			}
		}

		return nil
	}, &types.TaskOptions{
		Timeout:    5 * time.Minute,
		MaxRetries: 1,
	})

	if err != nil {
		tracker := m.progressManager.GetTracker(task.ID)
		if tracker != nil {
			tracker.Fail("Failed to submit task", err)
		}
	}

	// Wait for task to complete or timeout
	// In a real implementation, you'd want to make this non-blocking
	// or provide a way to detach and track later
	m.waitForTaskCompletion(task.ID, 30*time.Second)
}

// GenerateInsights generates insights for a repository
func (m *Manager) GenerateInsights(repoTarget string, format string, outputFile string) {
	task := m.CreateTask("Generate insights", repoTarget)

	// Execute the task with a timeout
	err := m.executor.Execute(task, func(ctx context.Context) error {
		// Here is where the actual insights generation would occur
		// This is a placeholder for the actual implementation

		// Update progress through events
		steps := []string{
			"Analysing repository structure...",
			"Identifying key components...",
			"Processing dependencies...",
			"Generating architectural diagram...",
			"Finalising insights...",
		}

		for i, step := range steps {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
				progress := float64(i+1) / float64(len(steps)) * 100
				m.eventEmitter.EmitTaskProgress(task.ID, progress, step)
				time.Sleep(600 * time.Millisecond)
			}
		}

		// Set task result
		task.Result = &types.RepositoryAnalysisResult{
			Repository: &types.RepositoryInfo{
				Name: repoTarget,
				URL:  repoTarget,
			},
			Metrics: map[string]float64{
				"complexity_score": 78.5,
				"maintainability":  64.2,
			},
		}

		return nil
	}, &types.TaskOptions{
		Timeout:    10 * time.Minute,
		MaxRetries: 1,
	})

	if err != nil {
		tracker := m.progressManager.GetTracker(task.ID)
		if tracker != nil {
			tracker.Fail("Failed to submit task", err)
		}
		return
	}

	// Wait for task to complete or timeout
	success := m.waitForTaskCompletion(task.ID, 60*time.Second)

	// Show additional information if the task completed successfully
	if success {
		outputInfo := "Insights generated"
		if outputFile != "" {
			outputInfo = fmt.Sprintf("Insights saved to %s", outputFile)
		}
		pterm.Success.Println(outputInfo)
	}
}

// ListTasks lists all tasks
func (m *Manager) ListTasks(all bool, failed bool, format string) {
	m.lock.RLock()
	defer m.lock.RUnlock()

	var data [][]string
	data = append(data, []string{"ID", "Status", "Repository", "Duration", "Description"})

	for _, task := range m.tasks {
		// Filter based on flags
		if (!all && task.Status != TaskRunning && task.Status != TaskFailed) ||
			(failed && task.Status != TaskFailed) {
			continue
		}

		// Calculate duration
		var duration string
		if !task.StartTime.IsZero() {
			endTime := task.EndTime
			if endTime.IsZero() {
				endTime = time.Now()
			}
			duration = endTime.Sub(task.StartTime).Round(time.Second).String()
		} else {
			duration = "-"
		}

		// Format status based on its value
		var status string
		switch task.Status {
		case TaskRunning:
			status = pterm.LightBlue(string(task.Status))
		case TaskCompleted:
			status = pterm.LightGreen(string(task.Status))
		case TaskFailed:
			status = pterm.LightRed(string(task.Status))
		default:
			status = string(task.Status)
		}

		data = append(data, []string{
			theme.ColourTaskId(task.ID),
			status,
			theme.ColourRepository(task.Repository),
			duration,
			task.Description,
		})
	}

	if len(data) > 1 {
		pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	} else {
		pterm.Info.Println("No tasks found")
	}
}

// ShowLogs shows logs for a specific task
func (m *Manager) ShowLogs(taskID string, follow bool) {
	m.lock.RLock()
	task, exists := m.tasks[taskID]
	m.lock.RUnlock()

	if !exists {
		pterm.Error.Println("Task not found:", taskID)
		return
	}

	// Print task information
	pterm.DefaultSection.Println("Task Information")
	pterm.Println("ID:", theme.ColourTaskId(task.ID))
	pterm.Println("Description:", task.Description)
	pterm.Println("Repository:", theme.ColourRepository(task.Repository))
	pterm.Println("Status:", string(task.Status))

	if !task.StartTime.IsZero() {
		pterm.Println("Start Time:", task.StartTime.Format("2006-01-02 15:04:05"))

		if !task.EndTime.IsZero() {
			pterm.Println("End Time:", task.EndTime.Format("2006-01-02 15:04:05"))
			pterm.Println("Duration:", task.EndTime.Sub(task.StartTime).Round(time.Second))
		} else if task.Status == TaskRunning {
			pterm.Println("Running for:", time.Since(task.StartTime).Round(time.Second))
		}
	}

	if task.Error != nil {
		pterm.Println("Error:", pterm.Red(task.Error.Error()))
	}

	pterm.Println()
	pterm.DefaultSection.Println("Task Logs")

	// Display task events
	// In a real implementation, you would retrieve logs from a storage system
	pterm.Warning.Println("Log retrieval is not fully implemented yet")
	pterm.Info.Println("This would show the task logs with options to follow in real-time")

	if follow && task.Status == TaskRunning {
		pterm.Info.Println("Following logs in real-time...")
		// Here you would implement real-time log following
		// For now, we'll just wait a bit to simulate
		time.Sleep(3 * time.Second)
	}
}

// handleEvent processes events from the executor
func (m *Manager) handleEvent(event Event) {
	tracker := m.progressManager.GetTracker(event.TaskID)
	if tracker == nil {
		return
	}

	switch event.Type {
	case EventTaskStarted:
		tracker.UpdateMessage("Task started")

	case EventTaskProgress:
		if progress, ok := event.Data["progress"].(float64); ok {
			message := "Processing"
			if msg, ok := event.Data["message"].(string); ok {
				message = msg
			}
			tracker.UpdateProgress(progress, message)
		}

	case EventTaskCompleted:
		tracker.Complete("Task completed successfully")

	case EventTaskFailed:
		errorMsg := "Task failed"
		if errStr, ok := event.Data["error"].(string); ok {
			errorMsg = errStr
		}
		tracker.Fail(errorMsg, nil)
	}
}

// waitForTaskCompletion waits for a task to complete with a timeout
func (m *Manager) waitForTaskCompletion(taskID string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		m.lock.RLock()
		task, exists := m.tasks[taskID]
		m.lock.RUnlock()

		if !exists {
			return false
		}

		if task.Status == TaskCompleted {
			return true
		}

		if task.Status == TaskFailed || task.Status == TaskCancelled {
			return false
		}

		time.Sleep(100 * time.Millisecond)
	}

	return false
}

// Cleanup releases resources and terminates the task manager
func (m *Manager) Cleanup() {
	m.executor.Cleanup()
}
