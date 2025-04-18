package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/lithammer/shortuuid/v4"
	appctx "github.com/thushan/inspectre/internal/core/context"
	"github.com/thushan/inspectre/internal/core/logging"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/core/utils"
)

var (
	ErrRepositoryNotFound    = errors.New("repository not found")
	ErrInvalidConfiguration  = errors.New("invalid configuration")
	ErrCloneFailure          = errors.New("failed to clone repository")
	ErrUnsupportedRepoType   = errors.New("unsupported repository type")
	ErrMissingAuthentication = errors.New("missing authentication details")

	// UUID alphabet for task IDs (hex only for better readability)
	UUIDAlphabet = "0123456789abcdef"

	// Default task directory paths
	DefaultTempDir = filepath.Join(os.TempDir(), "inspectre")
)

// Manager implements the RepositoryManager interface
type Manager struct {
	configPath string
	config     *Config
	mu         sync.RWMutex
	ctx        *appctx.AppContext
	logger     *logging.Logger
	display    types.DisplayProvider
	tempDir    string
}

// NewManager creates a new repository manager
func NewManager(configPath string, appCtx *appctx.AppContext) (*Manager, error) {
	if configPath == "" {
		// Default config path
		configPath = "configs/repositories.json"
	}

	m := &Manager{
		configPath: configPath,
		ctx:        appCtx,
		logger:     logging.GetLogger(),
		tempDir:    DefaultTempDir,
	}

	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	// Register shutdown hook
	if appCtx != nil {
		appCtx.AddShutdownHook(m.shutdown)
	}

	return m, nil
}

// SetDisplay sets the display manager
func (m *Manager) SetDisplay(display types.DisplayProvider) {
	m.display = display
}

// SetTempDir sets the temporary directory for repositories
func (m *Manager) SetTempDir(dir string) {
	m.tempDir = dir
}

// loadConfig reads and parses the repositories configuration file
func (m *Manager) loadConfig() error {
	file, err := os.Open(m.configPath)
	if os.IsNotExist(err) {
		m.logger.Warning("Config file not found at %s, creating empty configuration", m.configPath)

		// Create an empty config
		m.config = &Config{
			Version:      1,
			Repositories: []Repository{},
		}

		// Create directory if needed
		if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}

		// Save empty config
		return m.SaveConfig()
	} else if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Process environment variables in the config
	processedData := m.processEnvVars(string(data))

	var config Config
	if err := json.Unmarshal([]byte(processedData), &config); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidConfiguration, err)
	}

	m.config = &config
	return nil
}

// processEnvVars replaces environment variables in the format ${VAR_NAME}
func (m *Manager) processEnvVars(input string) string {
	re := regexp.MustCompile(`\${([^}]+)}`)
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name from ${VAR_NAME}
		varName := match[2 : len(match)-1]
		// Get environment variable value
		value := os.Getenv(varName)
		if value == "" {
			// If not found, keep the original placeholder
			return match
		}
		return value
	})
	return result
}

// SaveConfig saves the current configuration to disk
func (m *Manager) SaveConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetRepository finds a repository by name or URL
func (m *Manager) GetRepository(nameOrURL string) (*Repository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, repo := range m.config.Repositories {
		if repo.Name == nameOrURL || repo.URL == nameOrURL {
			// Return a copy to prevent modification of config
			repoCopy := repo
			return &repoCopy, nil
		}
	}
	return nil, ErrRepositoryNotFound
}

// AddRepository adds a new repository to the configuration
func (m *Manager) AddRepository(repo Repository) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if repository already exists
	for i, existing := range m.config.Repositories {
		if existing.Name == repo.Name {
			// Update existing repository
			m.config.Repositories[i] = repo
			return m.SaveConfig()
		}
	}

	// Add new repository
	m.config.Repositories = append(m.config.Repositories, repo)
	return m.SaveConfig()
}

