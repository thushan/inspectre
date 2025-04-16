package commands

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

func CoreCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "run",
			Usage: "Run analysis on a repository",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "Output format (json, table, etc)",
				},
				&cli.StringFlag{
					Name:    "plugin",
					Aliases: []string{"p"},
					Usage:   "Plugin to use for analysis",
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
	}
}

func runAction(c *cli.Context) error {
	repoURL := c.Args().First()
	output := c.String("output")
	plugin := c.String("plugin")

	fmt.Printf("Running analysis on %s with output=%s and plugin=%s\n", repoURL, output, plugin)
	return nil
}

func psAction(c *cli.Context) error {
	showAll := c.Bool("all")
	showFailed := c.Bool("failed")
	jsonOutput := c.Bool("json")

	fmt.Printf("Listing tasks (all=%v, failed=%v, json=%v)\n", showAll, showFailed, jsonOutput)
	return nil
}

func logsAction(c *cli.Context) error {
	taskID := c.Args().First()
	if taskID == "" {
		return fmt.Errorf("task ID is required")
	}

	follow := c.Bool("follow")
	fmt.Printf("Showing logs for task %s (follow=%v)\n", taskID, follow)
	return nil
}
