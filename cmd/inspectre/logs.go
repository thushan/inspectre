package commands

import (
	"fmt"
	"github.com/urfave/cli/v2"
	"io"
	"os"
	"strings"
	"time"
)

func logsAction(c *cli.Context) error {
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
	spinner := display.StartSpinner(fmt.Sprintf("Retrieving logs for task %s", taskID))

	// Get task
	task, err := taskManager.GetTask(taskID)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to get task: %v", err))
		return fmt.Errorf("failed to get task: %v", err)
	}

	// Show task info
	spinner.Success("Retrieved task information")
	display.ShowHeader("Task Information")
	display.ShowTaskInfo(task)

	// Show logs
	display.ShowHeader("Task Logs")

	reader, err := taskManager.GetLogReader(taskID)
	if err != nil {
		display.ShowError(fmt.Sprintf("Failed to get logs: %v", err))
		return fmt.Errorf("failed to get logs: %v", err)
	}
	defer reader.Close()

	follow := c.Bool("follow")
	if follow {
		// Follow logs (similar to tail -f)
		offsetFile, err := os.CreateTemp("", "inspectre-log-offset")
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to create temp file: %v", err))
			return fmt.Errorf("failed to create temp file: %v", err)
		}
		defer os.Remove(offsetFile.Name())
		defer offsetFile.Close()

		// Initial read
		data, err := io.ReadAll(reader)
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to read logs: %v", err))
			return fmt.Errorf("failed to read logs: %v", err)
		}

		// Display initial content
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Println(line)
			}
		}

		// Store offset
		offset := int64(len(data))
		_, err = fmt.Fprintf(offsetFile, "%d", offset)
		if err != nil {
			display.ShowWarning(fmt.Sprintf("Failed to store offset: %v", err))
		}

		// Start following
		display.ShowInfo("Following logs (press Ctrl+C to stop)...")

		// Poll for changes
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check if task is still running
				currentTask, err := taskManager.GetTask(taskID)
				if err != nil {
					display.ShowError(fmt.Sprintf("Failed to get task status: %v", err))
					return fmt.Errorf("failed to get task status: %v", err)
				}

				// Reopen log reader
				reader, err = taskManager.GetLogReader(taskID)
				if err != nil {
					display.ShowWarning(fmt.Sprintf("Failed to reopen log file: %v", err))
					time.Sleep(1 * time.Second)
					continue
				}

				// Seek to previous position
				offsetFile.Seek(0, 0)
				var storedOffset int64
				_, err = fmt.Fscanf(offsetFile, "%d", &storedOffset)
				if err != nil {
					display.ShowWarning(fmt.Sprintf("Failed to read offset: %v", err))
					storedOffset = 0
				}

				// Seek to stored offset
				_, err = reader.(*os.File).Seek(storedOffset, 0)
				if err != nil {
					display.ShowWarning(fmt.Sprintf("Failed to seek in log file: %v", err))
					reader.Close()
					continue
				}

				// Read new content
				newData, err := io.ReadAll(reader)
				reader.Close()
				if err != nil {
					display.ShowWarning(fmt.Sprintf("Failed to read logs: %v", err))
					continue
				}

				// Display new content
				if len(newData) > 0 {
					lines := strings.Split(string(newData), "\n")
					for _, line := range lines {
						if line != "" {
							fmt.Println(line)
						}
					}

					// Update offset
					offset = storedOffset + int64(len(newData))
					offsetFile.Truncate(0)
					offsetFile.Seek(0, 0)
					_, err = fmt.Fprintf(offsetFile, "%d", offset)
					if err != nil {
						display.ShowWarning(fmt.Sprintf("Failed to store offset: %v", err))
					}
				}

				// Exit if task is finished
				if currentTask.Status != "Running" && currentTask.Status != "Created" {
					display.ShowInfo(fmt.Sprintf("Task %s is %s, stopping log follow", taskID, currentTask.Status))
					return nil
				}
			case <-c.Context.Done():
				display.ShowInfo("Stopped following logs")
				return nil
			}
		}
	} else {
		// Just display logs once
		data, err := io.ReadAll(reader)
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to read logs: %v", err))
			return fmt.Errorf("failed to read logs: %v", err)
		}

		// Display content
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			if line != "" {
				fmt.Println(line)
			}
		}
	}

	return nil
}
