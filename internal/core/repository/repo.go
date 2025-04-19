package repository

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/core/utils"
	"os"
)

// GetRepository finds a repository by name or URL
func (m *Manager) GetRepository(nameOrURL string) (*Repository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return nil, ErrManagerClosed
	}
	m.closedMu.RUnlock()

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

	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return ErrManagerClosed
	}
	m.closedMu.RUnlock()

	// Check if repository already exists
	for i, existing := range m.config.Repositories {
		if existing.Name == repo.Name {
			// Update existing repository
			m.config.Repositories[i] = repo

			// Invalidate cache
			m.invalidateRepoCache(repo.Name)
			m.invalidateRepoCache(repo.URL)

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

	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return ErrManagerClosed
	}
	m.closedMu.RUnlock()

	for i, repo := range m.config.Repositories {
		if repo.Name == nameOrURL || repo.URL == nameOrURL {
			// Remove repository
			m.config.Repositories = append(m.config.Repositories[:i], m.config.Repositories[i+1:]...)

			// Invalidate cache
			m.invalidateRepoCache(repo.Name)
			m.invalidateRepoCache(repo.URL)

			return m.SaveConfig()
		}
	}

	return ErrRepositoryNotFound
}

// ListRepositories returns all configured repositories
func (m *Manager) ListRepositories() ([]Repository, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return nil, ErrManagerClosed
	}
	m.closedMu.RUnlock()

	// Create a copy to prevent modification of config
	repos := make([]Repository, len(m.config.Repositories))
	copy(repos, m.config.Repositories)

	return repos, nil
}

// Clone clones a repository to the specified target directory
func (m *Manager) Clone(repo *Repository, targetDir string) error {
	// Check if manager is closed
	m.closedMu.RLock()
	if m.closed {
		m.closedMu.RUnlock()
		return ErrManagerClosed
	}
	m.closedMu.RUnlock()

	// Start a spinner if display is available
	var spinner types.SpinnerProvider
	if m.display != nil {
		spinner = m.display.StartSpinner(fmt.Sprintf("Cloning %s", repo.URL))
		defer spinner.Success(fmt.Sprintf("Clone of %s completed", repo.URL))
	}

	// Check cache first
	if cached := m.checkRepoCache(repo.URL, targetDir); cached {
		m.logger.Info("Using cached repository: %s", repo.URL)
		if spinner != nil {
			spinner.UpdateText(fmt.Sprintf("Using cached repository: %s", repo.URL))
		}
		return nil
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
	if err := os.MkdirAll(targetDir, DirectoryPerm); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	auth, err := m.getAuthMethod(repo)
	if err != nil {
		return err
	}

	// Set up clone progress channel
	progressCh := make(chan CloneProgress, CloneProgressSize)

	// Process progress updates in the background
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for progress := range progressCh {
			if spinner != nil {
				status := progress.Message
				if progress.Total > 0 {
					percent := int((float64(progress.Current) / float64(progress.Total)) * 100)
					status = fmt.Sprintf("%s (%d%%)", progress.Message, percent)
				}
				spinner.UpdateText(status)
			}
		}
	}()

	cloneOpts := &git.CloneOptions{
		URL: repo.URL,
	}

	if auth != nil {
		cloneOpts.Auth = auth
	}

	// Use progress writer if spinner is available
	if spinner != nil {
		cloneOpts.Progress = &progressWriter{ch: progressCh}
	}

	m.logger.Info("Cloning repository %s to %s", repo.URL, targetDir)

	// Create a context that can be cancelled
	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	// Watch for shutdown
	go func() {
		select {
		case <-m.ctx.Done():
			// Manager shutting down, cancel clone
			cancel()
		case <-ctx.Done():
			// Clone finished or was cancelled
		}
	}()

	// Perform clone with timeout
	cloneCtx, cloneCancel := context.WithTimeout(ctx, CloneTimeout)
	defer cloneCancel()

	repository, err := git.PlainCloneContext(cloneCtx, targetDir, false, cloneOpts)

	// Close progress channel when clone completes
	close(progressCh)

	if err != nil {
		// Clean up the directory if cloning fails
		_ = utils.SafeRemoveAll(targetDir)
		return fmt.Errorf("%w: %v", ErrCloneFailure, err)
	}

	// Add to cache
	m.cacheRepository(repo.URL, repository, targetDir)

	m.logger.Info("Successfully cloned repository %s", repo.URL)
	return nil
}
