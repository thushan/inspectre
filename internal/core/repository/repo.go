package repository

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
