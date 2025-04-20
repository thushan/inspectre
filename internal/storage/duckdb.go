package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	_ "github.com/marcboeker/go-duckdb/v2"
	"github.com/thushan/inspectre/internal/core/analyser"
	"github.com/thushan/inspectre/internal/core/logging"
)

// DuckDB-specific error definitions
var (
	ErrDatabaseInitFailed = errors.New("failed to initialize database")
	ErrInsertFailed       = errors.New("failed to insert data")
	ErrQueryFailed        = errors.New("query failed")
	ErrTransactionFailed  = errors.New("transaction failed")
	ErrNilConnection      = errors.New("database connection is nil")
	ErrParseValueFailed   = errors.New("failed to parse value")
)

// DuckDBStorage implements a DuckDB-based storage backend
type DuckDBStorage struct {
	dbPath   string
	db       *sql.DB
	mu       sync.Mutex
	logger   *logging.Logger
	ctx      context.Context
	cancelFn context.CancelFunc
}

// NewDuckDBStorage creates a new DuckDB storage backend
func NewDuckDBStorage(basePath string) *DuckDBStorage {
	dbPath := filepath.Join(basePath, "inspectre.db")
	ctx, cancel := context.WithCancel(context.Background())

	return &DuckDBStorage{
		dbPath:   dbPath,
		logger:   logging.GetLogger(),
		ctx:      ctx,
		cancelFn: cancel,
	}
}

// WithContext returns a storage implementation with the given context
func (s *DuckDBStorage) WithContext(ctx context.Context) Storage {
	// Create a new instance with the passed context
	newStorage := &DuckDBStorage{
		dbPath: s.dbPath,
		db:     s.db,
		logger: s.logger,
		ctx:    ctx,
	}

	return newStorage
}

// Initialize sets up the database
func (s *DuckDBStorage) Initialize() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.logger.Info("Initializing DuckDB storage at %s", s.dbPath)

	// Ensure directory exists
	if err := createDirIfNotExists(filepath.Dir(s.dbPath)); err != nil {
		return fmt.Errorf("%w: %v", ErrDatabaseInitFailed, err)
	}

	// Connect to DuckDB
	// Using read_only=false to allow writing to the database
	connStr := fmt.Sprintf("file:%s?read_only=false", s.dbPath)
	db, err := sql.Open("duckdb", connStr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabaseInitFailed, err)
	}

	// Set connection configuration
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("%w: %v", ErrDatabaseInitFailed, err)
	}

	s.db = db

	// Initialize schema
	if err := s.initializeSchema(); err != nil {
		s.db.Close()
		s.db = nil
		return fmt.Errorf("%w: %v", ErrDatabaseInitFailed, err)
	}

	s.logger.Info("DuckDB storage initialized successfully")
	return nil
}

// initializeSchema creates necessary tables if they don't exist
func (s *DuckDBStorage) initializeSchema() error {
	// Create metrics table
	if _, err := s.db.ExecContext(s.ctx, CreateMetricsTableSQL); err != nil {
		return fmt.Errorf("failed to create metrics table: %w", err)
	}

	// Create tasks table
	if _, err := s.db.ExecContext(s.ctx, CreateTasksTableSQL); err != nil {
		return fmt.Errorf("failed to create tasks table: %w", err)
	}

	// Create indexes
	if _, err := s.db.ExecContext(s.ctx, CreateMetricsIndexSQL); err != nil {
		return fmt.Errorf("failed to create metrics indexes: %w", err)
	}

	return nil
}

// StoreResults saves analyser results
func (s *DuckDBStorage) StoreResults(taskID string, repository string, results []*analyser.Result) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return ErrNilConnection
	}

	// Begin transaction
	tx, err := s.db.BeginTx(s.ctx, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}

	// Prepare to either commit or rollback the transaction
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Prepare the statement for inserting metrics
	stmt, err := tx.PrepareContext(s.ctx, InsertMetricSQL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInsertFailed, err)
	}
	defer stmt.Close()

	// Insert metrics
	metricsCount := 0
	for _, result := range results {
		// Skip if there are no metrics
		if len(result.Metrics) == 0 {
			continue
		}

		// Store each metric
		for _, metric := range result.Metrics {
			// Generate a unique ID for the metric
			metricID := uuid.New().String()

			// Convert labels to JSON if present
			var labelsJSON sql.NullString
			if len(metric.Labels) > 0 {
				labelsBytes, err := json.Marshal(metric.Labels)
				if err != nil {
					return fmt.Errorf("failed to marshal labels: %w", err)
				}
				labelsJSON = sql.NullString{String: string(labelsBytes), Valid: true}
			}

			// Convert value to JSON string and determine value type
			valueJSON, valueType, err := valueToJSONAndType(metric.Value)
			if err != nil {
				return fmt.Errorf("failed to process value for metric %s: %w", metric.Name, err)
			}

			// Set key if present
			var keyNull sql.NullString
			if metric.Key != "" {
				keyNull = sql.NullString{String: metric.Key, Valid: true}
			}

			// Insert the metric
			_, err = stmt.ExecContext(s.ctx,
				metricID,
				taskID,
				repository,
				result.AnalyserName,
				metric.Name,
				keyNull,
				valueJSON,
				valueType,
				labelsJSON,
				metric.Timestamp,
			)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrInsertFailed, err)
			}

			metricsCount++
		}
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("%w: %v", ErrTransactionFailed, err)
	}

	s.logger.Info("Stored %d metrics for task %s (repository: %s)", metricsCount, taskID, repository)
	return nil
}

