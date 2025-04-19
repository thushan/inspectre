package cli

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

// Analyser implements an analyser that runs external CLI tools
type Analyser struct {
	name     string
	execPath string
	config   map[string]string
	repoPath string
	env      map[string]string
}

const (
	// ExecutionTimeout defines the maximum time allowed for a CLI tool to run
	ExecutionTimeout = 2 * time.Minute

	// OutputLimit defines the maximum output size in bytes to prevent memory issues
	OutputLimit = 10 * 1024 * 1024 // 10MB
)

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
	// Create a context with timeout to prevent hanging commands
	ctx, cancel := context.WithTimeout(context.Background(), ExecutionTimeout)
	defer cancel()

	// Prepare command with context
	cmd := exec.CommandContext(ctx, a.execPath, a.repoPath)

	cmd.Dir = a.repoPath
	cmd.Env = os.Environ()

	// Add environment variables for the command
	for k, v := range a.env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Add configuration as environment variables
	for k, v := range a.config {
		cmd.Env = append(cmd.Env, fmt.Sprintf("INSPECTRE_CONFIG_%s=%s", k, v))
	}

	// Set up output buffers with size limits
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{&stdout, OutputLimit}
	cmd.Stderr = &limitedWriter{&stderr, OutputLimit}

	// Run the command with proper error handling
	err := cmd.Run()

	// Check if context was cancelled or timeout occurred
	select {
	case <-ctx.Done():
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("command execution timed out after %v", ExecutionTimeout)
		}
		return nil, fmt.Errorf("command execution cancelled: %v", ctx.Err())
	default:
		// Context not cancelled, check for command errors
		if err != nil {
			// Capture the error output for better diagnostics
			errOutput := stderr.String()
			if len(errOutput) > 1000 {
				errOutput = errOutput[:1000] + "... (truncated)"
			}
			return nil, fmt.Errorf("command failed: %w\nError output: %s", err, errOutput)
		}
	}

	// Parse output as JSON metrics
	var metrics []analyser.Metric

	// Try to parse the output as JSON metrics
	output := stdout.String()
	if err := json.Unmarshal([]byte(output), &metrics); err != nil {
		// If not JSON, create a simple metric with the raw output
		// Truncate very large outputs to prevent memory issues
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

// limitedWriter is a writer that enforces a maximum size to prevent OOM issues
type limitedWriter struct {
	w     *bytes.Buffer
	limit int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	// Check if we'd exceed the limit
	if l.w.Len()+len(p) > l.limit {
		// Calculate how many bytes we can write without exceeding limit
		remaining := l.limit - l.w.Len()
		if remaining <= 0 {
			return 0, fmt.Errorf("output limit of %d bytes exceeded", l.limit)
		}

		// Write partial data up to the limit
		n, err := l.w.Write(p[:remaining])
		if err != nil {
			return n, err
		}

		// Return early with a truncation error
		return n, fmt.Errorf("output truncated after %d bytes", l.limit)
	}

	// Normal write if within limits
	return l.w.Write(p)
}
