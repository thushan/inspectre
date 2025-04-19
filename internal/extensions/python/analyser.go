package python

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/thushan/inspectre/internal/core/analyser"
)

// Execution constraints to prevent resource leaks
const (
	ExecutionTimeout = 3 * time.Minute
	OutputLimit      = 10 * 1024 * 1024 // 10MB
)

// PythonAnalyser implements an analyser that runs Python scripts
type PythonAnalyser struct {
	name       string
	scriptPath string
	config     map[string]string
	repoPath   string
	env        map[string]string
	pythonPath string
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewPythonAnalyser creates a new Python analyser
func NewPythonAnalyser(name, scriptPath string, config map[string]string) *PythonAnalyser {
	if config == nil {
		config = make(map[string]string)
	}

	// Create a context that can be cancelled during cleanup
	ctx, cancel := context.WithCancel(context.Background())

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
		ctx:        ctx,
		cancel:     cancel,
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
	// Create a timeout context for execution
	execCtx, execCancel := context.WithTimeout(a.ctx, ExecutionTimeout)
	defer execCancel()

	// Prepare command with timeout context
	cmd := exec.CommandContext(execCtx, a.pythonPath, a.scriptPath, a.repoPath)

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

	// Set up output buffers with size limits
	var stdout, stderr limitedBuffer
	stdout.Limit = OutputLimit
	stderr.Limit = OutputLimit
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Check for context timeout or cancellation
	select {
	case <-execCtx.Done():
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("python script execution timed out after %v", ExecutionTimeout)
		}
		return nil, fmt.Errorf("python script execution cancelled: %v", execCtx.Err())
	default:
		// Process completed without timeout
	}

	if err != nil {
		// Truncate error output for readability
		errOutput := stderr.String()
		if len(errOutput) > 500 {
			errOutput = errOutput[:500] + "... (truncated)"
		}
		return nil, fmt.Errorf("python script failed: %w\nError output: %s", err, errOutput)
	}

	// Check if output was truncated
	if stdout.Truncated {
		return nil, fmt.Errorf("python script output exceeded limit of %d bytes", OutputLimit)
	}

	// Parse output as JSON metrics
	var metrics []analyser.Metric

	// Try to parse the output as JSON metrics
	output := stdout.String()
	if err := json.Unmarshal([]byte(output), &metrics); err != nil {
		// If not JSON, create a simple metric with the raw output
		if len(output) > 10000 {
			output = output[:10000] + "... (truncated)"
		}

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
	// Cancel context to stop any ongoing operations
	a.cancel()
	return nil
}

// limitedBuffer is a buffer that enforces a maximum size to prevent OOM issues
type limitedBuffer struct {
	bytes.Buffer
	Limit     int
	Truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > b.Limit {
		// Calculate how many bytes we can safely write
		allowedBytes := b.Limit - b.Len()
		if allowedBytes <= 0 {
			b.Truncated = true
			return 0, nil
		}

		// Only write up to the limit
		n, err := b.Buffer.Write(p[:allowedBytes])
		b.Truncated = true
		return n, err
	}

	return b.Buffer.Write(p)
}
