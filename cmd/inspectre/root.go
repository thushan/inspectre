package commands

import (
	"sort"
	"time"

	"github.com/urfave/cli/v2"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:                 "inspectre",
		Usage:                "Code Analysis Tool to inspect repositories",
		Version:              "0.1.0",
		Compiled:             time.Now(),
		EnableBashCompletion: true,
		Authors: []*cli.Author{
			{
				Name: "Inspectre Team",
			},
		},
		Commands: []*cli.Command{},
	}

	app.Commands = append(app.Commands, CoreCommands()...)
	/*
		app.Commands = append(app.Commands, RepositoryCommands()...)
		app.Commands = append(app.Commands, PluginCommands()...)
		app.Commands = append(app.Commands, ConfigCommands()...)
		app.Commands = append(app.Commands, AnalysisCommands()...)
		app.Commands = append(app.Commands, SystemCommands()...)
	*/

	sortCommands(app.Commands)

	return app
}

func sortCommands(commands []*cli.Command) {
	sort.Sort(cli.CommandsByName(commands))
	for _, command := range commands {
		sort.Sort(cli.CommandsByName(command.Subcommands))
		sortCommands(command.Subcommands)
	}
}
