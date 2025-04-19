package ui

import (
	"context"
	"time"

	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
)

// UI display constants
const (
	EventChannelSize = 100
	ShutdownTimeout  = 500 * time.Millisecond
	EventTimeout     = 50 * time.Millisecond

	// Event types
	EventSpinnerStart   = "spinner_start"
	EventSpinnerUpdate  = "spinner_update"
	EventSpinnerStop    = "spinner_stop"
	EventSpinnerSuccess = "spinner_success"
	EventSpinnerFail    = "spinner_fail"
	EventSpinnerWarning = "spinner_warning"
	EventProgressStart  = "progress_start"
	EventProgressUpdate = "progress_update"
	EventProgressStop   = "progress_stop"
	EventShowSuccess    = "show_success"
	EventShowInfo       = "show_info"
	EventShowWarning    = "show_warning"
	EventShowError      = "show_error"
	EventShowHeader     = "show_header"
	EventShowResults    = "show_results"
	EventShowTaskInfo   = "show_task_info"
	EventPrintTable     = "print_table"
)

// NewDisplay creates a new display manager
func NewDisplay(options DisplayOptions) *Display {
	if options.Quiet {
		// Disable all output
		pterm.DisableOutput()
	}

	if options.NoColor {
		// Disable colors
		pterm.DisableColor()
	}

	// Create context for event handling
	ctx, cancel := context.WithCancel(context.Background())

	d := &Display{
		options:   options,
		eventChan: make(chan UIEvent, EventChannelSize),
		ctx:       ctx,
		cancel:    cancel,
		closed:    false,
	}

	// Start event handling goroutine
	d.startEventHandler()

	return d
}

// Close shuts down the display manager
func (d *Display) Close() {
	d.closedMu.Lock()
	if d.closed {
		d.closedMu.Unlock()
		return
	}
	d.closed = true
	d.closedMu.Unlock()

	// Get logger
	logger := logging.GetLogger()
	logger.Debug("Closing display manager")

	// Cancel context to signal shutdown to event handler
	d.cancel()

	// Stop any active spinner
	d.spinnerMu.Lock()
	if d.spinner != nil {
		d.spinner.Stop()
		d.spinner = nil
	}
	d.spinnerMu.Unlock()

	// Stop any active progress bar
	d.progressMu.Lock()
	if d.progressBar != nil {
		d.progressBar.Stop()
		d.progressBar = nil
	}
	d.progressMu.Unlock()

	// Wait for event handler to finish processing or timeout
	waitDone := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(waitDone)
	}()

	select {
	case <-waitDone:
		// Handler exited cleanly
		logger.Debug("Display event handler exited cleanly")
	case <-time.After(ShutdownTimeout):
		// Timeout - log warning
		logger.Warning("Display manager shutdown timed out waiting for event handler")
	}

	// Close event channel after event handler exits or times out
	// This prevents sends to a closed channel if the handler is still running
	close(d.eventChan)
	logger.Debug("Display manager closed")
}

// StartSpinner starts a spinner
func (d *Display) StartSpinner(text string) types.SpinnerProvider {
	d.queueEvent(EventSpinnerStart, text, nil)
	return &SpinnerAdapter{display: d}
}

// UpdateSpinnerText updates the spinner text
func (d *Display) UpdateSpinnerText(text string) {
	d.queueEvent(EventSpinnerUpdate, text, nil)
}

// StopSpinner stops the spinner
func (d *Display) StopSpinner(text string) {
	d.queueEvent(EventSpinnerStop, text, nil)
}

// ShowHeader displays a section header
func (d *Display) ShowHeader(title string) {
	d.queueEvent(EventShowHeader, title, nil)
}

// ShowTaskInfo displays task information
func (d *Display) ShowTaskInfo(task *types.Task) {
	d.queueEvent(EventShowTaskInfo, "", task)
}

// ShowResults displays analyser results
func (d *Display) ShowResults(results []*analyser.Result) {
	d.queueEvent(EventShowResults, "", results)
}

// ShowSuccess displays a success message
func (d *Display) ShowSuccess(message string) {
	d.queueEvent(EventShowSuccess, message, nil)
}

// ShowInfo displays an informational message
func (d *Display) ShowInfo(message string) {
	d.queueEvent(EventShowInfo, message, nil)
}

// ShowWarning displays a warning message
func (d *Display) ShowWarning(message string) {
	d.queueEvent(EventShowWarning, message, nil)
}

// ShowError displays an error message
func (d *Display) ShowError(message string) {
	d.queueEvent(EventShowError, message, nil)
}

// Confirm asks the user for confirmation
func (d *Display) Confirm(message string) bool {
	if d.options.Quiet {
		return true // Default to yes in quiet mode
	}

	result, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultText(message).
		WithDefaultValue(true).
		Show()

	return result
}
