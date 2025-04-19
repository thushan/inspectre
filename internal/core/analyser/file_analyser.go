package analyser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// File size constants
const (
	KB = 1024
	MB = 1024 * KB
	GB = 1024 * MB

	// Common exclude directories
	GitDir         = ".git"
	NodeModulesDir = "node_modules"
	VendorDir      = "vendor"
	IDEADir        = ".idea"
	VSCodeDir      = ".vscode"

	// Worker count for parallel processing
	FileWorkerCount = 4

	// Buffer sizes for channels
	FileChannelSize = 1000

	// Batch size for file operations
	FileBatchSize = 100

	// Timeout for file operations
	FileOpTimeout = 30 * time.Second
)

// DefaultExcludeDirs contains default directories to exclude from analyser
var DefaultExcludeDirs = []string{GitDir, NodeModulesDir, VendorDir, IDEADir, VSCodeDir}

// FileInfo contains information about a file
type FileInfo struct {
	Path      string
	Size      int64
	Extension string
	IsDir     bool
}

// FileAnalyser counts files by extension and provides basic stats
type FileAnalyser struct {
	repoPath     string
	excludeDirs  []string
	fileStats    map[string]int
	totalFiles   int
	totalSize    int64
	largestFile  string
	largestSize  int64
	extensionMap map[string]int
	resultsMu    sync.Mutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewFileAnalyser creates a new file analyser
func NewFileAnalyser() *FileAnalyser {
	// Create a default context that can be cancelled during cleanup
	ctx, cancel := context.WithCancel(context.Background())

	return &FileAnalyser{
		excludeDirs:  DefaultExcludeDirs,
		fileStats:    make(map[string]int),
		extensionMap: make(map[string]int),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Name returns the analyser identifier
func (a *FileAnalyser) Name() string {
	return "file_analyser"
}

// Initialize prepares the analyser
func (a *FileAnalyser) Initialize(repoPath string, env map[string]string) error {
	a.repoPath = repoPath

	// Check if custom exclude dirs are provided
	if excludes, ok := env["EXCLUDE_DIRS"]; ok && excludes != "" {
		a.excludeDirs = strings.Split(excludes, ",")
	}

	return nil
}

// Run performs the analyser with parallelization
func (a *FileAnalyser) Run() ([]Metric, error) {
	// Create a context with timeout for the entire operation
	runCtx, cancel := context.WithTimeout(a.ctx, FileOpTimeout)
	defer cancel()

	// Create channels for worker pool
	filesChan := make(chan FileInfo, FileChannelSize)
	resultsChan := make(chan error, FileWorkerCount)

	// Start worker pool
	var wg sync.WaitGroup

	// Start consumers
	for i := 0; i < FileWorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					resultsChan <- fmt.Errorf("panic in file processor: %v", r)
				}
			}()
			a.processFiles(runCtx, filesChan)
		}()
	}

	// Start producer
	producerDone := make(chan struct{})
	go func() {
		defer close(producerDone)
		defer close(filesChan)
		defer func() {
			if r := recover(); r != nil {
				resultsChan <- fmt.Errorf("panic in file walker: %v", r)
			}
		}()

		err := a.walkRepository(runCtx, filesChan)
		if err != nil {
			resultsChan <- fmt.Errorf("failed to walk repository: %w", err)
		}
	}()

	// Wait for producer to finish with timeout
	select {
	case <-producerDone:
		// Producer finished successfully
	case <-runCtx.Done():
		return nil, fmt.Errorf("file analysis timed out during directory walk: %w", runCtx.Err())
	}

	// Wait for workers to finish with timeout
	workersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workersDone)
		close(resultsChan)
	}()

	select {
	case <-workersDone:
		// Workers finished successfully
	case <-runCtx.Done():
		return nil, fmt.Errorf("file analysis timed out during processing: %w", runCtx.Err())
	}

	// Check for errors
	for err := range resultsChan {
		if err != nil {
			return nil, err
		}
	}

	// Build metrics from results
	return a.buildMetrics(), nil
}