// RemoveRepository removes a repository from the configuration
func (m *Manager) RemoveRepository(nameOrURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, repo := range m.config.Repositories {
		if repo.Name == nameOrURL || repo.URL == nameOrURL {
			// Remove repository
			m.config.Repositories = append(m.config.Repositories[:i], m.config.Repositories[i+1:]...)
			return m.SaveConfig()
		}
	}

	return ErrRepositoryNotFound
}

// ListRepositories returns all configured repositories
func (m *Manager) ListRepositories() ([]Repository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy to prevent modification of config
	repos := make([]Repository, len(m.config.Repositories))
	copy(repos, m.config.Repositories)

	return repos, nil
}

// Clone clones a repository to the specified target directory
func (m *Manager) Clone(repo *Repository, targetDir string) error {
	// Start a spinner if display is available
	if m.display != nil {
		m.display.StartSpinner(fmt.Sprintf("Cloning %s", repo.URL))
		defer m.display.StopSpinner(fmt.Sprintf("Clone of %s completed", repo.URL))
	}

	// Check if directory exists
	fi, err := os.Stat(targetDir)
	if err == nil {
		if !fi.IsDir() {
			return fmt.Errorf("target exists but is not a directory: %s", targetDir)
		}

		// Directory exists, check if it's empty
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			return fmt.Errorf("failed to read target directory: %w", err)
		}

		if len(entries) > 0 {
			// Not empty, try to clean it safely
			m.logger.Info("Cleaning existing directory: %s", targetDir)
			if err := utils.SafeRemoveAll(targetDir); err != nil {
				return fmt.Errorf("failed to clean target directory: %w", err)
			}
		}
	} else if !os.IsNotExist(err) {
		// Some error other than "not exists"
		return fmt.Errorf("failed to check target directory: %w", err)
	}

	// Ensure the directory exists (it was either removed or never existed)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	auth, err := m.getAuthMethod(repo)
	if err != nil {
		return err
	}

	cloneOpts := &git.CloneOptions{
		URL: repo.URL,
	}

	if auth != nil {
		cloneOpts.Auth = auth
	}

	// Use progress writer if available
	if m.display != nil {
		progressCh := make(chan string)
		cloneOpts.Progress = progressWriter{ch: progressCh}

		// Update spinner with progress
		go func() {
			for msg := range progressCh {
				m.display.UpdateSpinnerText(msg)
			}
		}()
	}

	m.logger.Info("Cloning repository %s to %s", repo.URL, targetDir)
	_, err = git.PlainClone(targetDir, false, cloneOpts)
	if err != nil {
		// Clean up the directory if cloning fails, but don't worry too much about errors
		_ = utils.SafeRemoveAll(targetDir)
		return fmt.Errorf("%w: %v", ErrCloneFailure, err)
	}

	m.logger.Info("Successfully cloned repository %s", repo.URL)
	return nil
}

// CleanUp removes the temporary directory
func (m *Manager) CleanUp(task *Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}

	m.logger.Info("Cleaning up task directory: %s", task.BaseDir)

	// Try to clean up with retries (but without sleeps - use context pattern)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return utils.RetryWithContext(ctx, 3, func() error {
		return os.RemoveAll(task.BaseDir)
	})
}

// shutdown performs a graceful shutdown
func (m *Manager) shutdown(ctx context.Context) error {
	m.logger.Info("Shutting down repository manager...")
	return nil
}

