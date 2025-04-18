package ui

import (
	"fmt"
	"time"

	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/analysis"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
)

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

// Display represents a UI display manager
type Display struct {
	options     DisplayOptions
	progressBar *pterm.ProgressbarPrinter
	spinner     *pterm.SpinnerPrinter
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

	return &Display{
		options: options,
	}
}

// ShowLogo displays the Inspectre logo
func (d *Display) ShowLogo() {
	fmt.Println(`╔──────────────────────────────────────────────────────────────────────────╗
│  ██╗███╗   ██╗███████╗██████╗ ███████╗ ██████╗████████╗██████╗ ███████╗  │
│  ██║████╗  ██║██╔════╝██╔══██╗██╔════╝██╔════╝╚══██╔══╝██╔══██╗██╔════╝  │
│  ██║██╔██╗ ██║███████╗██████╔╝█████╗  ██║        ██║   ██████╔╝█████╗    │
│  ██║██║╚██╗██║╚════██║██╔═══╝ ██╔══╝  ██║        ██║   ██╔══██╗██╔══╝    │
│  ██║██║ ╚████║███████║██║     ███████╗╚██████╗   ██║   ██║  ██║███████╗  │
│  ╚═╝╚═╝  ╚═══╝╚══════╝╚═╝     ╚══════╝ ╚═════╝   ╚═╝   ╚═╝  ╚═╝╚══════╝  │
╚──────────────────────────────────────────────────────────────────────────╝`)
}

// ShowHeader displays a section header
func (d *Display) ShowHeader(title string) {
	if d.options.Quiet {
		return
	}

	fmt.Println()
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgBlue)).WithMargin(2).Println(title)
	fmt.Println()
}

