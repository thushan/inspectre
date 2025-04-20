package storage

import (
	"context"
	"github.com/thushan/inspectre/internal/core/analyser"
)

// Storage represents a storage backend for analyser results
type Storage interface {
	// Initialize sets up the storage backend
	Initialize() error

	// StoreResults saves analyser results
	StoreResults(taskID string, repository string, results []*analyser.Result) error

	// QueryMetrics retrieves metrics based on a query
	QueryMetrics(query string) ([]map[string]interface{}, error)

	// GetMetricsByName retrieves metrics by name
	GetMetricsByName(name string, limit int) ([]map[string]interface{}, error)

	// GetMetricsByTask retrieves metrics for a specific task
	GetMetricsByTask(taskID string) ([]map[string]interface{}, error)

	// GetRepositorySummary gets summary data for a repository
	GetRepositorySummary(repository string) (map[string]interface{}, error)

	// Close cleans up resources
	Close() error
}

// MetricRecord represents a metric in the database
type MetricRecord struct {
	ID         string                 `json:"id"`
	TaskID     string                 `json:"task_id"`
	Repository string                 `json:"repository"`
	Analyser   string                 `json:"analyser"`
	Name       string                 `json:"name"`
	Key        string                 `json:"key,omitempty"`
	Value      interface{}            `json:"value"`
	Labels     map[string]string      `json:"labels,omitempty"`
	Timestamp  string                 `json:"timestamp"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ContextKey is used for context values
type ContextKey string

const (
	// CtxDuckDBConn is the context key for DuckDB connection
	CtxDuckDBConn ContextKey = "duckdb_conn"
)

// WithContext adds a context to the operation
type WithContext interface {
	WithContext(ctx context.Context) Storage
}
