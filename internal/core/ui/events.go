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
	logger.Debug("Starting UI event handler")

	d.wg.Add(1)
	go func() {
		// Ensure the waitgroup is decremented on exit
		defer d.wg.Done()

		// Handle panics to prevent goroutine leaks
		defer func() {
			if r := recover(); r != nil {
				logger.Error("Recovered from panic in UI event handler: %v", r)
			}
		}()

		logger.Debug("UI event handler goroutine started")

		for {
			select {
			case <-d.ctx.Done():
				// Context cancelled, exit
				logger.Debug("UI event handler stopping: context cancelled")

				// Process any remaining events before exiting
				// This prevents events getting dropped during shutdown
				drainTimeout := time.After(100 * time.Millisecond)
				drainCount := 0

			drainLoop:
				for {
					select {
					case event, ok := <-d.eventChan:
						if !ok {
							break drainLoop
						}
						d.handleEvent(event)
						drainCount++
					case <-drainTimeout:
						logger.Debug("Event handler drain timeout after processing %d events", drainCount)
						break drainLoop
					}
				}

				logger.Debug("UI event handler successfully drained %d events before exit", drainCount)
				return

			case event, ok := <-d.eventChan:
				if !ok {
					// Channel closed
					logger.Debug("UI event handler stopping: event channel closed")
					return
				}

				// Process event
				d.handleEvent(event)
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

	// Try to send event to channel with timeout
	select {
	case d.eventChan <- event:
		// Event sent successfully
	case <-d.ctx.Done():
		// Context cancelled
		logger.Debug("Dropped UI event %s: display shutting down", eventType)
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
		d.startSpinnerInternal(event.Message)

	case EventSpinnerUpdate:
		d.updateSpinnerTextInternal(event.Message)

	case EventSpinnerStop:
		d.stopSpinnerInternal(event.Message)

	case EventSpinnerSuccess:
		d.successSpinnerInternal(event.Message)

	case EventSpinnerFail:
		d.failSpinnerInternal(event.Message)

	case EventSpinnerWarning:
		d.warningSpinnerInternal(event.Message)

	case EventProgressStart:
		if progress, ok := event.Data.(map[string]interface{}); ok {
			total := int(progress["total"].(float64))
			title := progress["title"].(string)
			d.startProgressInternal(total, title)
		} else {
			logger.Warning("Invalid progress data received for EventProgressStart")
		}

	case EventProgressUpdate:
		if value, ok := event.Data.(float64); ok {
			d.updateProgressInternal(int(value))
		} else {
			logger.Warning("Invalid progress value received for EventProgressUpdate")
		}

	case EventProgressStop:
		d.stopProgressInternal()

	case EventShowSuccess:
		d.showSuccessInternal(event.Message)

	case EventShowInfo:
		d.showInfoInternal(event.Message)

	case EventShowWarning:
		d.showWarningInternal(event.Message)

	case EventShowError:
		d.showErrorInternal(event.Message)

	case EventShowHeader:
		d.showHeaderInternal(event.Message)

	case EventShowResults:
		if results, ok := event.Data.([]*analyser.Result); ok {
			d.showResultsInternal(results)
		} else {
			logger.Warning("Invalid results data received for EventShowResults")
		}

	case EventShowTaskInfo:
		if task, ok := event.Data.(*types.Task); ok {
			d.showTaskInfoInternal(task)
		} else {
			logger.Warning("Invalid task data received for EventShowTaskInfo")
		}

	case EventPrintTable:
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
