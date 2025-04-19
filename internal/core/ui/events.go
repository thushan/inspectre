package ui

import (
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
	"time"
)

// startEventHandler processes UI events in a single goroutine
func (d *Display) startEventHandler() {
	// Create logger locally to avoid circular dependencies
	logger := logging.GetLogger()
	logger.Info("Starting UI event handler")

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		logger.Info("UI event handler goroutine started")

		for {
			select {
			case <-d.ctx.Done():
				// Context cancelled, exit
				logger.Info("UI event handler stopping: context cancelled")
				return

			case event, ok := <-d.eventChan:
				if !ok {
					// Channel closed
					logger.Info("UI event handler stopping: event channel closed")
					return
				}

				// Log the event
				logger.Debug("Processing UI event: type=%s, message=%s", event.Type, event.Message)

				// Process event
				d.handleEvent(event)
				logger.Debug("UI event processed: type=%s", event.Type)
			}
		}
	}()
}

// queueEvent adds an event to the processing queue with timeout
func (d *Display) queueEvent(eventType, message string, data interface{}) {
	// Create logger locally to avoid circular dependencies
	logger := logging.GetLogger()

	// Check if display is closed
	d.closedMu.RLock()
	isClosed := d.closed
	d.closedMu.RUnlock()

	if isClosed {
		logger.Warning("Attempted to queue event %s when display is closed", eventType)
		return
	}

	// Create event
	event := UIEvent{
		Type:      eventType,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}

	logger.Debug("Queueing UI event: type=%s, message=%s", eventType, message)

	// Try to send event to channel with timeout
	select {
	case d.eventChan <- event:
		// Event sent successfully
		logger.Debug("UI event queued successfully: type=%s", eventType)
	case <-d.ctx.Done():
		// Context cancelled
		logger.Warning("Failed to queue UI event %s: context cancelled", eventType)
	case <-time.After(EventTimeout):
		// Timeout - log dropped event
		logger.Warning("UI event channel blocked, dropped event: %s", eventType)
	}
}

// handleEvent processes a UI event
func (d *Display) handleEvent(event UIEvent) {
	// Create logger locally to avoid circular dependencies
	logger := logging.GetLogger()

	if d.options.Quiet {
		logger.Debug("Skipping UI event in quiet mode: type=%s", event.Type)
		return
	}

	// Process based on event type
	switch event.Type {
	case EventSpinnerStart:
		logger.Debug("Starting spinner: %s", event.Message)
		d.startSpinnerInternal(event.Message)

	case EventSpinnerUpdate:
		logger.Debug("Updating spinner: %s", event.Message)
		d.updateSpinnerTextInternal(event.Message)

	case EventSpinnerStop:
		logger.Debug("Stopping spinner: %s", event.Message)
		d.stopSpinnerInternal(event.Message)

	case EventSpinnerSuccess:
		logger.Debug("Spinner success: %s", event.Message)
		d.successSpinnerInternal(event.Message)

	case EventSpinnerFail:
		logger.Debug("Spinner fail: %s", event.Message)
		d.failSpinnerInternal(event.Message)

	case EventSpinnerWarning:
		logger.Debug("Spinner warning: %s", event.Message)
		d.warningSpinnerInternal(event.Message)

	case EventProgressStart:
		if progress, ok := event.Data.(map[string]interface{}); ok {
			total := int(progress["total"].(float64))
			title := progress["title"].(string)
			logger.Debug("Starting progress bar: title=%s, total=%d", title, total)
			d.startProgressInternal(total, title)
		} else {
			logger.Warning("Invalid progress data received for EventProgressStart")
		}

	case EventProgressUpdate:
		if value, ok := event.Data.(float64); ok {
			logger.Debug("Updating progress bar: value=%f", value)
			d.updateProgressInternal(int(value))
		} else {
			logger.Warning("Invalid progress value received for EventProgressUpdate")
		}

	case EventProgressStop:
		logger.Debug("Stopping progress bar")
		d.stopProgressInternal()

	case EventShowSuccess:
		logger.Debug("Showing success: %s", event.Message)
		d.showSuccessInternal(event.Message)

	case EventShowInfo:
		logger.Debug("Showing info: %s", event.Message)
		d.showInfoInternal(event.Message)

	case EventShowWarning:
		logger.Debug("Showing warning: %s", event.Message)
		d.showWarningInternal(event.Message)

	case EventShowError:
		logger.Debug("Showing error: %s", event.Message)
		d.showErrorInternal(event.Message)

	case EventShowHeader:
		logger.Debug("Showing header: %s", event.Message)
		d.showHeaderInternal(event.Message)

	case EventShowResults:
		logger.Debug("Showing results")
		if results, ok := event.Data.([]*analyser.Result); ok {
			d.showResultsInternal(results)
		} else {
			logger.Warning("Invalid results data received for EventShowResults")
		}

	case EventShowTaskInfo:
		logger.Debug("Showing task info")
		if task, ok := event.Data.(*types.Task); ok {
			d.showTaskInfoInternal(task)
		} else {
			logger.Warning("Invalid task data received for EventShowTaskInfo")
		}

	case EventPrintTable:
		logger.Debug("Printing table")
		if tableData, ok := event.Data.(map[string]interface{}); ok {
			headers, hok := tableData["headers"].([]string)
			rows, rok := tableData["rows"].([][]string)
			if hok && rok {
				d.printTableInternal(headers, rows)
			} else {
				logger.Warning("Invalid table data structure for EventPrintTable")
			}
		} else {
			logger.Warning("Invalid table data received for EventPrintTable")
		}

	default:
		logger.Warning("Unknown UI event type: %s", event.Type)
	}
}
