package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
)

// FileStorage errors
var (
	ErrFileWriteFailed   = errors.New("failed to write file")
	ErrFileReadFailed    = errors.New("failed to read file")
	ErrInvalidQuery      = errors.New("invalid query")
	ErrQueryNotSupported = errors.New("complex queries not supported in file storage")
)

// FileStorage implements a simple file-based storage backend for testing
type FileStorage struct {
	basePath string
	mu       sync.Mutex
	logger   *logging.Logger
}

// NewFileStorage creates a new file-based storage backend
func NewFileStorage(basePath string) *FileStorage {
	return &FileStorage{
		basePath: basePath,
		logger:   logging.GetLogger(),
	}
}

// WithContext returns a storage implementation with the given context
func (s *FileStorage) WithContext(ctx context.Context) Storage {
	// Context isn't really used in file storage, so we just return the same instance
	return s
}

// Initialize sets up the storage directory
func (s *FileStorage) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure base directory exists
	resultsDir := filepath.Join(s.basePath, "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create results directory: %w", err)
	}

	return nil
}

// StoreResults saves analyser results to JSON files
func (s *FileStorage) StoreResults(taskID string, repository string, results []*analyser.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a results directory for the task
	resultsDir := filepath.Join(s.basePath, "results", taskID)
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return fmt.Errorf("failed to create task directory: %w", err)
	}

	// Create a summary file with basic info
	summaryPath := filepath.Join(resultsDir, "summary.json")
	summary := map[string]interface{}{
		"task_id":      taskID,
		"repository":   repository,
		"stored_at":    time.Now().Format(time.RFC3339),
		"result_count": len(results),
	}

	// Store summary
	summaryJSON, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	if err := os.WriteFile(summaryPath, summaryJSON, 0644); err != nil {
		return fmt.Errorf("%w: %v", ErrFileWriteFailed, err)
	}

	// Store each result in a separate file
	for i, result := range results {
		resultPath := filepath.Join(resultsDir, fmt.Sprintf("result_%d.json", i))
		resultJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal result: %w", err)
		}

		if err := os.WriteFile(resultPath, resultJSON, 0644); err != nil {
			return fmt.Errorf("%w: %v", ErrFileWriteFailed, err)
		}
	}

	// Store metrics in a separate file for each result
	metricsDir := filepath.Join(resultsDir, "metrics")
	if err := os.MkdirAll(metricsDir, 0755); err != nil {
		return fmt.Errorf("failed to create metrics directory: %w", err)
	}

	for i, result := range results {
		for j, metric := range result.Metrics {
			// Generate unique ID for the metric
			metricID := uuid.New().String()

			// Create a record for the metric
			record := MetricRecord{
				ID:         metricID,
				TaskID:     taskID,
				Repository: repository,
				Analyser:   result.AnalyserName,
				Name:       metric.Name,
				Key:        metric.Key,
				Value:      metric.Value,
				Labels:     metric.Labels,
				Timestamp:  metric.Timestamp.Format(time.RFC3339),
				Metadata: map[string]interface{}{
					"result_index": i,
					"metric_index": j,
				},
			}

			// Create filename based on metric name
			safeMetricName := strings.ReplaceAll(metric.Name, "/", "_")
			metricPath := filepath.Join(metricsDir, fmt.Sprintf("%s_%s.json", safeMetricName, metricID[:8]))

			// Store metric
			metricJSON, err := json.MarshalIndent(record, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal metric: %w", err)
			}

			if err := os.WriteFile(metricPath, metricJSON, 0644); err != nil {
				return fmt.Errorf("%w: %v", ErrFileWriteFailed, err)
			}
		}
	}

	return nil
}

