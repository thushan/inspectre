package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
)

// UI display constants
const (
	// Channel buffer sizes
	EventChannelSize = 100

	// Wait timeouts
	ShutdownTimeout = 500 * time.Millisecond
	EventTimeout    = 50 * time.Millisecond

	// Progress bar styles
	ProgressBarWidth = 40

	// Color constants
	ColorSuccess = "#00aa00"
	ColorWarning = "#aaaa00"
	ColorError   = "#aa0000"
	ColorInfo    = "#0000aa"

	// Symbols
	SymbolCheck = "✓"
	SymbolCross = "✗"
	SymbolInfo  = "ℹ"

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

// DisplayOptions represents display options
type DisplayOptions struct {
	NoColor bool
	Format  string // "table", "json", "text"
	Quiet   bool
}

// UIEvent represents an event that needs to be displayed
type UIEvent struct {
	Type      string
	Message   string
	Data      interface{}
	Timestamp time.Time
}

// Display represents a UI display manager
type Display struct {
	options     DisplayOptions
	spinner     *pterm.SpinnerPrinter
	progressBar *pterm.ProgressbarPrinter

	// Event handling
	eventChan chan UIEvent
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Thread safety for UI components
	spinnerMu  sync.Mutex
	progressMu sync.Mutex
	closed     bool
	closedMu   sync.RWMutex
}

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

// Close shuts down the display manager
func (d *Display) Close() {
	d.closedMu.Lock()
	if d.closed {
		d.closedMu.Unlock()
		return
	}
	d.closed = true
	d.closedMu.Unlock()

	// Cancel context
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

	// Close event channel
	close(d.eventChan)

	// Wait for event handler to finish with timeout
	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Handler exited cleanly
	case <-time.After(ShutdownTimeout):
		// Timeout - log warning
		logger := logging.GetLogger()
		logger.Warning("Display manager shutdown timed out waiting for event handler")
	}
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

// PrintResultTable prints a table of results
func (d *Display) PrintResultTable(headers []string, rows [][]string) {
	tableData := map[string]interface{}{
		"headers": headers,
		"rows":    rows,
	}
	d.queueEvent(EventPrintTable, "", tableData)
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

// Internal handler methods for UI events
// --------------------------------------

func (d *Display) startSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	// Stop existing spinner if needed
	if d.spinner != nil {
		d.spinner.Stop()
	}

	// Create new spinner
	spinner, _ := pterm.DefaultSpinner.WithText(text).Start()
	d.spinner = spinner
}

func (d *Display) updateSpinnerTextInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.UpdateText(text)
	}
}

func (d *Display) stopSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Stop()
		d.spinner = nil
	}
}

func (d *Display) successSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Success(text)
		d.spinner = nil
	}
}

func (d *Display) failSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Fail(text)
		d.spinner = nil
	}
}

func (d *Display) warningSpinnerInternal(text string) {
	if d.options.Quiet {
		return
	}

	d.spinnerMu.Lock()
	defer d.spinnerMu.Unlock()

	if d.spinner != nil {
		d.spinner.Warning(text)
		d.spinner = nil
	}
}

func (d *Display) startProgressInternal(total int, title string) {
	if d.options.Quiet {
		return
	}

	d.progressMu.Lock()
	defer d.progressMu.Unlock()

	// Stop existing progress bar if needed
	if d.progressBar != nil {
		d.progressBar.Stop()
	}

	// Create new progress bar
	bar, _ := pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithRemoveWhenDone(true).
		Start()

	d.progressBar = bar
}

func (d *Display) updateProgressInternal(increment int) {
	if d.options.Quiet {
		return
	}

	d.progressMu.Lock()
	defer d.progressMu.Unlock()

	if d.progressBar != nil {
		d.progressBar.Add(increment)
	}
}

func (d *Display) stopProgressInternal() {
	if d.options.Quiet {
		return
	}

	d.progressMu.Lock()
	defer d.progressMu.Unlock()

	if d.progressBar != nil {
		d.progressBar.Stop()
		d.progressBar = nil
	}
}

func (d *Display) showSuccessInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Success.Println(message)
}

func (d *Display) showInfoInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Info.Println(message)
}

func (d *Display) showWarningInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Warning.Println(message)
}

func (d *Display) showErrorInternal(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Error.Println(message)
}

func (d *Display) showHeaderInternal(title string) {
	if d.options.Quiet {
		return
	}

	fmt.Println()
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).WithMargin(2).Println(title)
	fmt.Println()
}

func (d *Display) showTaskInfoInternal(task *types.Task) {
	if d.options.Quiet {
		return
	}

	// Create a table for task details
	tableData := pterm.TableData{
		{"Task ID", task.ID},
		{"Repository", task.Repository},
		{"Status", d.formatTaskStatus(task.Status)},
		{"Start Time", task.StartTime.Format(time.RFC3339)},
	}

	if !task.EndTime.IsZero() {
		tableData = append(tableData, []string{"End Time", task.EndTime.Format(time.RFC3339)})
		duration := task.EndTime.Sub(task.StartTime)
		tableData = append(tableData, []string{"Duration", formatDuration(duration)})
	}

	if task.Error != "" {
		tableData = append(tableData, []string{"Error", pterm.Red(task.Error)})
	}

	// Print the table
	pterm.DefaultTable.WithHasHeader(false).WithData(tableData).Render()
	fmt.Println()
}

func (d *Display) showResultsInternal(results []*analyser.Result) {
	if d.options.Quiet {
		return
	}

	formattedResults := logging.FormatResults(results, logging.FormatOptions{
		NoColor: d.options.NoColor,
		Format:  d.options.Format,
	})

	fmt.Println(formattedResults)
}

func (d *Display) printTableInternal(headers []string, rows [][]string) {
	if d.options.Quiet {
		return
	}

	tableData := make(pterm.TableData, 0, len(rows)+1)
	tableData = append(tableData, headers)

	for _, row := range rows {
		tableData = append(tableData, row)
	}

	pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()
	fmt.Println()
}

// formatTaskStatus formats a task status with colors
func (d *Display) formatTaskStatus(status string) string {
	if d.options.NoColor {
		return status
	}

	switch status {
	case "Completed":
		return pterm.FgGreen.Sprint(status)
	case "Running":
		return pterm.FgBlue.Sprint(status)
	case "Queued":
		return pterm.FgCyan.Sprint(status)
	case "Failed":
		return pterm.FgRed.Sprint(status)
	case "Cancelled":
		return pterm.FgYellow.Sprint(status)
	default:
		return status
	}
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	} else if d < time.Minute {
		return fmt.Sprintf("%.2f sec", d.Seconds())
	} else {
		minutes := d / time.Minute
		seconds := (d % time.Minute) / time.Second
		return fmt.Sprintf("%d min %d sec", minutes, seconds)
	}
}

// SpinnerAdapter adapts the internal display to the SpinnerProvider interface
type SpinnerAdapter struct {
	display *Display
}

func (s *SpinnerAdapter) UpdateText(text string) {
	s.display.UpdateSpinnerText(text)
}

func (s *SpinnerAdapter) Success(text string) {
	s.display.queueEvent(EventSpinnerSuccess, text, nil)
}

func (s *SpinnerAdapter) Fail(text string) {
	s.display.queueEvent(EventSpinnerFail, text, nil)
}

func (s *SpinnerAdapter) Warning(text string) {
	s.display.queueEvent(EventSpinnerWarning, text, nil)
}

func (s *SpinnerAdapter) Info(text string) {
	s.display.queueEvent(EventSpinnerStop, text, nil)
}
