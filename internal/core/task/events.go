package task

import (
	"fmt"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/ui/theme"
	"time"
)

// startEventHandlers starts background goroutines to process task results and UI events
func (m *Manager) startEventHandlers() {
	// Process task results
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.logger.Info("Task result processor started")

		for {
			select {
			case <-m.ctx.Done():
				m.logger.Info("Task result processor shutting down: context cancelled")
				return
			case result, ok := <-m.taskResults:
				if !ok {
					m.logger.Info("Task result processor shutting down: channel closed")
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
		m.logger.Info("UI event processor started")

		for {
			select {
			case <-m.ctx.Done():
				m.logger.Info("UI event processor shutting down: context cancelled")
				return
			case event, ok := <-m.uiEvents:
				if !ok {
					m.logger.Info("UI event processor shutting down: channel closed")
					return
				}
				m.handleUIEvent(event)
			}
		}
	}()
}

// processTaskResult handles task completion, updates task status, and stores results
func (m *Manager) processTaskResult(result TaskResult) {
	m.tasksMu.Lock()
	task, exists := m.tasks[result.TaskID]
	if !exists {
		m.tasksMu.Unlock()
		m.logger.Warning("Received result for unknown task %s", theme.ColourTaskId(result.TaskID))
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
		m.logger.TaskInfo(task.ID, "Storing analyser results...")

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

	taskId := theme.ColourTaskId(event.TaskID)

	switch event.EventType {
	case "started":
		m.display.StartSpinner(fmt.Sprintf("Starting task %s for repository %s", taskId, event.Message))

	case "cloning":
		// Force a pause before updating spinner to avoid overlap with task info display
		time.Sleep(100 * time.Millisecond)
		m.display.UpdateSpinnerText(fmt.Sprintf("Cloning repository %s", event.Message))

	case "analysing":
		// Add a short delay to ensure the message alignment
		time.Sleep(100 * time.Millisecond)
		m.display.UpdateSpinnerText(fmt.Sprintf("Running analysers for task %s", taskId))

	case "completed":
		// Ensure a clean completion message
		m.display.StopSpinner(fmt.Sprintf("Task %s completed successfully", taskId))

		// Add a brief delay before showing results
		time.Sleep(200 * time.Millisecond)

		if results, ok := event.Data.([]*analyser.Result); ok {
			m.display.ShowResults(results)
		}

	case "failed":
		m.display.ShowError(fmt.Sprintf("Task %s failed: %s", taskId, event.Message))

	case "cancelled":
		m.display.ShowWarning(fmt.Sprintf("Task %s was cancelled: %s", taskId, event.Message))

	case "progress":
		m.display.UpdateSpinnerText(fmt.Sprintf("%s | %s", taskId, event.Message))
	}
}

// sendUIEvent sends an event to the UI event channel (non-blocking)
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
