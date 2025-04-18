package utils

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// File operation constants
const (
	// File permissions
	DirPerm  = 0755
	FilePerm = 0644

	// Copy operation settings
	CopyBufferSize = 32 * 1024              // 32 KB buffer
	MaxFileSize    = 2 * 1024 * 1024 * 1024 // 2 GB maximum file size
	BatchSize      = 100                    // Number of files to process in a batch
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
		if err := os.MkdirAll(dir, DirPerm); err != nil {
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

// CopyDirectory recursively copies a directory tree, preserving file mode
func CopyDirectory(src, dst string) error {
	// Get properties of source directory
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	directory, err := os.Open(src)
	if err != nil {
		return err
	}
	defer directory.Close()

	entries, err := directory.ReadDir(-1)
	if err != nil {
		return err
	}

	// Process files and subdirectories in batches for better performance
	for i := 0; i < len(entries); i += BatchSize {
		end := i + BatchSize
		if end > len(entries) {
			end = len(entries)
		}

		batch := entries[i:end]
		for _, entry := range batch {
			currSrc := filepath.Join(src, entry.Name())
			currDst := filepath.Join(dst, entry.Name())

			if entry.IsDir() {
				// Recursively copy the directory
				if err = CopyDirectory(currSrc, currDst); err != nil {
					return err
				}
			} else {
				// Copy the file
				if err = CopyFile(currSrc, currDst); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// CopyFile copies a single file from src to dst
func CopyFile(src, dst string) error {
	// Get file info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Check file size to avoid excessive memory usage
	if srcInfo.Size() > MaxFileSize {
		return fmt.Errorf("file too large to copy safely: %s (%d bytes)", src, srcInfo.Size())
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.CreateTemp(filepath.Dir(dst), ".tmp-copy-*")
	if err != nil {
		return err
	}
	tempPath := dstFile.Name()

	// Copy file contents with a buffer for efficiency
	buf := make([]byte, CopyBufferSize)
	_, err = io.CopyBuffer(dstFile, srcFile, buf)

	// Close file before further operations
	dstFile.Close()

	if err != nil {
		// Clean up the temp file on error
		os.Remove(tempPath)
		return err
	}

	// Set permissions to match source
	if err = os.Chmod(tempPath, srcInfo.Mode()); err != nil {
		os.Remove(tempPath)
		return err
	}

	// Rename to final destination (atomic operation)
	if err = os.Rename(tempPath, dst); err != nil {
		os.Remove(tempPath)
		return err
	}

	return nil
}

// BatchProcessFiles processes files in batches for better performance
func BatchProcessFiles(ctx context.Context, rootDir string, fn func(path string) error) error {
	var (
		batch     []string
		batchSize = BatchSize
	)

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		if err != nil {
			return err
		}

		if !info.IsDir() {
			batch = append(batch, path)

			// Process batch when full
			if len(batch) >= batchSize {
				if err := processBatch(ctx, batch, fn); err != nil {
					return err
				}
				batch = batch[:0] // Clear but keep capacity
			}
		}
		return nil
	})

	// Process any remaining files
	if err == nil && len(batch) > 0 {
		err = processBatch(ctx, batch, fn)
	}

	return err
}

// processBatch applies the function to each file in a batch
func processBatch(ctx context.Context, batch []string, fn func(path string) error) error {
	for _, path := range batch {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		if err := fn(path); err != nil {
			return err
		}
	}
	return nil
}
