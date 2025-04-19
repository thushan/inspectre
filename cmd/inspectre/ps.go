package commands

import (
	"encoding/json"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/urfave/cli/v2"
)

func psAction(c *cli.Context) error {
	display := createDisplay(c)

	if err := setup(c.String("config")); err != nil {
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}

	showAll := c.Bool("all")
	showFailed := c.Bool("failed")
	outputFormat := c.String("output")

	// Start spinner while retrieving tasks
	spinner := display.StartSpinner("Retrieving tasks...")

	tasks := taskManager.ListTasks(showAll, showFailed)

	if len(tasks) == 0 {
		spinner.Info("No tasks found")
		return nil
	}

	spinner.Success(fmt.Sprintf("Found %d tasks", len(tasks)))

	// Format output based on format
	if outputFormat == "json" {
		output, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to marshal tasks to JSON: %v", err))
			return fmt.Errorf("failed to marshal tasks to JSON: %v", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Create table data
	headers := []string{"TASK ID", "REPOSITORY", "STATUS", "START TIME", "END TIME"}
	rows := make([][]string, 0, len(tasks))

	for _, t := range tasks {
		endTime := "-"
		if !t.EndTime.IsZero() {
			endTime = t.EndTime.Format("2006-01-02 15:04:05")
		}

		// Format status with color
		status := t.Status
		switch t.Status {
		case "Completed":
			status = pterm.FgGreen.Sprint(t.Status)
		case "Failed":
			status = pterm.FgRed.Sprint(t.Status)
		case "Running":
			status = pterm.FgBlue.Sprint(t.Status)
		case "Cancelled":
			status = pterm.FgYellow.Sprint(t.Status)
		}

		rows = append(rows, []string{
			t.ID,
			t.Repository,
			status,
			t.StartTime.Format("2006-01-02 15:04:05"),
			endTime,
		})
	}

	// Print table
	display.PrintResultTable(headers, rows)
	return nil
}
