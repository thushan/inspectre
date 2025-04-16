package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
}

// NewFileAnalyser creates a new file analyser
func NewFileAnalyser() *FileAnalyser {
	return &FileAnalyser{
		excludeDirs:  []string{".git", "node_modules", "vendor", ".idea", ".vscode"},
		fileStats:    make(map[string]int),
		extensionMap: make(map[string]int),
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

// Run performs the analysis
func (a *FileAnalyser) Run() ([]Metric, error) {
	// Walk the repository and gather file information
	err := filepath.Walk(a.repoPath, func(path string, info os.FileInfo, err error) error {
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
		a.totalFiles++
		a.totalSize += info.Size()

		// Track largest file
		if info.Size() > a.largestSize {
			a.largestSize = info.Size()
			a.largestFile, _ = filepath.Rel(a.repoPath, path)
		}

		// Get file extension
		ext := strings.ToLower(filepath.Ext(path))
		if ext == "" {
			ext = "(no extension)"
		}
		a.extensionMap[ext]++

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("file analysis failed: %w", err)
	}

	// Prepare metrics
	var metrics []Metric
	now := time.Now()

	// Add total counts
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
		Value:     a.largestFile,
		Labels:    map[string]string{"size_bytes": fmt.Sprintf("%d", a.largestSize)},
		Timestamp: now,
	})

	// Add extension stats
	for ext, count := range a.extensionMap {
		metrics = append(metrics, Metric{
			Name:      "files_by_extension",
			Value:     count,
			Labels:    map[string]string{"extension": ext},
			Timestamp: now,
		})
	}

	return metrics, nil
}

// Cleanup performs any necessary cleanup
func (a *FileAnalyser) Cleanup() error {
	// No cleanup needed for this analyser
	return nil
}
