package analyser

import (
	"errors"
	"fmt"
	"github.com/thushan/inspectre/internal/core/ui/theme"
	"os"
	"path/filepath"
	"time"
)

var (
	ErrInvalidRepoPath  = errors.New("invalid repository path")
	ErrNoAnalysersFound = errors.New("no analysers found")
	ErrAnalysisFailed   = errors.New("analyser failed")
)

// Metric represents a single measurement from an analyser
type Metric struct {
	Name      string            `json:"name"`
	Key       string            `json:"key,omitempty"`
	Value     interface{}       `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// Result contains the output of an analyser run
type Result struct {
	AnalyserName string    `json:"analyser_name"`
	Repository   string    `json:"repository"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	Metrics      []Metric  `json:"metrics"`
	Error        string    `json:"error,omitempty"`
	Success      bool      `json:"success"`
}

// Analyser is the interface that all analysers must implement
type Analyser interface {
	// Name returns the analyser's identifier
	Name() string

	// Initialize prepares the analyser with the repository path and environment
	Initialize(repoPath string, env map[string]string) error

	// Run performs the analyser and returns metrics
	Run() ([]Metric, error)

	// Cleanup performs any necessary cleanup
	Cleanup() error
}

// Manager coordinates analyser operations
type Manager struct {
	analysers []Analyser
	logger    func(format string, args ...interface{})
}

// NewManager creates a new analyser manager
func NewManager(analysers []Analyser, logger func(format string, args ...interface{})) *Manager {
	if logger == nil {
		logger = func(format string, args ...interface{}) {
			fmt.Printf(format+"\n", args...)
		}
	}

	return &Manager{
		analysers: analysers,
		logger:    logger,
	}
}

// AnalyseRepository runs all registered analysers on a repository
func (m *Manager) AnalyseRepository(repoPath string, env map[string]string) ([]*Result, error) {
	// Validate the repository path
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRepoPath, repoPath)
	}

	// Verify we have analysers
	if len(m.analysers) == 0 {
		return nil, ErrNoAnalysersFound
	}

	var results []*Result

	// Run each analyser
	for _, analyser := range m.analysers {
		result := &Result{
			AnalyserName: analyser.Name(),
			Repository:   filepath.Base(repoPath),
			StartTime:    time.Now(),
			Success:      false,
		}

		m.logger("Running analyser: %s", theme.ColourAnalyser(analyser.Name()))

		// Initialize analyser
		if err := analyser.Initialize(repoPath, env); err != nil {
			result.Error = fmt.Sprintf("Initialization failed: %v", err)
			result.EndTime = time.Now()
			results = append(results, result)
			m.logger("Failed to initialize analyser %s: %v", theme.ColourAnalyser(analyser.Name()), err)
			continue
		}

		// Run analyser
		metrics, err := analyser.Run()
		if err != nil {
			result.Error = fmt.Sprintf("Execution failed: %v", err)
			result.EndTime = time.Now()
			results = append(results, result)
			m.logger("Analysis failed for %s: %v", theme.ColourAnalyser(analyser.Name()), err)
			continue
		}

		// Record metrics
		result.Metrics = metrics
		result.Success = true
		result.EndTime = time.Now()
		results = append(results, result)

		m.logger("Analyser %s completed successfully with %d metrics", theme.ColourAnalyser(analyser.Name()), len(metrics))

		// Cleanup
		if err := analyser.Cleanup(); err != nil {
			m.logger("Warning: cleanup failed for %s: %v", theme.ColourAnalyser(analyser.Name()), err)
		}
	}

	if len(results) == 0 {
		return nil, ErrAnalysisFailed
	}

	return results, nil
}

// RegisterAnalyser adds an analyser to the manager
func (m *Manager) RegisterAnalyser(analyser Analyser) {
	m.analysers = append(m.analysers, analyser)
	m.logger("Registered analyser: %s", theme.ColourAnalyser(analyser.Name()))
}
