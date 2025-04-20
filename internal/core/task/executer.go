package task

import (
	"context"
	"fmt"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/types"
	"github.com/thushan/inspectre/internal/core/ui/theme"
	"github.com/thushan/inspectre/internal/core/utils"
	"os"
	"time"
)

// executeTask performs the actual repository analyser
func (m *Manager) executeTask(task *types.Task) TaskResult {
	// Create task-specific context that can be cancelled
	taskCtx, taskCancel := context.WithTimeout(m.ctx, TaskExecutionTimeout)
	defer taskCancel()

	result := TaskResult{
		TaskID:      task.ID,
		Repository:  task.Repository,
		CompletedAt: time.Now(),
	}

	// Get repository details
	repo, err := m.getRepositoryDetails(task.Repository)
	if err != nil {
		result.Error = err
		return result
	}

	// Check if task context is cancelled
	if taskCtx.Err() != nil {
		result.Error = ErrTaskCancelled
		return result
	}

	// Ensure directories exist
	if err := m.ensureTaskDirectories(task); err != nil {
		result.Error = err
		return result
	}

	// Send UI event
	m.sendUIEvent(UIEvent{
		TaskID:    task.ID,
		EventType: "cloning",
		Message:   fmt.Sprintf("Cloning repository %s", theme.ColourRepository(repo.URL)),
	})

	// Clone the repository
	m.logger.TaskInfo(task.ID, "Cloning repository %s to %s", theme.ColourRepository(repo.URL), theme.ColourWorkDir(task.RepoDir))
	if err := m.repoManager.Clone(repo, task.RepoDir); err != nil {
		result.Error = fmt.Errorf("failed to clone repository: %w", err)
		return result
	}

	// Check if task context is cancelled
	if taskCtx.Err() != nil {
		result.Error = ErrTaskCancelled
		return result
	}

	// Run analyser
	m.logger.TaskInfo(task.ID, "Repository cloned successfully. Beginning analyser...")

	// Send UI event
	m.sendUIEvent(UIEvent{
		TaskID:    task.ID,
		EventType: "analysing",
		Message:   "Running analysers...",
	})

	// Create analyser list starting with built-in analysers
	analysers := m.prepareAnalysers(task)

	// Create logging function for analyser manager
	loggerFn := func(format string, args ...interface{}) {
		m.logger.TaskInfo(task.ID, format, args...)
	}

	// Set up analyser manager
	analyserManager := analyser.NewManager(analysers, loggerFn)

	// Prepare environment variables for analysers
	env := m.createAnalyserEnvironment(task, repo)

	// Check context again
	if taskCtx.Err() != nil {
		result.Error = ErrTaskCancelled
		return result
	}

	// Run the analysers
	m.logger.TaskInfo(task.ID, "Running analysers on repository...")

	results, err := analyserManager.AnalyseRepository(task.RepoDir, env)
	if err != nil {
		result.Error = fmt.Errorf("analyser failed: %w", err)
		return result
	}

	// Log results
	m.logger.TaskInfo(task.ID, "Analysis completed with %d result sets", len(results))

	// Store results in output
	result.Results = results

	return result
}

// getRepositoryDetails retrieves or creates repository details
func (m *Manager) getRepositoryDetails(repoNameOrURL string) (*types.Repository, error) {
	repo, err := m.repoManager.GetRepository(repoNameOrURL)
	if err != nil {
		// Handle URLs that aren't in the config
		if !utils.IsURLString(repoNameOrURL) {
			return nil, fmt.Errorf("repository not found: %w", err)
		}

		// Create temporary repository object for direct URLs
		repo = &types.Repository{
			URL:  repoNameOrURL,
			Type: utils.GuessRepoType(repoNameOrURL),
			Auth: types.Auth{}, // Empty Auth struct
		}
	}

	return repo, nil
}

// ensureTaskDirectories creates required directories for a task
func (m *Manager) ensureTaskDirectories(task *types.Task) error {
	dirs := []string{task.BaseDir, task.RepoDir, task.AssetsDir}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Register task log file
	if err := m.logger.RegisterTaskLog(task.ID, task.LogFile); err != nil {
		return fmt.Errorf("failed to register task log: %w", err)
	}

	return nil
}

// prepareAnalysers creates the list of analysers to run
func (m *Manager) prepareAnalysers(task *types.Task) []analyser.Analyser {
	// Create analyser list starting with built-in analysers
	analysers := []analyser.Analyser{
		analyser.NewFileAnalyser(),
		analyser.NewGitAnalyser(),
	}

	// Add extension analysers if extension manager is available
	if m.extensionManager != nil {
		m.logger.TaskInfo(task.ID, "Loading extensions...")
		extAnalysers, err := m.extensionManager.LoadAllEnabled()
		if err != nil {
			m.logger.TaskWarning(task.ID, "Warning: failed to load some extensions: %v", err)
		}

		if len(extAnalysers) > 0 {
			analysers = append(analysers, extAnalysers...)
			m.logger.TaskInfo(task.ID, "Loaded %d extension analysers", len(extAnalysers))
		}
	}

	return analysers
}

// createAnalyserEnvironment prepares environment variables for analysers
func (m *Manager) createAnalyserEnvironment(task *types.Task, repo *types.Repository) map[string]string {
	return map[string]string{
		"REPOSITORY_NAME": repo.Name,
		"REPOSITORY_URL":  repo.URL,
		"REPOSITORY_TYPE": repo.Type,
		"TASK_ID":         task.ID,
		"ASSETS_DIR":      task.AssetsDir,
	}
}