// ShowTaskInfo displays task information
func (d *Display) ShowTaskInfo(task *types.Task) {
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

// ShowRepositoryInfo displays repository information
func (d *Display) ShowRepositoryInfo(repo *types.Repository) {
	if d.options.Quiet {
		return
	}

	// Determine auth type
	authType := "none"
	if repo.Auth.Token != "" {
		authType = "token"
	} else if repo.Auth.Username != "" {
		authType = "user/pass"
	}

	// Create a table for repository details
	tableData := pterm.TableData{
		{"Name", repo.Name},
		{"URL", repo.URL},
		{"Type", repo.Type},
		{"Authentication", authType},
	}

	// Print the table
	pterm.DefaultTable.WithHasHeader(false).WithData(tableData).Render()
	fmt.Println()
}

// StartProgressBar starts a progress bar
func (d *Display) StartProgressBar(total int, title string) {
	if d.options.Quiet {
		return
	}

	d.progressBar, _ = pterm.DefaultProgressbar.
		WithTotal(total).
		WithTitle(title).
		WithRemoveWhenDone(true).
		Start()
}

// UpdateProgressBar updates the progress bar
func (d *Display) UpdateProgressBar(increment int) {
	if d.options.Quiet || d.progressBar == nil {
		return
	}

	d.progressBar.Add(increment)
}

// StopProgressBar stops the progress bar
func (d *Display) StopProgressBar() {
	if d.options.Quiet || d.progressBar == nil {
		return
	}

	_, _ = d.progressBar.Stop()
	d.progressBar = nil
}

// StartSpinner starts a spinner
func (d *Display) StartSpinner(text string) types.SpinnerProvider {
	if d.options.Quiet {
		return &NullSpinner{}
	}

	spinner, _ := pterm.DefaultSpinner.
		WithText(text).
		Start()

	d.spinner = spinner
	return &SpinnerWrapper{spinner: spinner}
}

// UpdateSpinnerText updates the spinner text
func (d *Display) UpdateSpinnerText(text string) {
	if d.options.Quiet || d.spinner == nil {
		return
	}

	d.spinner.UpdateText(text)
}

// StopSpinner stops the spinner
func (d *Display) StopSpinner(text string) {
	if d.options.Quiet || d.spinner == nil {
		return
	}

	d.spinner.Success(text)
	d.spinner = nil
}

// SpinnerWrapper wraps pterm.SpinnerPrinter to implement SpinnerProvider
type SpinnerWrapper struct {
	spinner *pterm.SpinnerPrinter
}

// UpdateText updates the spinner text
func (s *SpinnerWrapper) UpdateText(text string) {
	if s.spinner != nil {
		s.spinner.UpdateText(text)
	}
}

// Success stops the spinner with a success message
func (s *SpinnerWrapper) Success(text string) {
	if s.spinner != nil {
		s.spinner.Success(text)
	}
}

// Fail stops the spinner with a failure message
func (s *SpinnerWrapper) Fail(text string) {
	if s.spinner != nil {
		s.spinner.Fail(text)
	}
}

// Warning stops the spinner with a warning message
func (s *SpinnerWrapper) Warning(text string) {
	if s.spinner != nil {
		s.spinner.Warning(text)
	}
}

// Info stops the spinner with an info message
func (s *SpinnerWrapper) Info(text string) {
	if s.spinner != nil {
		s.spinner.Info(text)
	}
}

// NullSpinner is a no-op implementation of SpinnerProvider
type NullSpinner struct{}

// UpdateText is a no-op
func (s *NullSpinner) UpdateText(text string) {}

// Success is a no-op
func (s *NullSpinner) Success(text string) {}

// Fail is a no-op
func (s *NullSpinner) Fail(text string) {}

// Warning is a no-op
func (s *NullSpinner) Warning(text string) {}

// Info is a no-op
func (s *NullSpinner) Info(text string) {}

// ShowResults displays analysis results
func (d *Display) ShowResults(results []*analysis.Result) {
	if d.options.Quiet {
		return
	}

	formattedResults := logging.FormatResults(results, logging.FormatOptions{
		NoColor: d.options.NoColor,
		Format:  d.options.Format,
	})

	fmt.Println(formattedResults)
}

// ShowSuccess displays a success message
func (d *Display) ShowSuccess(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Success.Println(message)
}

// ShowInfo displays an informational message
func (d *Display) ShowInfo(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Info.Println(message)
}

// ShowWarning displays a warning message
func (d *Display) ShowWarning(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Warning.Println(message)
}

// ShowError displays an error message
func (d *Display) ShowError(message string) {
	if d.options.Quiet {
		return
	}

	pterm.Error.Println(message)
}

// ShowBulletList displays a bullet list
func (d *Display) ShowBulletList(items []string) {
	if d.options.Quiet {
		return
	}

	for _, item := range items {
		pterm.DefaultBulletList.WithItems([]pterm.BulletListItem{
			{Level: 0, Text: item},
		}).Render()
	}
	fmt.Println()
}

// ShowTreeView displays a tree view of data
func (d *Display) ShowTreeView(title string, items map[string][]string) {
	if d.options.Quiet {
		return
	}

	leveledList := pterm.LeveledList{
		pterm.LeveledListItem{Level: 0, Text: title},
	}

	for category, subItems := range items {
		leveledList = append(leveledList, pterm.LeveledListItem{Level: 1, Text: category})

		for _, subItem := range subItems {
			leveledList = append(leveledList, pterm.LeveledListItem{Level: 2, Text: subItem})
		}
	}

	pterm.DefaultTree.WithRoot(pterm.NewTreeFromLeveledList(leveledList)).Render()
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
	case "Failed":
		return pterm.FgRed.Sprint(status)
	case "Cancelled":
		return pterm.FgYellow.Sprint(status)
	default:
		return status
	}
}

// PrintResultTable prints a table of results
func (d *Display) PrintResultTable(headers []string, rows [][]string) {
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

// AskForInput asks the user for input
func (d *Display) AskForInput(message, defaultValue string) string {
	if d.options.Quiet {
		return defaultValue
	}

	result, _ := pterm.DefaultInteractiveTextInput.
		WithDefaultText(message).
		WithDefaultValue(defaultValue).
		Show()

	return result
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