// getAuthMethod determines the appropriate authentication method
func (m *Manager) getAuthMethod(repo *Repository) (transport.AuthMethod, error) {
	switch strings.ToLower(repo.Type) {
	case "github", "gitlab":
		if repo.Auth.Token != "" {
			return &http.BasicAuth{
				Username: "x-oauth-basic", // For GitHub, the username doesn't matter
				Password: repo.Auth.Token,
			}, nil
		} else if repo.Auth.Username != "" && repo.Auth.Password != "" {
			return &http.BasicAuth{
				Username: repo.Auth.Username,
				Password: repo.Auth.Password,
			}, nil
		}
		// This is for public repositories
		return nil, nil
	case "bitbucket":
		if repo.Auth.Username != "" && repo.Auth.Password != "" {
			return &http.BasicAuth{
				Username: repo.Auth.Username,
				Password: repo.Auth.Password,
			}, nil
		} else if repo.Auth.Token != "" {
			return &http.BasicAuth{
				Username: "x-token-auth",
				Password: repo.Auth.Token,
			}, nil
		}
		// This is for public repositories
		return nil, nil
	default:
		// For generic Git repositories, try to use basic auth if provided
		if repo.Auth.Username != "" && (repo.Auth.Password != "" || repo.Auth.Token != "") {
			password := repo.Auth.Password
			if password == "" {
				password = repo.Auth.Token
			}
			return &http.BasicAuth{
				Username: repo.Auth.Username,
				Password: password,
			}, nil
		}
		// For public repositories or SSH (which is handled by go-git)
		return nil, nil
	}
}

// CreateTask creates a new analysis task
func (m *Manager) CreateTask(nameOrURL string) (*Task, error) {
	repo, err := m.GetRepository(nameOrURL)
	if err != nil {
		// Handle case when URL is provided directly
		if isURL(nameOrURL) {
			// Create a temporary repository entry
			repo = &Repository{
				Name: extractRepoName(nameOrURL),
				URL:  nameOrURL,
				Type: GuessRepoType(nameOrURL),
				Auth: Auth{}, // No auth provided
			}
		} else {
			return nil, err
		}
	}

	// Generate task ID
	taskID := shortuuid.NewWithAlphabet(UUIDAlphabet)

	// Create base task directory
	baseDir := filepath.Join(m.tempDir, taskID)

	// Create separate subdirectories
	repoDir := filepath.Join(baseDir, "repo")     // Repository clone directory
	assetsDir := filepath.Join(baseDir, "assets") // Assets and plugin data
	logFile := filepath.Join(baseDir, "task.log") // Main log file

	m.logger.Info("Creating task %s for repository %s", taskID, repo.URL)

	return &Task{
		ID:         taskID,
		Repository: repo.URL,
		Status:     "Created",
		StartTime:  timeNow(),
		BaseDir:    baseDir,
		RepoDir:    repoDir,
		AssetsDir:  assetsDir,
		LogFile:    logFile,
	}, nil
}

// timeNow is a separate function to make testing easier
var timeNow = func() time.Time {
	return time.Now()
}

// GuessRepoType tries to determine the repository type from the URL
func GuessRepoType(url string) string {
	if strings.Contains(url, "github.com") {
		return "github"
	} else if strings.Contains(url, "gitlab.com") {
		return "gitlab"
	} else if strings.Contains(url, "bitbucket.org") {
		return "bitbucket"
	}
	return "git" // default type
}

// isURL checks if a string is a URL
func isURL(s string) bool {
	return s != "" && (len(s) > 4) && (strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "git@"))
}

// extractRepoName extracts a repository name from its URL
func extractRepoName(url string) string {
	// Remove trailing .git if present
	if strings.HasSuffix(url, ".git") {
		url = url[:len(url)-4]
	}

	// Extract the last component of the path
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		name := parts[len(parts)-1]
		if name != "" {
			return name
		}
	}

	// Fallback to a generic name
	return "repository"
}

// progressWriter is a helper to relay Git clone progress to the display
type progressWriter struct {
	ch chan<- string
}

// Write implements io.Writer
func (pw progressWriter) Write(p []byte) (n int, err error) {
	// Extract a readable progress message
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		select {
		case pw.ch <- msg:
		default:
			// Don't block if channel is full
		}
	}
	return len(p), nil
}

// Close closes the progress channel
func (pw progressWriter) Close() error {
	close(pw.ch)
	return nil
}
