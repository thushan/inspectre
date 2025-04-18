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
}

// setup initializes the core components
func setup(configPath string) error {
	var setupErr error

	setupOnce.Do(func() {
		// Load configuration
		appConfig, err := config.LoadConfig(configPath)
		if err != nil {
			setupErr = err
			return
		}

		// Initialize repository manager
		repoManager, err = repository.NewManager(appConfig.RepositoriesFile, appContext)
		if err != nil {
			setupErr = err
			return
		}

		// Ensure data directory exists
		dataDir := filepath.Join("data")
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			setupErr = err
			return
		}

		// Initialize storage manager
		storageManager, err = storage.NewManager("file", dataDir)
		if err != nil {
			setupErr = err
			return
		}

		// Initialize extension manager
		extensionManager = extensions.NewManager(appConfig.PluginsDir)
		if err := extensionManager.LoadExtensionsFromConfig(""); err != nil {
			logger.Warning("Failed to load extensions: %v", err)
		}

		// Initialize task manager
		taskManager = task.NewManager(repoManager, storageManager, extensionManager, appContext)
	})

	return setupErr
}

// updateManagersWithDisplay updates the managers with the given display
func updateManagersWithDisplay(display types.DisplayProvider) {
	// Apply display to components that support it
	if repoManager != nil {
		repoManager.SetDisplay(display)
	}

	if taskManager != nil {
		taskManager.SetDisplay(display)
	}
}
