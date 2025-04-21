package commands

import (
	"context"
	"fmt"
	"github.com/thushan/inspectre/internal/core/ui/theme"
	"github.com/urfave/cli/v2"
	"io"
	"strings"
	"time"
)

func runAction(c *cli.Context) error {
	logger.Debug("Starting run action")
	display := createDisplay(c)
	logger.Debug("Display created for run action")

	if err := setup(c.String("config")); err != nil {
		logger.Error("Setup failed: %v", err)
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}
	logger.Debug("Setup completed successfully")

	// Verify task manager is initialized
	if taskManager == nil {
		logger.Error("Task manager is nil after setup")
		display.ShowError("Internal error: task manager not initialized")
		return fmt.Errorf("internal error: task manager not initialized")
	}

	// Set the display on task manager
	taskManager.SetDisplay(display)
	logger.Debug("Display set on task manager")

	repoURL := c.Args().First()
	if repoURL == "" {
		logger.Error("Repository name or URL is required")
		display.ShowError("Repository name or URL is required")
		return fmt.Errorf("repository name or URL is required")
	}
	logger.Debug("Repository URL: %s", theme.ColourRepository(repoURL))

	// Create a spinner for the task creation
	spinner := display.StartSpinner(fmt.Sprintf("Creating task for %s", theme.ColourRepository(repoURL)))
	logger.Debug("Spinner started for task creation")

	// Verify repository manager is initialized
	if repoManager == nil {
		spinner.Fail("Internal error: repository manager not initialized")
		logger.Error("Repository manager is nil when creating task")
		return fmt.Errorf("internal error: repository manager not initialized")
	}

	task, err := taskManager.CreateTask(repoURL)
	if err != nil {
		logger.Error("Failed to create task: %v", err)
		spinner.Fail(fmt.Sprintf("Failed to create task: %v", err))
		return fmt.Errorf("failed to create task: %v", err)
	}
	logger.Debug("Task created with ID: %s", theme.ColourTaskId(task.ID))

	// Update spinner text
	spinner.UpdateText(fmt.Sprintf("Starting task %s for %s", theme.ColourTaskId(task.ID), theme.ColourRepository(repoURL)))
	logger.Debug("Starting task %s", theme.ColourTaskId(task.ID))

	// Start the task
	if err := taskManager.StartTask(task.ID); err != nil {
		logger.Error("Failed to start task: %v", err)
		spinner.Fail(fmt.Sprintf("Failed to start task: %v", err))
		return fmt.Errorf("failed to start task: %v", err)
	}
	logger.Debug("Task %s started successfully", theme.ColourTaskId(task.ID))

	spinner.Success(fmt.Sprintf("Started task %s for repository %s", theme.ColourTaskId(task.ID), theme.ColourRepository(repoURL)))

	noWait := c.Bool("no-wait")
	if noWait {
		logger.Info("Not waiting for task completion (no-wait option used)")
		display.ShowInfo(fmt.Sprintf("Task is running in the background. Check status with: inspectre ps"))
		display.ShowInfo(fmt.Sprintf("View logs with: inspectre logs %s", theme.ColourTaskId(task.ID)))
		return nil
	}
	logger.Debug("Waiting for task %s to complete", theme.ColourTaskId(task.ID))

	// Show task info after a short delay to avoid UI conflicts
	time.Sleep(200 * time.Millisecond)
	display.ShowHeader("Task Information")
	display.ShowTaskInfo(task)

	waitSpinner := display.StartSpinner("Task is running, waiting for completion...")
	logger.Debug("Wait spinner started")

	// Use a context with timeout to avoid hanging indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Monitor task completion
	for {
		select {
		case <-ctx.Done():
			logger.Error("Timeout waiting for task to complete")
			waitSpinner.Fail("Timeout waiting for task to complete")
			return fmt.Errorf("timeout waiting for task to complete")
		case <-time.After(1 * time.Second):
			// Check task status
			currentTask, err := taskManager.GetTask(task.ID)
			if err != nil {
				logger.Error("Failed to get task status: %v", err)
				waitSpinner.Fail(fmt.Sprintf("Failed to get task status: %v", err))
				return fmt.Errorf("failed to get task status: %v", err)
			}

			if currentTask.Status == "Completed" || currentTask.Status == "Failed" || currentTask.Status == "Cancelled" {
				// Task has finished
				if currentTask.Status == "Failed" {
					logger.Error("Task failed: %s", currentTask.Error)
					waitSpinner.Fail(fmt.Sprintf("Task failed: %s", currentTask.Error))
					return fmt.Errorf("task failed: %s", currentTask.Error)
				} else if currentTask.Status == "Cancelled" {
					logger.Warning("Task was cancelled")
					waitSpinner.Warning("Task was cancelled")
					return fmt.Errorf("task was cancelled")
				}

				logger.Info("Task completed successfully")
				waitSpinner.Success("Task completed successfully")

				// Add a small delay to ensure clean transitions between UI elements
				time.Sleep(300 * time.Millisecond)

				// Show task logs with proper header
				display.ShowHeader("Task Logs")
				logger.Info("Showing task logs")

				// Try multiple times to get the log file since we may have just closed it
				var reader io.ReadCloser
				var logErr error

				for retry := 0; retry < 3; retry++ {
					reader, logErr = taskManager.GetLogReader(task.ID)
					if logErr == nil {
						break
					}
					logger.Warning("Retry %d for log file: %v", retry, logErr)
					time.Sleep(100 * time.Millisecond)
				}

				if logErr != nil {
					logger.Warning("Failed to get logs: %v", logErr)
					display.ShowWarning(fmt.Sprintf("Failed to get logs: %v", logErr))
				} else {
					defer reader.Close()
					// Copy log contents to stdout with a prefix
					data, err := io.ReadAll(reader)
					if err != nil {
						logger.Warning("Failed to read logs: %v", err)
						display.ShowWarning(fmt.Sprintf("Failed to read logs: %v", err))
					} else {
						lines := strings.Split(string(data), "\n")
						for _, line := range lines {
							if line != "" {
								fmt.Printf("  %s\n", line)
							}
						}
					}
				}

				logger.Info("Run action completed successfully")
				return nil
			}

			// Update spinner text with current status
			waitSpinner.UpdateText(fmt.Sprintf(
				"Task %s is %s (Running for %s)...",
				theme.ColourTaskId(task.ID),
				theme.ColourStatus(currentTask.Status),
				formatDuration(time.Since(currentTask.StartTime)),
			))
		}
	}
}

// formatDuration returns a friendly string representation of a duration
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1f seconds", d.Seconds())
	} else if d < time.Hour {
		m := int(d.Minutes())
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%d minutes %d seconds", m, s)
	} else {
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		s := int(d.Seconds()) % 60
		return fmt.Sprintf("%d hours %d minutes %d seconds", h, m, s)
	}
}
