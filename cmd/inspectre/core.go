package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"github.com/thushan/inspectre/internal/core/config"
	"github.com/thushan/inspectre/internal/core/repository"
	"github.com/thushan/inspectre/internal/core/task"
	"github.com/thushan/inspectre/internal/extensions"
	"github.com/thushan/inspectre/internal/storage"
	"github.com/urfave/cli/v2"
)

var (
	repoManager      *repository.Manager
	storageManager   *storage.Manager
	extensionManager *extensions.Manager
	taskManager      *task.Manager
	setupOnce        sync.Once
)

// setup initializes the core components
func setup(configPath string) error {
	var setupErr error

	setupOnce.Do(func() {
		appConfig, err := config.LoadConfig(configPath)
		if err != nil {
			setupErr = fmt.Errorf("failed to load config: %w", err)
			return
		}

		repoManager, setupErr = repository.NewManager(appConfig.RepositoriesFile)
		if setupErr != nil {
			return
		}

		dataDir := filepath.Join("data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			setupErr = fmt.Errorf("failed to create data directory: %w", err)
			return
		}

		storageManager, setupErr = storage.NewManager("file", dataDir)
		if setupErr != nil {
			return
		}

		extensionManager = extensions.NewManager(appConfig.PluginsDir)
		if err := extensionManager.LoadExtensionsFromConfig(""); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to load extensions: %v\n", err)
		}

		taskManager = task.NewManager(repoManager, storageManager, extensionManager)
	})

	return setupErr
}

func CoreCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:      "run",
			Usage:     "Run analysis on a repository",
			ArgsUsage: "<repository-name-or-url>",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "Output format (json, table, etc)",
					Value:   "table",
				},
				&cli.StringFlag{
					Name:    "config",
					Aliases: []string{"c"},
					Usage:   "Custom path for configuration data",
				},
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
				&cli.BoolFlag{
					Name:  "all",
					Usage: "Show all tasks including completed",
				},
				&cli.BoolFlag{
					Name:  "failed",
					Usage: "Show only failed tasks",
				},
				&cli.BoolFlag{
					Name:  "json",
					Usage: "Output in JSON format",
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
				&cli.BoolFlag{
					Name:  "json",
					Usage: "Output in JSON format",
				},
			},
			Action: reposAction,
		},
		{
			Name:  "query",
			Usage: "Query analysis results",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "sql",
					Aliases: []string{"s"},
					Usage:   "SQL query to execute",
					Value:   "SELECT * FROM metrics LIMIT 10",
				},
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "Output format (json, table)",
					Value:   "table",
				},
			},
			Action: queryAction,
		},
	}
}

func runAction(c *cli.Context) error {
	if err := setup(c.String("config")); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	repoURL := c.Args().First()
	if repoURL == "" {
		return fmt.Errorf("repository name or URL is required")
	}

	task, err := taskManager.CreateTask(repoURL)
	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	// Add no-wait flag to allow background execution
	noWait := c.Bool("no-wait")

	if err := taskManager.StartTask(task.ID); err != nil {
		return fmt.Errorf("failed to start task: %w", err)
	}

	fmt.Printf("Started analysis task %s for repository %s\n", task.ID, repoURL)

	if noWait {
		fmt.Printf("Task is running in the background. Check status with: inspectre ps\n")
		fmt.Printf("View logs with: inspectre logs %s\n", task.ID)
		return nil
	}

	fmt.Println("Waiting for task to complete...")

	// Monitor task status until completion
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentTask, err := taskManager.GetTask(task.ID)
			if err != nil {
				return fmt.Errorf("failed to get task status: %w", err)
			}

			if currentTask.Status == "Completed" || currentTask.Status == "Failed" {
				// Task has finished
				if currentTask.Status == "Failed" {
					return fmt.Errorf("task failed: %s", currentTask.Error)
				}

				fmt.Println("Task completed successfully")

				// Show task logs
				reader, err := taskManager.GetLogReader(task.ID)
				if err != nil {
					return fmt.Errorf("failed to get logs: %w", err)
				}
				defer reader.Close()

				// Copy log contents to stdout
				if _, err := io.Copy(os.Stdout, reader); err != nil {
					return fmt.Errorf("failed to read logs: %w", err)
				}

				return nil
			}
		}
	}
}

func psAction(c *cli.Context) error {
	if err := setup(c.String("config")); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	showAll := c.Bool("all")
	showFailed := c.Bool("failed")
	jsonOutput := c.Bool("json")

	tasks := taskManager.ListTasks(showAll, showFailed)

	if jsonOutput {
		output, err := json.MarshalIndent(tasks, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal tasks to JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	if len(tasks) == 0 {
		fmt.Println("No tasks found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TASK ID\tREPOSITORY\tSTATUS\tSTART TIME\tEND TIME")

	for _, t := range tasks {
		endTime := "-"
		if !t.EndTime.IsZero() {
			endTime = t.EndTime.Format("2006-01-02 15:04:05")
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			t.ID,
			t.Repository,
			t.Status,
			t.StartTime.Format("2006-01-02 15:04:05"),
			endTime,
		)
	}

	w.Flush()
	return nil
}

func logsAction(c *cli.Context) error {
	if err := setup(c.String("config")); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	taskID := c.Args().First()
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	reader, err := taskManager.GetLogReader(taskID)
	if err != nil {
		return fmt.Errorf("failed to get logs: %w", err)
	}
	defer reader.Close()

	// Copy log contents to stdout
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		return fmt.Errorf("failed to read logs: %w", err)
	}

	// TODO: Implement follow functionality

	return nil
}

func reposAction(c *cli.Context) error {
	if err := setup(c.String("config")); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	repos, err := repoManager.ListRepositories()
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	jsonOutput := c.Bool("json")

	if jsonOutput {
		output, err := json.MarshalIndent(repos, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal repositories to JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	if len(repos) == 0 {
		fmt.Println("No repositories configured")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tURL\tTYPE\tAUTH")

	for _, repo := range repos {
		authType := "none"
		if repo.Auth.Token != "" {
			authType = "token"
		} else if repo.Auth.Username != "" {
			authType = "user/pass"
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", repo.Name, repo.URL, repo.Type, authType)
	}

	w.Flush()
	return nil
}

func queryAction(c *cli.Context) error {
	if err := setup(c.String("config")); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	query := c.String("sql")
	outputFormat := c.String("output")

	results, err := storageManager.QueryMetrics(query)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if len(results) == 0 {
		fmt.Println("No results found")
		return nil
	}

	if outputFormat == "json" {
		output, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal results to JSON: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	if len(results) > 0 {
		var headers []string
		for key := range results[0] {
			headers = append(headers, key)
		}
		fmt.Fprintln(w, strings.Join(headers, "\t"))
	}

	for _, row := range results {
		var values []string
		for key := range results[0] { // Use first row's keys to maintain order
			val := row[key]
			values = append(values, fmt.Sprintf("%v", val))
		}
		fmt.Fprintln(w, strings.Join(values, "\t"))
	}

	w.Flush()
	return nil
}
