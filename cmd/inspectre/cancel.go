package commands

import (
	"fmt"
	"github.com/thushan/inspectre/internal/core/ui/theme"
	"github.com/urfave/cli/v2"
)

func cancelAction(c *cli.Context) error {
	display := createDisplay(c)

	if err := setup(c.String("config")); err != nil {
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}

	taskID := c.Args().First()
	if taskID == "" {
		display.ShowError("Task ID is required")
		return fmt.Errorf("task ID is required")
	}

	// Start spinner
	spinner := display.StartSpinner(fmt.Sprintf("Cancelling task %s", theme.ColourTaskId(taskID)))

	// Get task
	task, err := taskManager.GetTask(taskID)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to get task: %v", err))
		return fmt.Errorf("failed to get task: %v", err)
	}

	// Show task info
	display.ShowTaskInfo(task)

	// Confirm cancellation
	if !c.Bool("yes") {
		spinner.Warning("Cancellation requires confirmation")
		confirmed := display.Confirm(fmt.Sprintf("Are you sure you want to cancel task %s?", theme.ColourTaskId(taskID)))
		if !confirmed {
			display.ShowInfo("Cancellation aborted")
			return nil
		}
	}

	// Cancel task
	spinner.UpdateText(fmt.Sprintf("Cancelling task %s...", theme.ColourTaskId(taskID)))

	if err := taskManager.CancelTask(taskID); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to cancel task: %v", err))
		return fmt.Errorf("failed to cancel task: %v", err)
	}

	spinner.Success(fmt.Sprintf("Task %s cancelled", theme.ColourTaskId(taskID)))
	return nil
}
