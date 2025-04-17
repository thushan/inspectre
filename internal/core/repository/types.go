package repository

import (
	"time"
)

// Repository defines a code repository configuration
type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Type string `json:"type"` // github, gitlab, bitbucket
	Auth Auth   `json:"auth"`
}

// Auth contains authentication details for repository access
type Auth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

// Config defines the structure of the repositories.json file
type Config struct {
	Version      int          `json:"version"`
	Repositories []Repository `json:"repositories"`
}

// RepositoryManager handles repository operations
type RepositoryManager interface {
	GetRepository(nameOrURL string) (*Repository, error)
	ListRepositories() ([]Repository, error)
	Clone(repo *Repository, targetDir string) error
	CleanUp(targetDir string) error
}

// Task represents a running analysis task
type Task struct {
	ID         string    `json:"id"`
	Repository string    `json:"repository"` // Repository URL or name
	Status     string    `json:"status"`     // Created, Running, Completed, Failed
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time,omitempty"`
	WorkDir    string    `json:"work_dir"` // Temporary directory where repo is cloned
	LogFile    string    `json:"log_file"` // Path to log file
	Error      string    `json:"error,omitempty"`
}
