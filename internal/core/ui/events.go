package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/types"
)

// startEventHandler processes UI events in a single goroutine
func (d *Display) startEventHandler() {
	d.logger.Debug("Starting UI event handler")

	d.wg.Add(1)
	go func() {
		// Ensure the waitgroup is decremented on exit
		defer d.wg.Done()

		// Handle panics to prevent goroutine leaks
		defer func() {
			if r := recover(); r != nil {
				d.logger.Error("Recovered from panic in UI event handler: %v", r)
			}
		}()

		d.logger.Debug("UI event handler goroutine started")

		// Create mutex to synchronize UI updates
		var uiMutex sync.Mutex

		// Keep track of active spinners to prevent duplicate status updates
		activeSpinner := false

		for {
			select {
			case <-d.ctx.Done():
				// Context cancelled, exit
				d.logger.Debug("UI event handler stopping: context cancelled")

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

						// Lock UI during update
						uiMutex.Lock()
						d.handleEvent(event, &activeSpinner)
						uiMutex.Unlock()

						drainCount++
					case <-drainTimeout:
						d.logger.Debug("Event handler drain timeout after processing %d events", drainCount)
						break drainLoop
					}
				}

				d.logger.Debug("UI event handler successfully drained %d events before exit", drainCount)
				return

			case event, ok := <-d.eventChan:
				if !ok {
					// Channel closed
					d.logger.Debug("UI event handler stopping: event channel closed")
					return
				}

				// Lock UI during update to prevent interleaved output
				uiMutex.Lock()
				d.handleEvent(event, &activeSpinner)
				uiMutex.Unlock()
			}
		}
	}()
}

// queueEvent adds an event to the processing queue with timeout
func (d *Display) queueEvent(eventType, message string, data interface{}) {
	// Check if display is closed
	d.closedMu.RLock()
	isClosed := d.closed
	d.closedMu.RUnlock()

	if isClosed {
		d.logger.Warning("Attempted to queue event %s when display is closed", eventType)
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
		d.logger.Debug("Dropped UI event %s: display shutting down", eventType)
	case <-time.After(EventTimeout):
		// Timeout - log dropped event
		d.logger.Warning("UI event channel blocked, dropped event: %s", eventType)
	}
}

// handleEvent processes a UI event with spinner state tracking
func (d *Display) handleEvent(event UIEvent, activeSpinner *bool) {
	if d.options.Quiet {
		d.logger.Debug("Skipping UI event in quiet mode: type=%s", event.Type)
		return
	}

	// Process based on event type
	switch event.Type {
	case EventSpinnerStart:
		// Force a newline before starting a new spinner
		if *activeSpinner {
			fmt.Println()
		}
		d.startSpinnerInternal(event.Message)
		*activeSpinner = true

	case EventSpinnerUpdate:
		d.updateSpinnerTextInternal(event.Message)
		// Keep activeSpinner state unchanged

	case EventSpinnerStop:
		d.stopSpinnerInternal(event.Message)
		*activeSpinner = false

		// Add a small delay after stopping spinner
		time.Sleep(50 * time.Millisecond)

	case EventSpinnerSuccess:
		d.successSpinnerInternal(event.Message)
		*activeSpinner = false

		// Add a small delay after success message
		time.Sleep(50 * time.Millisecond)

	case EventSpinnerFail:
		d.failSpinnerInternal(event.Message)
		*activeSpinner = false

		// Add a small delay after fail message
		time.Sleep(50 * time.Millisecond)

	case EventSpinnerWarning:
		d.warningSpinnerInternal(event.Message)
		*activeSpinner = false

		// Add a small delay after warning message
		time.Sleep(50 * time.Millisecond)

	case EventProgressStart:
		// Ensure no spinner is active before starting progress
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(50 * time.Millisecond)
		}

		if progress, ok := event.Data.(map[string]interface{}); ok {
			total := int(progress["total"].(float64))
			title := progress["title"].(string)
			d.startProgressInternal(total, title)
		} else {
			d.logger.Warning("Invalid progress data received for EventProgressStart")
		}

	case EventProgressUpdate:
		if value, ok := event.Data.(float64); ok {
			d.updateProgressInternal(int(value))
		} else {
			d.logger.Warning("Invalid progress value received for EventProgressUpdate")
		}

	case EventProgressStop:
		d.stopProgressInternal()

	case EventShowSuccess:
		// Ensure no spinner is active before showing a message
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(50 * time.Millisecond)
		}
		d.showSuccessInternal(event.Message)

	case EventShowInfo:
		// Ensure no spinner is active before showing a message
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(50 * time.Millisecond)
		}
		d.showInfoInternal(event.Message)

	case EventShowWarning:
		// Ensure no spinner is active before showing a message
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(50 * time.Millisecond)
		}
		d.showWarningInternal(event.Message)

	case EventShowError:
		// Ensure no spinner is active before showing a message
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(50 * time.Millisecond)
		}
		d.showErrorInternal(event.Message)

	case EventShowHeader:
		// Ensure no spinner is active before showing a header
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(100 * time.Millisecond)
		}
		d.showHeaderInternal(event.Message)

	case EventShowResults:
		// Ensure no spinner is active before showing results
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(100 * time.Millisecond)
		}

		if results, ok := event.Data.([]*analyser.Result); ok {
			d.showResultsInternal(results)
		} else {
			d.logger.Warning("Invalid results data received for EventShowResults")
		}

	case EventShowTaskInfo:
		// Ensure no spinner is active before showing task info
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(100 * time.Millisecond)
		}

		if task, ok := event.Data.(*types.Task); ok {
			d.showTaskInfoInternal(task)
		} else {
			d.logger.Warning("Invalid task data received for EventShowTaskInfo")
		}

	case EventPrintTable:
		// Ensure no spinner is active before printing a table
		if *activeSpinner {
			d.stopSpinnerInternal("")
			*activeSpinner = false
			time.Sleep(100 * time.Millisecond)
		}

		if tableData, ok := event.Data.(map[string]interface{}); ok {
			headers, hok := tableData["headers"].([]string)
			rows, rok := tableData["rows"].([][]string)
			if hok && rok {
				d.printTableInternal(headers, rows)
			} else {
				d.logger.Warning("Invalid table data structure for EventPrintTable")
			}
		} else {
			d.logger.Warning("Invalid table data received for EventPrintTable")
		}

	default:
		d.logger.Warning("Unknown UI event type: %s", event.Type)
	}
}
