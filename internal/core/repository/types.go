package repository

import (
	"context"
	"github.com/go-git/go-git/v5"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
	"sync"
	"time"
)

// timeNow is a separate function to make testing easier
var timeNow = func() time.Time {
	return time.Now()
}

// CloneProgress wraps progress notifications from git operations
type CloneProgress struct {
	Message string
	Current int64
	Total   int64
}

// RepoCache caches repository metadata
type RepoCache struct {
	Repo      *git.Repository
	LastUsed  time.Time
	ClonePath string
}

// Manager implements the RepositoryManager interface
type Manager struct {
	configPath   string
	config       *Config
	mu           sync.RWMutex
	ctx          context.Context
	cancelFunc   context.CancelFunc
	logger       *logging.Logger
	display      types.DisplayProvider
	tempDir      string
	shutdownOnce sync.Once
	wg           sync.WaitGroup
	closed       bool
	closedMu     sync.RWMutex

	// Cache for repositories
	repoCache   map[string]*RepoCache
	repoCacheMu sync.RWMutex
}

// progressWriter is a helper to relay Git clone progress to the display
type progressWriter struct {
	ch chan<- CloneProgress
}

// Use the common Task type from types package
type Task = types.Task

// Use the common Repository type from types package
type Repository = types.Repository

// Use the common Auth type from types package
type Auth = types.Auth

// Use the common Config type from types package
type Config = types.Config

// RepositoryManager handles repository operations
type RepositoryManager interface {
	types.RepositoryManager
}
