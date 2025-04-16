package commands

import (
	"github.com/thushan/inspectre/internal/version"
	"github.com/urfave/cli/v2"
	"sort"
)

func NewApp() *cli.App {
	app := &cli.App{
		Name:                 version.Name,
		Usage:                version.Description,
		Version:              version.Version,
		Compiled:             version.Date,
		EnableBashCompletion: true,
		Authors: []*cli.Author{
			{
				Name: version.Authors,
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
