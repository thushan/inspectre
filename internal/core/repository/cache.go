package repository

import (
	"github.com/go-git/go-git/v5"
	"github.com/thushan/inspectre/internal/core/utils"
	"time"
)

// cacheRepository adds a repo to the cache
func (m *Manager) cacheRepository(repoURL string, repo *git.Repository, clonePath string) {
	m.repoCacheMu.Lock()
	defer m.repoCacheMu.Unlock()

	m.repoCache[repoURL] = &RepoCache{
		Repo:      repo,
		LastUsed:  time.Now(),
		ClonePath: clonePath,
	}
}

// checkRepoCache checks if repo is in cache and copies to target dir if needed
func (m *Manager) checkRepoCache(repoURL, targetDir string) bool {
	m.repoCacheMu.RLock()
	cachedRepo, exists := m.repoCache[repoURL]
	m.repoCacheMu.RUnlock()

	if !exists || time.Since(cachedRepo.LastUsed) > RepoCacheTTL {
		return false
	}

	// Cache hit, but we need to copy to target dir if they're different
	if cachedRepo.ClonePath == targetDir {
		// Same path, no need to copy
		return true
	}

	// Copy repository to target
	err := utils.CopyDirectory(cachedRepo.ClonePath, targetDir)
	if err != nil {
		m.logger.Warning("Failed to copy cached repo, will clone instead: %v", err)
		return false
	}

	// Update last used time
	m.repoCacheMu.Lock()
	cachedRepo.LastUsed = time.Now()
	m.repoCacheMu.Unlock()

	return true
}

// invalidateRepoCache removes a repo from the cache
func (m *Manager) invalidateRepoCache(key string) {
	if key == "" {
		return
	}

	m.repoCacheMu.Lock()
	defer m.repoCacheMu.Unlock()

	delete(m.repoCache, key)
}
