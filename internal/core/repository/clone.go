package repository

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/core/utils"
	"os"
	"sync"
	"time"
)

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

	// Use WaitGroup to ensure progress goroutine completes
	var progressWg sync.WaitGroup
	progressWg.Add(1)

	// Process progress updates in the background with proper resource cleanup
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		defer progressWg.Done()
		defer func() {
			if r := recover(); r != nil {
				m.logger.Error("Panic in clone progress handler: %v", r)
			}
		}()

		for {
			select {
			case <-m.ctx.Done():
				m.logger.Debug("Clone progress handler shutting down: context cancelled")
				return
			case progress, ok := <-progressCh:
				if !ok {
					m.logger.Debug("Clone progress handler shutting down: channel closed")
					return
				}

				if spinner != nil {
					status := progress.Message
					if progress.Total > 0 {
						percent := int((float64(progress.Current) / float64(progress.Total)) * 100)
						status = fmt.Sprintf("%s (%d%%)", progress.Message, percent)
					}
					spinner.UpdateText(status)
				}
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
			m.logger.Debug("Cancelling clone operation due to manager shutdown")
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

	// Wait for progress handler to exit with timeout to avoid leaks
	done := make(chan struct{})
	go func() {
		progressWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Progress handler exited properly
	case <-time.After(500 * time.Millisecond):
		m.logger.Warning("Timeout waiting for progress handler to exit")
	}

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
