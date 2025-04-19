package task

import (
	"fmt"
	"github.com/thushan/inspectre/internal/core/analyser"
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

	switch event.EventType {
	case "started":
		m.display.StartSpinner(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))

	case "cloning":
		m.display.UpdateSpinnerText(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))

	case "analysing":
		m.display.UpdateSpinnerText(fmt.Sprintf("[%s] %s", event.TaskID, event.Message))

	case "completed":
		m.display.StopSpinner(fmt.Sprintf("[%s] Analysis completed successfully", event.TaskID))
		if results, ok := event.Data.([]*analyser.Result); ok {
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
