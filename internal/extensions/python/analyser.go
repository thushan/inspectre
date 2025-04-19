package python

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

// PythonAnalyser implements an analyser that runs Python scripts
type PythonAnalyser struct {
	name       string
	scriptPath string
	config     map[string]string
	repoPath   string
	env        map[string]string
	pythonPath string
}

// NewPythonAnalyser creates a new Python analyser
func NewPythonAnalyser(name, scriptPath string, config map[string]string) *PythonAnalyser {
	if config == nil {
		config = make(map[string]string)
	}

	// Find Python executable
	pythonPath := "python3"
	if _, err := exec.LookPath(pythonPath); err != nil {
		// Try python as fallback
		pythonPath = "python"
		if _, err := exec.LookPath(pythonPath); err != nil {
			// Will report error during initialization
			pythonPath = ""
		}
	}

	return &PythonAnalyser{
		name:       name,
		scriptPath: scriptPath,
		config:     config,
		pythonPath: pythonPath,
	}
}

// Name returns the analyser's identifier
func (a *PythonAnalyser) Name() string {
	return a.name
}

// Initialize prepares the analyser
func (a *PythonAnalyser) Initialize(repoPath string, env map[string]string) error {
	if a.pythonPath == "" {
		return fmt.Errorf("python executable not found in PATH")
	}

	a.repoPath = repoPath
	a.env = env

	// Ensure the script exists
	if _, err := os.Stat(a.scriptPath); os.IsNotExist(err) {
		return fmt.Errorf("script not found: %s", a.scriptPath)
	}

	return nil
}

// Run executes the Python script and collects metrics
func (a *PythonAnalyser) Run() ([]analyser.Metric, error) {
	// Prepare command
	cmd := exec.Command(a.pythonPath, a.scriptPath, a.repoPath)

	// Set working directory to the repository path
	cmd.Dir = a.repoPath

	// Prepare environment
	cmd.Env = os.Environ()
	for k, v := range a.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add config values to environment
	for k, v := range a.config {
		cmd.Env = append(cmd.Env, fmt.Sprintf("INSPECTRE_CONFIG_%s=%s", k, v))
	}

	// Add special environment variables for Python
	cmd.Env = append(cmd.Env, "PYTHONUNBUFFERED=1")

	// Set up output buffers
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("python script failed: %w\nOutput: %s\nError: %s",
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
func (a *PythonAnalyser) Cleanup() error {
	// Most Python scripts don't need cleanup
	return nil
}
