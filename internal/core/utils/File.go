package utils

import (
	"fmt"
	"os"
	"time"
)

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// EnsureTaskDirectories creates all required directories for a task
func EnsureTaskDirectories(baseDir, repoDir, assetsDir string) error {
	dirs := []string{baseDir, repoDir, assetsDir}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// SafeRemoveAll attempts to remove a directory with retries for Windows systems
func SafeRemoveAll(path string) error {
	maxRetries := 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		err := os.RemoveAll(path)
		if err == nil {
			return nil // Success
		}

		lastErr = err
		// Exponential backoff
		delay := 100 * (1 << i) // 100ms, 200ms, 400ms

		// Sleep to give file handles time to close, must be a better way?
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}

	return fmt.Errorf("failed to remove directory after %d attempts: %w", maxRetries, lastErr)
}
