package ui

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
	"time"
)

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

// PrintResultTable prints a table of results
func (d *Display) PrintResultTable(headers []string, rows [][]string) {
	tableData := map[string]interface{}{
		"headers": headers,
		"rows":    rows,
	}
	d.queueEvent(EventPrintTable, "", tableData)
}
