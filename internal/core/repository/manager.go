package repository

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lithammer/shortuuid/v4"
	"github.com/thushan/inspectre/internal/core/utils"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

var (
	ErrRepositoryNotFound    = errors.New("repository not found")
	ErrInvalidConfiguration  = errors.New("invalid configuration")
	ErrCloneFailure          = errors.New("failed to clone repository")
	ErrUnsupportedRepoType   = errors.New("unsupported repository type")
	ErrMissingAuthentication = errors.New("missing authentication details")
)

// Manager implements the RepositoryManager interface
type Manager struct {
	configPath string
	config     *Config
}

// NewManager creates a new repository manager
func NewManager(configPath string) (*Manager, error) {
	if configPath == "" {
		// Default config path
		configPath = "configs/repositories.json"
	}

	m := &Manager{
		configPath: configPath,
	}

	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadConfig reads and parses the repositories configuration file
func (m *Manager) loadConfig() error {
	file, err := os.Open(m.configPath)
	if err != nil {
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

// GetRepository finds a repository by name or URL
func (m *Manager) GetRepository(nameOrURL string) (*Repository, error) {
	for _, repo := range m.config.Repositories {
		if repo.Name == nameOrURL || repo.URL == nameOrURL {
			return &repo, nil
		}
	}
	return nil, ErrRepositoryNotFound
}

// ListRepositories returns all configured repositories
func (m *Manager) ListRepositories() ([]Repository, error) {
	return m.config.Repositories, nil
}

// Clone clones a repository to the specified target directory
func (m *Manager) Clone(repo *Repository, targetDir string) error {
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
		URL:      repo.URL,
		Progress: os.Stdout,
	}

	if auth != nil {
		cloneOpts.Auth = auth
	}

	_, err = git.PlainClone(targetDir, false, cloneOpts)
	if err != nil {
		// Clean up the directory if cloning fails, but don't worry too much about errors
		_ = utils.SafeRemoveAll(targetDir)
		return fmt.Errorf("%w: %v", ErrCloneFailure, err)
	}

	return nil
}

// CleanUp removes the temporary directory
func (m *Manager) CleanUp(task *Task) error {
	if task == nil {
		return errors.New("task cannot be nil")
	}

	// Close any open files first
	time.Sleep(100 * time.Millisecond) // Small delay to ensure files are released

	// Try to clean up with retries
	var lastErr error
	maxRetries := 3

	for i := 0; i < maxRetries; i++ {
		err := os.RemoveAll(task.BaseDir)
		if err == nil {
			return nil // Successfully removed
		}

		lastErr = err
		// Wait a bit longer between retries
		time.Sleep(500 * time.Millisecond * time.Duration(i+1))
	}

	return fmt.Errorf("failed to clean up task directory after %d attempts: %w", maxRetries, lastErr)
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
		return nil, ErrUnsupportedRepoType
	}
}

// CreateTask creates a new analysis task
func (m *Manager) CreateTask(nameOrURL string) (*Task, error) {
	repo, err := m.GetRepository(nameOrURL)
	if err != nil {
		// Handle case when URL is provided directly
		if strings.HasPrefix(nameOrURL, "http") || strings.HasPrefix(nameOrURL, "git@") {
			// Create a temporary repository entry
			repo = &Repository{
				Name: filepath.Base(nameOrURL),
				URL:  nameOrURL,
				Type: GuessRepoType(nameOrURL),
				Auth: Auth{}, // No auth provided
			}
		} else {
			return nil, err
		}
	}

	// Generate task ID
	taskID := shortuuid.NewWithAlphabet("0123456789abcdef")

	// Create base task directory
	baseDir := filepath.Join(os.TempDir(), "inspectre", taskID)

	// Create separate subdirectories
	repoDir := filepath.Join(baseDir, "_repo")         // Repository clone directory
	assetsDir := filepath.Join(baseDir, "assets")      // Assets and plugin data
	logFile := filepath.Join(baseDir, "inspectre.log") // Main log file

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
