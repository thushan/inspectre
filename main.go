package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	commands "github.com/thushan/inspectre/cmd/inspectre"
	appctx "github.com/thushan/inspectre/internal/core/context"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/version"
	"github.com/urfave/cli/v2"
)

func main() {
	// Create application context for graceful shutdown
	appContext := appctx.NewAppContext(10 * time.Second)

	// Configure the logger
	logger := logging.GetLogger()
	logger.SetLogLevel(logging.LevelInfo)

	// Register a shutdown hook to flush logs
	appContext.AddShutdownHook(func(ctx context.Context) error {
		logger.Info("Flushing logs before exit...")
		logger.Flush()
		logger.Close()
		return nil
	})

	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		logger.Warning("Error loading .env file: %v", err)
	}

	// Initialize the CLI app
	app := commands.NewApp(appContext)
	app.Before = func(c *cli.Context) error {
		showLogo()
		return nil
	}

	// Run the app
	if err := app.Run(os.Args); err != nil {
		logger.Error("Application error: %v", err)
		os.Exit(1)
	}

	// Ensure orderly shutdown
	appContext.Shutdown()
}

// showLogo displays the Inspectre logo
func showLogo() {
	vlog := log.New(log.Writer(), "", 0)
	version.PrintVersionInfo(true, vlog)
}
