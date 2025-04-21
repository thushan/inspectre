package ui

import (
	"fmt"
	"github.com/pterm/pterm"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
	"strings"
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

	// Split the results into lines and process each line for better formatting
	lines := strings.Split(formattedResults, "\n")

	// Print lines with proper spacing to ensure clean output
	for i, line := range lines {
		// Add extra spacing before headers for better readability
		if i > 0 && (strings.HasPrefix(line, "Results from") ||
			strings.HasPrefix(line, "Duration:") ||
			strings.HasPrefix(line, "Metrics:")) {
			fmt.Println()
		}

		fmt.Println(line)

		// Add extra spacing after headers for better readability
		if strings.HasSuffix(line, "-----") || strings.HasSuffix(line, "===") {
			fmt.Println()
		}
	}
}

func (d *Display) printTableInternal(headers []string, rows [][]string) {
	if d.options.Quiet {
		return
	}

	// Create a table with proper spacing and formatting
	tableData := make(pterm.TableData, 0, len(rows)+1)
	tableData = append(tableData, headers)

	// Add each row to the table
	for _, row := range rows {
		tableData = append(tableData, row)
	}

	// Configure the table with proper formatting
	table := pterm.DefaultTable.
		WithHasHeader().
		WithData(tableData)

	// Render the table with proper spacing
	tableStr, _ := table.Srender()
	fmt.Println(tableStr)
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

	// Print a blank line before task info for better spacing
	fmt.Println()

	// Create a table for task details with good spacing and format
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

	// Configure and render the table
	table := pterm.DefaultTable.
		WithHasHeader(false).
		WithData(tableData)

	tableStr, _ := table.Srender()
	fmt.Println(tableStr)

	// Print a blank line after task info for better spacing
	fmt.Println()
}
