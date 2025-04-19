package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/urfave/cli/v2"
)

func CoreCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:      "run",
			Usage:     "Run analyser on a repository",
			ArgsUsage: "<repository-name-or-url>",
			Flags: []cli.Flag{
				outputFormatFlag,
				configFlag,
				&cli.BoolFlag{
					Name:    "no-wait",
					Aliases: []string{"n"},
					Usage:   "Don't wait for task to complete (run in background)",
					Value:   false,
				},
			},
			Action: runAction,
		},
		{
			Name:  "ps",
			Usage: "List running tasks",
			Flags: []cli.Flag{
				outputFormatFlag,
				&cli.BoolFlag{
					Name:  "all",
					Usage: "Show all tasks including completed",
				},
				&cli.BoolFlag{
					Name:  "failed",
					Usage: "Show only failed tasks",
				},
			},
			Action: psAction,
		},
		{
			Name:      "logs",
			Usage:     "Show logs for a task",
			ArgsUsage: "<task-id>",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "follow",
					Aliases: []string{"f"},
					Usage:   "Follow logs as they are generated",
				},
			},
			Action: logsAction,
		},
		{
			Name:  "repos",
			Usage: "List configured repositories",
			Flags: []cli.Flag{
				outputFormatFlag,
			},
			Action: reposAction,
		},
		{
			Name:  "query",
			Usage: "Query analyser results",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "sql",
					Aliases: []string{"s"},
					Usage:   "SQL query to execute",
					Value:   "SELECT * FROM metrics LIMIT 10",
				},
				outputFormatFlag,
			},
			Action: queryAction,
		},
		{
			Name:      "cancel",
			Usage:     "Cancel a running task",
			ArgsUsage: "<task-id>",
			Flags: []cli.Flag{
				&cli.BoolFlag{
					Name:    "yes",
					Aliases: []string{"y"},
					Usage:   "Skip confirmation",
					Value:   false,
				},
			},
			Action: cancelAction,
		},
	}
}
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

func reposAction(c *cli.Context) error {
	display := createDisplay(c)

	if err := setup(c.String("config")); err != nil {
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}

	outputFormat := c.String("output")

	// Start spinner while retrieving repositories
	spinner := display.StartSpinner("Retrieving repositories...")

	repos, err := repoManager.ListRepositories()
	if err != nil {
		spinner.Fail(fmt.Sprintf("Failed to list repositories: %v", err))
		return fmt.Errorf("failed to list repositories: %v", err)
	}

	if len(repos) == 0 {
		spinner.Info("No repositories configured")
		return nil
	}

	spinner.Success(fmt.Sprintf("Found %d repositories", len(repos)))

	// Format output based on format
	if outputFormat == "json" {
		output, err := json.MarshalIndent(repos, "", "  ")
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to marshal repositories to JSON: %v", err))
			return fmt.Errorf("failed to marshal repositories to JSON: %v", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Create table data
	headers := []string{"NAME", "URL", "TYPE", "AUTH"}
	rows := make([][]string, 0, len(repos))

	for _, repo := range repos {
		authType := "none"
		if repo.Auth.Token != "" {
			authType = "token"
		} else if repo.Auth.Username != "" {
			authType = "user/pass"
		}

		rows = append(rows, []string{
			repo.Name,
			repo.URL,
			repo.Type,
			authType,
		})
	}

	// Print table
	display.PrintResultTable(headers, rows)
	return nil
}

func queryAction(c *cli.Context) error {
	display := createDisplay(c)

	if err := setup(c.String("config")); err != nil {
		display.ShowError(fmt.Sprintf("Setup failed: %v", err))
		return fmt.Errorf("setup failed: %v", err)
	}

	query := c.String("sql")
	outputFormat := c.String("output")

	// Start spinner while executing query
	spinner := display.StartSpinner(fmt.Sprintf("Executing query: %s", query))

	results, err := storageManager.QueryMetrics(query)
	if err != nil {
		spinner.Fail(fmt.Sprintf("Query failed: %v", err))
		return fmt.Errorf("query failed: %v", err)
	}

	if len(results) == 0 {
		spinner.Info("No results found")
		return nil
	}

	spinner.Success(fmt.Sprintf("Query returned %d results", len(results)))

	// Format output based on format
	if outputFormat == "json" {
		output, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			display.ShowError(fmt.Sprintf("Failed to marshal results to JSON: %v", err))
			return fmt.Errorf("failed to marshal results to JSON: %v", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Create table data
	var headers []string
	if len(results) > 0 {
		for key := range results[0] {
			headers = append(headers, key)
		}
	}

	rows := make([][]string, 0, len(results))
	for _, row := range results {
		var values []string
		for _, key := range headers {
			val := row[key]
			values = append(values, fmt.Sprintf("%v", val))
		}
		rows = append(rows, values)
	}

	// Print table
	display.PrintResultTable(headers, rows)
	return nil
}

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
	spinner := display.StartSpinner(fmt.Sprintf("Cancelling task %s", taskID))

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
		confirmed := display.Confirm(fmt.Sprintf("Are you sure you want to cancel task %s?", taskID))
		if !confirmed {
			display.ShowInfo("Cancellation aborted")
			return nil
		}
	}

	// Cancel task
	spinner.UpdateText(fmt.Sprintf("Cancelling task %s...", taskID))

	if err := taskManager.CancelTask(taskID); err != nil {
		spinner.Fail(fmt.Sprintf("Failed to cancel task: %v", err))
		return fmt.Errorf("failed to cancel task: %v", err)
	}

	spinner.Success(fmt.Sprintf("Task %s cancelled", taskID))
	return nil
}