// QueryMetrics retrieves metrics based on a query
func (s *FileStorage) QueryMetrics(query string) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// For simple file storage, we'll implement a basic query language
	// Format: name=value,name2=value2
	if query == "" {
		// Default to returning all metrics
		return s.getAllMetrics()
	}

	// Very simple query parser
	filters := make(map[string]string)
	for _, part := range strings.Split(query, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			filters[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	// Apply filters to all metrics
	return s.getFilteredMetrics(filters)
}

// getAllMetrics returns all metrics from all tasks
func (s *FileStorage) getAllMetrics() ([]map[string]interface{}, error) {
	resultsDir := filepath.Join(s.basePath, "results")
	if _, err := os.Stat(resultsDir); os.IsNotExist(err) {
		return []map[string]interface{}{}, nil
	}

	var allMetrics []map[string]interface{}

	// List all task directories
	taskDirs, err := os.ReadDir(resultsDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFileReadFailed, err)
	}

	// Process each task directory
	for _, taskDir := range taskDirs {
		if !taskDir.IsDir() {
			continue
		}

		// Check for metrics directory
		metricsDir := filepath.Join(resultsDir, taskDir.Name(), "metrics")
		if _, err := os.Stat(metricsDir); os.IsNotExist(err) {
			continue
		}

		// Read all metric files
		metricFiles, err := os.ReadDir(metricsDir)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFileReadFailed, err)
		}

		// Process each metric file
		for _, metricFile := range metricFiles {
			if metricFile.IsDir() || !strings.HasSuffix(metricFile.Name(), ".json") {
				continue
			}

			// Read and parse metric file
			metricPath := filepath.Join(metricsDir, metricFile.Name())
			metricData, err := os.ReadFile(metricPath)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrFileReadFailed, err)
			}

			var metric map[string]interface{}
			if err := json.Unmarshal(metricData, &metric); err != nil {
				s.logger.Warning("Skipping invalid metric file %s: %v", metricPath, err)
				continue
			}

			allMetrics = append(allMetrics, metric)
		}
	}

	return allMetrics, nil
}

// getFilteredMetrics returns metrics matching the specified filters
func (s *FileStorage) getFilteredMetrics(filters map[string]string) ([]map[string]interface{}, error) {
	allMetrics, err := s.getAllMetrics()
	if err != nil {
		return nil, err
	}

	var filteredMetrics []map[string]interface{}

	// Apply filters
	for _, metric := range allMetrics {
		match := true
		for key, value := range filters {
			metricValue, exists := metric[key]
			if !exists || fmt.Sprintf("%v", metricValue) != value {
				match = false
				break
			}
		}

		if match {
			filteredMetrics = append(filteredMetrics, metric)
		}
	}

	return filteredMetrics, nil
}

// GetMetricsByName retrieves metrics by name
func (s *FileStorage) GetMetricsByName(name string, limit int) ([]map[string]interface{}, error) {
	filters := map[string]string{"name": name}
	metrics, err := s.getFilteredMetrics(filters)
	if err != nil {
		return nil, err
	}

	// Apply limit if specified
	if limit > 0 && len(metrics) > limit {
		metrics = metrics[:limit]
	}

	return metrics, nil
}

// GetMetricsByTask retrieves metrics for a specific task
func (s *FileStorage) GetMetricsByTask(taskID string) ([]map[string]interface{}, error) {
	filters := map[string]string{"task_id": taskID}
	return s.getFilteredMetrics(filters)
}

// GetRepositorySummary gets summary data for a repository
func (s *FileStorage) GetRepositorySummary(repository string) (map[string]interface{}, error) {
	filters := map[string]string{"repository": repository}
	metrics, err := s.getFilteredMetrics(filters)
	if err != nil {
		return nil, err
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("no data found for repository %s", repository)
	}

	// Count unique tasks
	tasks := make(map[string]bool)
	metricTypes := make(map[string]bool)
	var lastTimestamp time.Time

	for _, metric := range metrics {
		if taskID, ok := metric["task_id"].(string); ok {
			tasks[taskID] = true
		}

		if name, ok := metric["name"].(string); ok {
			metricTypes[name] = true
		}

		if timestamp, ok := metric["timestamp"].(string); ok {
			t, err := time.Parse(time.RFC3339, timestamp)
			if err == nil && t.After(lastTimestamp) {
				lastTimestamp = t
			}
		}
	}

	summary := map[string]interface{}{
		"repository":    repository,
		"task_count":    len(tasks),
		"metric_types":  len(metricTypes),
		"last_analysis": lastTimestamp.Format(time.RFC3339),
	}

	return summary, nil
}

// Close cleans up resources
func (s *FileStorage) Close() error {
	// Nothing to close for file storage
	return nil
}
