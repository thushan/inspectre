package commands

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/thushan/inspectre/internal/core/config"
	appctx "github.com/thushan/inspectre/internal/core/context"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/repository"
	"github.com/thushan/inspectre/internal/core/task"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/extensions"
	"github.com/thushan/inspectre/internal/storage"
)

var (
	repoManager      *repository.Manager
	storageManager   *storage.Manager
	extensionManager *extensions.Manager
	taskManager      *task.Manager
	setupOnce        sync.Once
	appContext       *appctx.AppContext
	logger           *logging.Logger
)

// setupWithContext initializes the core components with the given context
func setupWithContext(ctx *appctx.AppContext) {
	appContext = ctx
	logger = logging.GetLogger()

	// Log that context initialization has occurred
	logger.Info("Application context initialized")
}

// setup initializes the core components
func setup(configPath string) error {
	var setupErr error

	setupOnce.Do(func() {
		// Log the start of setup
		logger.Info("Starting core components setup")

		// Load configuration
		appConfig, err := config.LoadConfig(configPath)
		if err != nil {
			logger.Error("Failed to load configuration: %v", err)
			setupErr = err
			return
		}
		logger.Info("Configuration loaded successfully")

		// Ensure required directories exist
		if err := config.EnsureDirectories(appConfig); err != nil {
			logger.Error("Failed to create required directories: %v", err)
			setupErr = err
			return
		}

		// Initialize repository manager
		repoManager, err = repository.NewManager(appConfig.RepositoriesFile, appContext)
		if err != nil {
			logger.Error("Failed to initialize repository manager: %v", err)
			setupErr = err
			return
		}
		logger.Info("Repository manager initialized")

		// Ensure data directory exists
		dataDir := filepath.Join("data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			logger.Error("Failed to create data directory: %v", err)
			setupErr = err
			return
		}

		// Initialize storage manager
		storageManager, err = storage.NewManager(storage.TypeFile, dataDir)
		if err != nil {
			logger.Error("Failed to initialize storage manager: %v", err)
			setupErr = err
			return
		}
		logger.Debug("Storage manager initialized")

		// Initialize extension manager
		extensionManager = extensions.NewManager(appConfig.PluginsDir)
		if err := extensionManager.LoadExtensionsFromConfig(""); err != nil {
			logger.Warning("Failed to load extensions: %v", err)
		}
		logger.Debug("Extension manager initialized")

		// Initialize task manager
		taskManager = task.NewManager(repoManager, storageManager, extensionManager, appContext)
		logger.Debug("Task manager initialized")

		// Log completion of setup
		logger.Debug("Core components setup completed successfully")
	})

	return setupErr
}

// updateManagersWithDisplay updates the managers with the given display
func updateManagersWithDisplay(display types.DisplayProvider) {
	if display == nil {
		logger.Warning("Attempted to update managers with nil display")
		return
	}

	logger.Debug("Updating managers with display provider")

	// Apply display to components that support it
	if repoManager != nil {
		repoManager.SetDisplay(display)
	} else {
		logger.Warning("Repository manager is nil when setting display")
	}

	if taskManager != nil {
		taskManager.SetDisplay(display)
	} else {
		logger.Warning("Task manager is nil when setting display")
	}

	logger.Debug("Managers updated with display provider")
}
