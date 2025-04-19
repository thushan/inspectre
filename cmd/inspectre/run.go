package commands

import (
	"context"
	"fmt"
	"github.com/urfave/cli/v2"
	"io"
	"strings"
	"time"
)

func runAction(c *cli.Context) error {
	logger.Info("Starting run action")
	display := createDisplay(c)
	logger.Info("Display created for run action")

	if err := setup(c.String("config")); err != nil {
		logger.Error("Setup failed: %v", err)
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}
	logger.Info("Setup completed successfully")

	// Verify task manager is initialized
	if taskManager == nil {
		logger.Error("Task manager is nil after setup")
		display.ShowError("Internal error: task manager not initialized")
		return fmt.Errorf("internal error: task manager not initialized")
	}

	// Set the display on task manager
	taskManager.SetDisplay(display)
	logger.Info("Display set on task manager")

	repoURL := c.Args().First()
	if repoURL == "" {
		logger.Error("Repository name or URL is required")
		display.ShowError("Repository name or URL is required")
		return fmt.Errorf("repository name or URL is required")
	}
	logger.Info("Repository URL: %s", repoURL)

	// Create a spinner for the task creation
	spinner := display.StartSpinner(fmt.Sprintf("Creating task for %s", repoURL))
	logger.Info("Spinner started for task creation")

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
	logger.Info("Task created with ID: %s", task.ID)

	// Update spinner text
	spinner.UpdateText(fmt.Sprintf("Starting task %s for %s", task.ID, repoURL))
	logger.Info("Starting task %s", task.ID)

	// Start the task
	if err := taskManager.StartTask(task.ID); err != nil {
		logger.Error("Failed to start task: %v", err)
		spinner.Fail(fmt.Sprintf("Failed to start task: %v", err))
		return fmt.Errorf("failed to start task: %v", err)
	}
	logger.Info("Task %s started successfully", task.ID)

	spinner.Success(fmt.Sprintf("Started task %s for repository %s", task.ID, repoURL))

	noWait := c.Bool("no-wait")
	if noWait {
		logger.Info("Not waiting for task completion (no-wait option used)")
		display.ShowInfo(fmt.Sprintf("Task is running in the background. Check status with: inspectre ps"))
		display.ShowInfo(fmt.Sprintf("View logs with: inspectre logs %s", task.ID))
		return nil
	}
	logger.Info("Waiting for task %s to complete", task.ID)

	// Show task info
	display.ShowHeader("Task Information")
	display.ShowTaskInfo(task)

	waitSpinner := display.StartSpinner("Waiting for task to complete...")
	logger.Info("Wait spinner started")

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

				// Show task logs
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
								fmt.Println(line)
							}
						}
					}
				}

				logger.Info("Run action completed successfully")
				return nil
			}
		}
	}
}