// valueToJSONAndType converts a value to a JSON string and determines its type
func valueToJSONAndType(value interface{}) (string, string, error) {
	if value == nil {
		return "null", "null", nil
	}

	// Determine value type
	var valueType string
	switch reflect.TypeOf(value).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		valueType = "number"
	case reflect.Bool:
		valueType = "boolean"
	case reflect.String:
		valueType = "string"
	default:
		valueType = "object"
	}

	// Convert value to JSON
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrParseValueFailed, err)
	}

	return string(valueBytes), valueType, nil
}

// QueryMetrics retrieves metrics based on a query
func (s *DuckDBStorage) QueryMetrics(query string) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil, ErrNilConnection
	}

	// If query is empty, use a default query
	if strings.TrimSpace(query) == "" {
		query = "SELECT * FROM metrics ORDER BY timestamp DESC LIMIT 100"
	}

	// Validate the query to prevent SQL injection
	// This is a simple check - in a production system, you'd use proper SQL parsing
	query = strings.TrimSpace(query)
	if !strings.HasPrefix(strings.ToUpper(query), "SELECT") {
		return nil, fmt.Errorf("only SELECT queries are allowed")
	}

	// Query the database
	rows, err := s.db.QueryContext(s.ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	// Prepare result
	var results []map[string]interface{}

	// Iterate through rows
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row into valuePtrs
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
		}

		// Create a map for this row
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Process values from database to more usable formats
			switch v := val.(type) {
			case []byte:
				// Try to unmarshal JSON fields
				if col == "labels" || col == "value" {
					var jsonObj interface{}
					if err := json.Unmarshal(v, &jsonObj); err == nil {
						rowMap[col] = jsonObj
					} else {
						rowMap[col] = string(v)
					}
				} else {
					rowMap[col] = string(v)
				}
			default:
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	// Check for errors after iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	return results, nil
}

// GetMetricsByName retrieves metrics by name
func (s *DuckDBStorage) GetMetricsByName(name string, limit int) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil, ErrNilConnection
	}

	// Set default limit if not provided
	if limit <= 0 {
		limit = 100
	}

	// Query the database
	rows, err := s.db.QueryContext(s.ctx, GetMetricsByNameSQL, name, limit)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	defer rows.Close()

	// Process results
	results, err := processRows(rows)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetMetricsByTask retrieves metrics for a specific task
func (s *DuckDBStorage) GetMetricsByTask(taskID string) ([]map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil, ErrNilConnection
	}

	// Query the database
	rows, err := s.db.QueryContext(s.ctx, GetMetricsByTaskSQL, taskID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}
	defer rows.Close()

	// Process results
	results, err := processRows(rows)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// GetRepositorySummary gets summary data for a repository
func (s *DuckDBStorage) GetRepositorySummary(repository string) (map[string]interface{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db == nil {
		return nil, ErrNilConnection
	}

	// Query the database
	row := s.db.QueryRowContext(s.ctx, GetRepositorySummarySQL, repository)

	// Create a map for the result
	result := make(map[string]interface{})

	// Scan into variables
	var repoName string
	var taskCount int
	var lastAnalysis time.Time
	var metricTypes int

	if err := row.Scan(&repoName, &taskCount, &lastAnalysis, &metricTypes); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no data found for repository %s", repository)
		}
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	// Populate the result map
	result["repository"] = repoName
	result["task_count"] = taskCount
	result["last_analysis"] = lastAnalysis.Format(time.RFC3339)
	result["metric_types"] = metricTypes

	return result, nil
}

// processRows converts SQL rows to a slice of maps
func processRows(rows *sql.Rows) ([]map[string]interface{}, error) {
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	// Prepare result
	var results []map[string]interface{}

	// Iterate through rows
	for rows.Next() {
		// Create a slice of interface{} to hold the values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		// Scan the row into valuePtrs
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
		}

		// Create a map for this row
		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]

			// Process values from database to more usable formats
			switch v := val.(type) {
			case []byte:
				// Try to unmarshal JSON fields
				if col == "labels" || col == "value" {
					var jsonObj interface{}
					if err := json.Unmarshal(v, &jsonObj); err == nil {
						rowMap[col] = jsonObj
					} else {
						rowMap[col] = string(v)
					}
				} else {
					rowMap[col] = string(v)
				}
			default:
				rowMap[col] = val
			}
		}

		results = append(results, rowMap)
	}

	// Check for errors after iteration
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrQueryFailed, err)
	}

	return results, nil
}

// Close closes the database connection
func (s *DuckDBStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Cancel context
	s.cancelFn()

	if s.db != nil {
		if err := s.db.Close(); err != nil {
			return fmt.Errorf("failed to close database: %w", err)
		}
		s.db = nil
	}

	return nil
}

// Helper function to create a directory if it doesn't exist
func createDirIfNotExists(dir string) error {
	// Check if directory exists
	info, err := os.Stat(dir)
	if err == nil {
		// Path exists, check if it's a directory
		if info.IsDir() {
			return nil // Directory already exists
		}
		return fmt.Errorf("path exists but is not a directory: %s", dir)
	}

	// Create the directory with all parents
	if os.IsNotExist(err) {
		return os.MkdirAll(dir, 0755)
	}

	// Some other error occurred
	return fmt.Errorf("failed to check directory: %w", err)
}

// Ensure DuckDBStorage implements Storage
var _ Storage = (*DuckDBStorage)(nil)

// init registers the duckdb driver
func init() {
	// Register DuckDB driver
	// sql.Register("duckdb", duckdb.NewDriver())
}
