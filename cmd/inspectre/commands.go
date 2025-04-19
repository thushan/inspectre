package commands

import (
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

// ExtensionCommands returns the CLI commands for extension management
func ExtensionCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:  "plugin",
			Usage: "Manage analyser plugins",
			Subcommands: []*cli.Command{
				{
					Name:  "list",
					Usage: "List available plugins",
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:  "type",
							Usage: "Filter by plugin type (cli, python, golang)",
						},
						&cli.BoolFlag{
							Name:  "json",
							Usage: "Output in JSON format",
						},
					},
					Action: listPluginsAction,
				},
				{
					Name:      "enable",
					Usage:     "Enable a plugin",
					ArgsUsage: "<plugin-name>",
					Action:    enablePluginAction,
				},
				{
					Name:      "disable",
					Usage:     "Disable a plugin",
					ArgsUsage: "<plugin-name>",
					Action:    disablePluginAction,
				},
				{
					Name:      "info",
					Usage:     "Show plugin details",
					ArgsUsage: "<plugin-name>",
					Flags: []cli.Flag{
						&cli.BoolFlag{
							Name:  "json",
							Usage: "Output in JSON format",
						},
					},
					Action: pluginInfoAction,
				},
			},
		},
	}
}

// InsightsCommands returns CLI commands for generating insights
func InsightsCommands() []*cli.Command {
	return []*cli.Command{
		{
			Name:      "insights",
			Usage:     "Generate insights for a repository",
			ArgsUsage: "<repository-name-or-url>",
			Flags: []cli.Flag{
				&cli.StringFlag{
					Name:    "output",
					Aliases: []string{"o"},
					Usage:   "Output format (json, markdown, text)",
					Value:   "text",
				},
				&cli.StringFlag{
					Name:    "file",
					Aliases: []string{"f"},
					Usage:   "Output file path (default: stdout)",
				},
				&cli.StringFlag{
					Name:    "config",
					Aliases: []string{"c"},
					Usage:   "Custom path for configuration data",
				},
				&cli.BoolFlag{
					Name:  "force",
					Usage: "Force re-analyser even if insights exist",
				},
			},
			Action: insightsAction,
		},
	}
}
