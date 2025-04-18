// internal/core/ui/display.go - Corrected version
package ui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/analysis"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
)

// UI display constants
const (
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
		eventChan: make(chan UIEvent, 100),
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
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		for {
			select {
			case <-d.ctx.Done():
				// Context cancelled, exit
				return

			case event, ok := <-d.eventChan:
				if !ok {
					// Channel closed
					return
				}

				// Process event
				d.handleEvent(event)
			}
		}
	}()
}

// handleEvent processes a UI event
func (d *Display) handleEvent(event UIEvent) {
	switch event.Type {
	case "spinner_start":
		d.startSpinnerInternal(event.Message)

	case "spinner_update":
		d.updateSpinnerTextInternal(event.Message)

	case "spinner_stop":
		d.stopSpinnerInternal(event.Message)

	case "spinner_success":
		d.successSpinnerInternal(event.Message)

	case "spinner_fail":
		d.failSpinnerInternal(event.Message)

	case "spinner_warning":
		d.warningSpinnerInternal(event.Message)

	case "progress_start":
		if progress, ok := event.Data.(map[string]interface{}); ok {
			total := int(progress["total"].(float64))
			title := progress["title"].(string)
			d.startProgressInternal(total, title)
		}

	case "progress_update":
		if value, ok := event.Data.(float64); ok {
			d.updateProgressInternal(int(value))
		}

	case "progress_stop":
		d.stopProgressInternal()

	case "show_success":
		d.showSuccessInternal(event.Message)

	case "show_info":
		d.showInfoInternal(event.Message)

	case "show_warning":
		d.showWarningInternal(event.Message)

	case "show_error":
		d.showErrorInternal(event.Message)

	case "show_header":
		d.showHeaderInternal(event.Message)

	case "show_results":
		if results, ok := event.Data.([]*analysis.Result); ok {
			d.showResultsInternal(results)
		}

	case "show_task_info":
		if task, ok := event.Data.(*types.Task); ok {
			d.showTaskInfoInternal(task)
		}

	case "print_table":
		if tableData, ok := event.Data.(map[string]interface{}); ok {
			headers := tableData["headers"].([]string)
			rows := tableData["rows"].([][]string)
			d.printTableInternal(headers, rows)
		}
	}
}

// queueEvent adds an event to the processing queue
func (d *Display) queueEvent(eventType, message string, data interface{}) {
	// Check if display is closed
	d.closedMu.RLock()
	isClosed := d.closed
	d.closedMu.RUnlock()

	if isClosed {
		return
	}

	// Create event
	event := UIEvent{
		Type:      eventType,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Try to send event to channel (non-blocking)
	select {
	case d.eventChan <- event:
		// Event sent
	case <-d.ctx.Done():
		// Context cancelled
	default:
		// Channel full, log this
		logger := logging.GetLogger()
		logger.Warning("UI event channel full, dropped event: %s", eventType)
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

	// Wait for event handler to finish
	d.wg.Wait()
}

// ShowLogo displays the Inspectre logo
func (d *Display) ShowLogo() {
	if d.options.Quiet {
		return
	}

	fmt.Println(`╔──────────────────────────────────────────────────────────────────────────╗
│  ██╗███╗   ██╗███████╗██████╗ ███████╗ ██████╗████████╗██████╗ ███████╗  │
│  ██║████╗  ██║██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝██╔══██╗██╔════╝  │
│  ██║██╔██╗ ██║███████╗██████╔╝█████╗  ██║        ██║   ██████╔╝█████╗    │
│  ██║██║╚██╗██║╚════██║██╔═══╝ ██╔══╝  ██║        ██║   ██╔══██╗██╔══╝    │
│  ██║██║ ╚████║███████║██║     ███████╗╚██████╗   ██║   ██║  ██║███████╗  │
│  ╚═╝╚═╝  ╚═══╝╚══════╝╚═╝     ╚══════╝ ╚═════╝   ╚═╝   ╚═╝  ╚═╝╚══════╝  │
╚──────────────────────────────────────────────────────────────────────────╝`)
}

// StartSpinner starts a spinner
func (d *Display) StartSpinner(text string) types.SpinnerProvider {
	d.queueEvent("spinner_start", text, nil)
	return &SpinnerAdapter{display: d}
}

// UpdateSpinnerText updates the spinner text
func (d *Display) UpdateSpinnerText(text string) {
	d.queueEvent("spinner_update", text, nil)
}

// StopSpinner stops the spinner
func (d *Display) StopSpinner(text string) {
	d.queueEvent("spinner_stop", text, nil)
}

// ShowHeader displays a section header
func (d *Display) ShowHeader(title string) {
	d.queueEvent("show_header", title, nil)
}

// ShowTaskInfo displays task information
func (d *Display) ShowTaskInfo(task *types.Task) {
	d.queueEvent("show_task_info", "", task)
}

// ShowResults displays analysis results
func (d *Display) ShowResults(results []*analysis.Result) {
	d.queueEvent("show_results", "", results)
}

// ShowSuccess displays a success message
func (d *Display) ShowSuccess(message string) {
	d.queueEvent("show_success", message, nil)
}

// ShowInfo displays an informational message
func (d *Display) ShowInfo(message string) {
	d.queueEvent("show_info", message, nil)
}

// ShowWarning displays a warning message
func (d *Display) ShowWarning(message string) {
	d.queueEvent("show_warning", message, nil)
}

// ShowError displays an error message
func (d *Display) ShowError(message string) {
	d.queueEvent("show_error", message, nil)
}

// PrintResultTable prints a table of results
func (d *Display) PrintResultTable(headers []string, rows [][]string) {
	tableData := map[string]interface{}{
		"headers": headers,
		"rows":    rows,
	}
	d.queueEvent("print_table", "", tableData)
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

func (d *Display) showResultsInternal(results []*analysis.Result) {
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
	s.display.queueEvent("spinner_success", text, nil)
}

func (s *SpinnerAdapter) Fail(text string) {
	s.display.queueEvent("spinner_fail", text, nil)
}

func (s *SpinnerAdapter) Warning(text string) {
	s.display.queueEvent("spinner_warning", text, nil)
}

func (s *SpinnerAdapter) Info(text string) {
	s.display.queueEvent("spinner_stop", text, nil)
}
