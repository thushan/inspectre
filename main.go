package main

import (
	"fmt"
	"github.com/thushan/inspectre/internal/version"
	"log"
	"os"

	"github.com/joho/godotenv"
	commands "github.com/thushan/inspectre/cmd/inspectre"
	"github.com/urfave/cli/v2"
)

func main() {
	// Load environment variables from .env file if it exists
	_ = godotenv.Load()

	app := &cli.App{
		Name:     "inspectre",
		Usage:    "Repository analysis tool",
		Commands: append(commands.CoreCommands(), commands.ExtensionCommands()...),
		Version:  "0.1.0",
		Before: func(c *cli.Context) error {
			vlog := log.New(log.Writer(), "", 0)
			version.PrintVersionInfo(true, vlog)
			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