// walkRepository traverses the repository and sends file info to the channel
func (a *FileAnalyser) walkRepository(ctx context.Context, filesChan chan<- FileInfo) error {
	return filepath.Walk(a.repoPath, func(path string, info os.FileInfo, err error) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue processing
		}

		if err != nil {
			return err
		}

		// Skip excluded directories
		if info.IsDir() {
			relPath, err := filepath.Rel(a.repoPath, path)
			if err != nil {
				return nil // Skip paths we can't make relative
			}

			// Check if this directory should be excluded
			for _, excludeDir := range a.excludeDirs {
				if relPath == excludeDir || strings.HasPrefix(relPath, excludeDir+string(os.PathSeparator)) {
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Process file
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if ext == "" {
			ext = "(none)"
		}

		// Send file info to channel with context awareness
		select {
		case filesChan <- FileInfo{
			Path:      path,
			Size:      info.Size(),
			Extension: ext,
			IsDir:     false,
		}:
			// Successfully sent
		case <-ctx.Done():
			return ctx.Err()
		}

		return nil
	})
}

// processFiles consumes file info from the channel and updates statistics
func (a *FileAnalyser) processFiles(ctx context.Context, filesChan <-chan FileInfo) {
	// Process files in batches for better performance
	batch := make([]FileInfo, 0, FileBatchSize)

	for {
		select {
		case <-ctx.Done():
			// Context cancelled, process any remaining files and exit
			if len(batch) > 0 {
				a.processBatch(batch)
			}
			return
		case fileInfo, ok := <-filesChan:
			if !ok {
				// Channel closed, process any remaining files and exit
				if len(batch) > 0 {
					a.processBatch(batch)
				}
				return
			}

			batch = append(batch, fileInfo)

			// Process batch when it's full
			if len(batch) >= FileBatchSize {
				a.processBatch(batch)
				batch = batch[:0] // Clear batch but keep capacity
			}
		}
	}
}

// processBatch updates statistics for a batch of files
func (a *FileAnalyser) processBatch(batch []FileInfo) {
	// Collect stats locally first
	var batchSize int64
	var batchLargest int64
	var batchLargestPath string
	batchExtensions := make(map[string]int)

	for _, file := range batch {
		batchSize += file.Size
		batchExtensions[file.Extension]++

		if file.Size > batchLargest {
			batchLargest = file.Size
			batchLargestPath = file.Path
		}
	}

	// Update global stats once with lock
	a.resultsMu.Lock()
	defer a.resultsMu.Unlock()

	a.totalFiles += len(batch)
	a.totalSize += batchSize

	for ext, count := range batchExtensions {
		a.extensionMap[ext] += count
	}

	if batchLargest > a.largestSize {
		a.largestSize = batchLargest
		a.largestFile, _ = filepath.Rel(a.repoPath, batchLargestPath)
	}
}

// buildMetrics creates metrics from the collected statistics
func (a *FileAnalyser) buildMetrics() []Metric {
	now := time.Now()
	nExt := len(a.extensionMap)
	metricTotals := 3

	exts := make([]string, 0, nExt)
	for ext := range a.extensionMap {
		exts = append(exts, ext)
	}
	sort.Strings(exts)

	metrics := make([]Metric, 0, metricTotals+nExt)

	metrics = append(metrics, Metric{
		Name:      "total_files",
		Value:     a.totalFiles,
		Timestamp: now,
	})

	metrics = append(metrics, Metric{
		Name:      "total_size_bytes",
		Value:     a.totalSize,
		Timestamp: now,
	})

	metrics = append(metrics, Metric{
		Name:      "largest_file",
		Key:       a.largestFile,
		Value:     a.largestSize,
		Labels:    map[string]string{"size_bytes": fmt.Sprintf("%d", a.largestSize)},
		Timestamp: now,
	})

	for _, ext := range exts {
		metrics = append(metrics, Metric{
			Name:      "files_by_extension",
			Key:       ext,
			Value:     a.extensionMap[ext],
			Labels:    map[string]string{"extension": ext},
			Timestamp: now,
		})
	}

	return metrics
}

// Cleanup performs any necessary cleanup
func (a *FileAnalyser) Cleanup() error {
	// Cancel context to stop any running operations
	a.cancel()

	// Clear maps to help garbage collection
	a.fileStats = nil
	a.extensionMap = nil

	return nil
}
