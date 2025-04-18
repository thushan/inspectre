// main.go - Improved version
package main

import (
	"context"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/joho/godotenv"
	commands "github.com/thushan/inspectre/cmd/inspectre"
	appctx "github.com/thushan/inspectre/internal/core/context"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/version"
	"github.com/urfave/cli/v2"
)

func main() {
	// Extend GOMAXPROCS to better utilize cores but avoid overwhelming the system
	// Use at least 4 for background tasks even on smaller machines
	minProcs := 4
	if runtime.NumCPU() > minProcs {
		minProcs = runtime.NumCPU()
	}
	runtime.GOMAXPROCS(minProcs)

	// Create application context with generous timeout for graceful shutdown
	appContext := appctx.NewAppContext(30 * time.Second)

	// Configure the logger first thing
	logger := logging.GetLogger()
	logger.SetLogLevel(logging.LevelInfo)

	// Register logger shutdown as highest priority
	appContext.AddShutdownHookWithPriority("logger.shutdown", appctx.PriorityHighest, func(ctx context.Context) error {
		logger.Info("Flushing logs and closing logger...")
		logger.Flush()
		logger.Close()
		return nil
	})

	// Load environment variables from .env file if it exists
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		logger.Warning("Error loading .env file: %v", err)
	}

	// Set up panic recovery
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Recovered from panic: %v", r)
			// Log stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			logger.Error("Stack trace: %s", string(buf[:n]))

			// Ensure orderly shutdown
			logger.Info("Attempting graceful shutdown after panic")
			appContext.Shutdown()

			os.Exit(1)
		}
	}()

	// Initialize the CLI app
	app := commands.NewApp(appContext)

	// Show logo before command runs
	app.Before = func(c *cli.Context) error {
		showLogo()
		return nil
	}

	// Add a global after hook to catch errors
	app.After = func(c *cli.Context) error {
		// This runs after command completes, even if there's an error
		if appContext != nil && c.Context != nil && c.Context.Err() != nil {
			logger.Info("Command context terminated: %v", c.Context.Err())
		}
		return nil
	}

	// Run the app and handle errors
	if err := app.Run(os.Args); err != nil {
		logger.Error("Application error: %v", err)

		// Ensure orderly shutdown even on error
		appContext.Shutdown()

		os.Exit(1)
	}

	// Initiate graceful shutdown
	logger.Info("Command completed, initiating graceful shutdown...")
	appContext.Shutdown()

	// Explicitly exit with success code
	os.Exit(0)
}

// showLogo displays the Inspectre logo
func showLogo() {
	vlog := log.New(log.Writer(), "", 0)
	version.PrintVersionInfo(true, vlog)
}
