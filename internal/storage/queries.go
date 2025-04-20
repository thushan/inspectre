package storage

// SQL queries for DuckDB storage
const (
	// Schema creation
	CreateMetricsTableSQL = `
		CREATE TABLE IF NOT EXISTS metrics (
			id VARCHAR PRIMARY KEY,
			task_id VARCHAR NOT NULL,
			repository VARCHAR NOT NULL,
			analyser VARCHAR NOT NULL,
			name VARCHAR NOT NULL,
			key VARCHAR,
			value VARCHAR NOT NULL,  -- Stored as JSON string
			value_type VARCHAR NOT NULL, -- Type of the value (string, number, bool)
			labels VARCHAR,          -- Stored as JSON string
			timestamp TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	CreateMetricsIndexSQL = `
		CREATE INDEX IF NOT EXISTS idx_metrics_task_id ON metrics(task_id);
		CREATE INDEX IF NOT EXISTS idx_metrics_repository ON metrics(repository);
		CREATE INDEX IF NOT EXISTS idx_metrics_name ON metrics(name);
		CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON metrics(timestamp);
	`

	CreateTasksTableSQL = `
		CREATE TABLE IF NOT EXISTS tasks (
			id VARCHAR PRIMARY KEY,
			repository VARCHAR NOT NULL,
			status VARCHAR NOT NULL,
			start_time TIMESTAMP NOT NULL,
			end_time TIMESTAMP,
			error VARCHAR,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`

	// Insertion queries
	InsertMetricSQL = `
		INSERT INTO metrics (id, task_id, repository, analyser, name, key, value, value_type, labels, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
	`

	InsertTaskSQL = `
		INSERT INTO tasks (id, repository, status, start_time, end_time, error)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			status = excluded.status,
			end_time = excluded.end_time,
			error = excluded.error;
	`

	// Get metrics by name
	GetMetricsByNameSQL = `
		SELECT id, task_id, repository, analyser, name, key, value, labels, timestamp
		FROM metrics
		WHERE name = ?
		ORDER BY timestamp DESC
		LIMIT ?;
	`

	// Get metrics by task
	GetMetricsByTaskSQL = `
		SELECT id, task_id, repository, analyser, name, key, value, labels, timestamp
		FROM metrics
		WHERE task_id = ?
		ORDER BY name, timestamp;
	`

	// Get repository summary
	GetRepositorySummarySQL = `
		SELECT 
			repository,
			COUNT(DISTINCT task_id) AS task_count,
			MAX(timestamp) AS last_analysis,
			COUNT(DISTINCT name) AS metric_types
		FROM metrics
		WHERE repository = ?
		GROUP BY repository;
	`

	// Get task details
	GetTaskDetailsSQL = `
		SELECT id, repository, status, start_time, end_time, error
		FROM tasks
		WHERE id = ?;
	`

	// Get task metrics summary
	GetTaskMetricsSummarySQL = `
		SELECT 
			name, 
			COUNT(*) AS count,
			MIN(CAST(value AS FLOAT)) AS min_value,
			MAX(CAST(value AS FLOAT)) AS max_value,
			AVG(CAST(value AS FLOAT)) AS avg_value
		FROM metrics
		WHERE task_id = ? AND value_type = 'number'
		GROUP BY name
		ORDER BY name;
	`
)
