package types

import (
	"github.com/thushan/inspectre/internal/core/analysis"
	"time"
)
import "io"

// DisplayProvider defines the interface for display operations
type DisplayProvider interface {
	StartSpinner(text string) SpinnerProvider
	UpdateSpinnerText(text string)
	StopSpinner(text string)
	ShowSuccess(message string)
	ShowInfo(message string)
	ShowWarning(message string)
	ShowError(message string)
	Confirm(message string) bool
	ShowResults(results []*analysis.Result)
	ShowHeader(title string)
	ShowTaskInfo(task *Task)
	PrintResultTable(headers []string, rows [][]string)
}

// SpinnerProvider defines the interface for spinner operations
type SpinnerProvider interface {
	UpdateText(text string)
	Success(text string)
	Fail(text string)
	Warning(text string)
	Info(text string)
}

// TaskManager defines the interface for task operations
type TaskManager interface {
	CreateTask(nameOrURL string) (*Task, error)
	StartTask(taskID string) error
	GetTask(taskID string) (*Task, error)
	ListTasks(showAll, showFailed bool) []*Task
	GetLogReader(taskID string) (io.ReadCloser, error)
	CancelTask(taskID string) error
}

// RepositoryManager defines the interface for repository operations
type RepositoryManager interface {
	GetRepository(nameOrURL string) (*Repository, error)
	ListRepositories() ([]Repository, error)
	Clone(repo *Repository, targetDir string) error
	CleanUp(task *Task) error
}

// Task represents a running analysis task
type Task struct {
	ID         string    `json:"id"`
	Repository string    `json:"repository"` // Repository URL or name
	Status     string    `json:"status"`     // Created, Running, Completed, Failed
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time,omitempty"`
	BaseDir    string    `json:"base_dir"`   // Base directory for all task files
	RepoDir    string    `json:"repo_dir"`   // Directory where repo is cloned
	AssetsDir  string    `json:"assets_dir"` // Directory for storing assets/results
	LogFile    string    `json:"log_file"`   // Path to log file
	Error      string    `json:"error,omitempty"`
}

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
