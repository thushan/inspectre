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
	app.Commands = append(app.Commands, ExtensionCommands()...)
	app.Commands = append(app.Commands, InsightsCommands()...)

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
