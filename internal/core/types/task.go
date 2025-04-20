package types

import "time"

type TaskOptions struct {
	Timeout      time.Duration
	MaxRetries   int
	Priority     int
	Dependencies []string
	Environment  map[string]string
}

type TaskResult struct {
	Success    bool
	Data       any
	Artifacts  []string
	Metrics    map[string]float64
	Duration   time.Duration
	StartTime  time.Time
	EndTime    time.Time
	RetryCount int
}

type RepositoryInfo struct {
	Name       string
	URL        string
	Type       string // github, gitlab, bitbucket
	Branch     string
	Path       string // Local path after cloning
	CommitSHA  string
	CloneTime  time.Time
	AuthConfig *RepositoryAuthConfig
}

type RepositoryAuthConfig struct {
	Type        string // token, ssh, basic
	Username    string
	Password    string // or token
	SSHKeyPath  string
	TokenEnvVar string
}

type ExtensionInfo struct {
	Name        string
	Type        string // cli, python, golang
	Path        string
	Description string
	Enabled     bool
	Config      map[string]interface{}
}

type RepositoryAnalysisResult struct {
	Repository   *RepositoryInfo
	FileAnalysis *FileAnalysisResult
	GitAnalysis  *GitAnalysisResult
	Metrics      map[string]float64
	Extensions   map[string]interface{}
}

type FileAnalysisResult struct {
	TotalFiles       int
	TotalSize        int64
	FilesByType      map[string]int
	SizeByType       map[string]int64
	ImportantFiles   []string
	DirectoryDepth   int
	DirectoryBreadth int
}

type GitAnalysisResult struct {
	CommitCount     int
	BranchCount     int
	TagCount        int
	Contributors    int
	FirstCommitDate time.Time
	LastCommitDate  time.Time
	CommitsByMonth  map[string]int
}
