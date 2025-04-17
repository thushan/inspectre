package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
		if ext == "" {
			ext = "(none)"
		}
		a.extensionMap[ext]++

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("file analysis failed: %w", err)
	}

	now := time.Now()
	nExt := len(a.extensionMap)
	metricTotals := 3

	exts := make([]string, 0, nExt)
	for ext := range a.extensionMap {
		exts = append(exts, ext)
	}
	sort.Strings(exts)

	metrics := make([]Metric, metricTotals+nExt)

	metrics[0] = Metric{
		Name:      "total_files",
		Value:     a.totalFiles,
		Timestamp: now,
	}
	metrics[1] = Metric{
		Name:      "total_size_bytes",
		Value:     a.totalSize,
		Timestamp: now,
	}
	metrics[2] = Metric{
		Name:      "largest_file",
		Key:       a.largestFile,
		Value:     a.largestSize,
		Labels:    map[string]string{"size_bytes": fmt.Sprintf("%d", a.largestSize)},
		Timestamp: now,
	}

	for i, ext := range exts {
		metrics[i+metricTotals] = Metric{
			Name:      "files_by_extension",
			Key:       ext,
			Value:     a.extensionMap[ext],
			Labels:    map[string]string{"extension": ext},
			Timestamp: now,
		}
	}

	return metrics, nil
}

// Cleanup performs any necessary cleanup
func (a *FileAnalyser) Cleanup() error {
	// No cleanup needed for this analyser
	return nil
}
