package commands

import (
	"sort"
	"time"

	"github.com/pterm/pterm"
	appctx "github.com/thushan/inspectre/internal/core/context"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/core/ui"
	"github.com/thushan/inspectre/internal/version"
	"github.com/urfave/cli/v2"
)

var (
	// Global flag for no color output
	noColorFlag = &cli.BoolFlag{
		Name:    "no-color",
		Usage:   "Disable colored output",
		Aliases: []string{"nc"},
		EnvVars: []string{"NO_COLOR", "INSPECTRE_NO_COLOR"},
	}

	// Global flag for quiet mode
	quietFlag = &cli.BoolFlag{
		Name:    "quiet",
		Usage:   "Quiet mode (minimal output)",
		Aliases: []string{"q"},
		EnvVars: []string{"QUIET", "INSPECTRE_QUIET"},
	}

	// Global flag for verbose mode
	verboseFlag = &cli.BoolFlag{
		Name:    "verbose",
		Usage:   "Verbose output",
		Aliases: []string{"vv"}, // Changed from "v" to "vv" to avoid conflict with version flag
		EnvVars: []string{"VERBOSE", "INSPECTRE_VERBOSE"},
	}

	// Global flag for output format
	outputFormatFlag = &cli.StringFlag{
		Name:    "output",
		Usage:   "Output format (json, table, text)",
		Aliases: []string{"o"},
		Value:   "table",
		EnvVars: []string{"OUTPUT_FORMAT", "INSPECTRE_OUTPUT_FORMAT"},
	}

	// Global flag for config file
	configFlag = &cli.StringFlag{
		Name:    "config",
		Usage:   "Configuration file path",
		Aliases: []string{"c"},
		EnvVars: []string{"CONFIG", "INSPECTRE_CONFIG"},
	}
)

// NewApp creates a new CLI application
func NewApp(appCtx *appctx.AppContext) *cli.App {
	// Initialize the shared setup once
	setupWithContext(appCtx)

	app := &cli.App{
		Name:                 version.Name,
		Usage:                version.Description,
		Version:              version.Version,
		Compiled:             time.Now(),
		EnableBashCompletion: true,
		Authors: []*cli.Author{
			{
				Name: version.Authors,
			},
		},
		Flags: []cli.Flag{
			noColorFlag,
			quietFlag,
			verboseFlag,
			outputFormatFlag,
			configFlag,
		},
		Before: func(c *cli.Context) error {
			// Configure global options
			if c.Bool("no-color") {
				pterm.DisableColor()
			}

			if c.Bool("verbose") {
				logging.GetLogger().SetLogLevel(logging.LevelDebug)
			} else if c.Bool("quiet") {
				logging.GetLogger().SetLogLevel(logging.LevelError)
			}

			return nil
		},
		Commands: []*cli.Command{},
	}

	app.Commands = append(app.Commands, CoreCommands()...)
	app.Commands = append(app.Commands, ExtensionCommands()...)
	app.Commands = append(app.Commands, InsightsCommands()...)

	sortCommands(app.Commands)

	return app
}

// sortCommands sorts commands in place by name
func sortCommands(commands []*cli.Command) {
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Name < commands[j].Name
	})

	// Sort subcommands recursively
	for _, command := range commands {
		if len(command.Subcommands) > 0 {
			sortCommands(command.Subcommands)
		}
	}
}

// getOutputOptions gets the display options from CLI context
func getOutputOptions(c *cli.Context) ui.DisplayOptions {
	return ui.DisplayOptions{
		NoColor: c.Bool("no-color"),
		Format:  c.String("output"),
		Quiet:   c.Bool("quiet"),
	}
}

// createDisplay creates a display manager from CLI context
func createDisplay(c *cli.Context) types.DisplayProvider {
	display := ui.NewDisplay(getOutputOptions(c))
	updateManagersWithDisplay(display)
	return display
}
