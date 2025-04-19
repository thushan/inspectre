package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thushan/inspectre/internal/core/analyser"
)

// Analyser implements an analyser that runs external CLI tools
type Analyser struct {
	name     string
	execPath string
	config   map[string]string
	repoPath string
	env      map[string]string
}

// NewCLIAnalyser creates a new CLI analyser
func NewCLIAnalyser(name, execPath string, config map[string]string) *Analyser {
	if config == nil {
		config = make(map[string]string)
	}

	return &Analyser{
		name:     name,
		execPath: execPath,
		config:   config,
	}
}

// Name returns the analyser's identifier
func (a *Analyser) Name() string {
	return a.name
}

// Initialize prepares the analyser
func (a *Analyser) Initialize(repoPath string, env map[string]string) error {
	a.repoPath = repoPath
	a.env = env

	// Ensure the executable exists
	if !isExecutable(a.execPath) {
		return fmt.Errorf("executable not found or not executable: %s", a.execPath)
	}

	return nil
}

// Run executes the CLI tool and collects metrics
func (a *Analyser) Run() ([]analyser.Metric, error) {
	// Prepare command
	cmd := exec.Command(a.execPath, a.repoPath)

	cmd.Dir = a.repoPath
	cmd.Env = os.Environ()

	for k, v := range a.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	for k, v := range a.config {
		cmd.Env = append(cmd.Env, fmt.Sprintf("INSPECTRE_CONFIG_%s=%s", k, v))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("command failed: %w\nOutput: %s\nError: %s",
			err, stdout.String(), stderr.String())
	}

	// Parse output as JSON metrics
	var metrics []analyser.Metric

	// Try to parse the output as JSON metrics
	output := stdout.String()
	if err := json.Unmarshal([]byte(output), &metrics); err != nil {
		// If not JSON, create a simple metric with the raw output
		metrics = []analyser.Metric{
			{
				Name:      fmt.Sprintf("%s_output", a.name),
				Value:     strings.TrimSpace(output),
				Timestamp: time.Now(),
			},
		}
	}

	return metrics, nil
}

// Cleanup performs any necessary cleanup
func (a *Analyser) Cleanup() error {
	// Most CLI tools don't need cleanup
	return nil
}

// isExecutable checks if a file exists and is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if it's a regular file and has execute permission
	return !info.IsDir() && (info.Mode()&0111 != 0)
}
