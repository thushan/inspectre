package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/pterm/pterm"
)

// LogLevel represents the severity of a log message
type LogLevel int

// Log levels
const (
	LevelDebug LogLevel = iota
	LevelInfo
	LevelWarning
	LevelError
	LevelNone // Special level to disable logging
)

// Default configuration
const (
	DefaultLogLevel = LevelInfo
)

// OutputFormat defines how log messages are formatted
type OutputFormat int

const (
	FormatPlain   OutputFormat = iota // Simple text output
	FormatColored                     // Colored output for console
	FormatJSON                        // JSON format for machine parsing
)

// Logger manages logging operations
type Logger struct {
	mu            sync.RWMutex
	level         LogLevel
	writers       map[string]io.Writer
	defaultWriter io.Writer
	format        OutputFormat
	taskWriters   map[string]io.Writer
	taskMu        sync.RWMutex
	filePerm      os.FileMode
	closed        bool // Flag to track if the logger is closed
}

var (
	instance *Logger
	once     sync.Once
)

// GetLogger returns the singleton logger instance
func GetLogger() *Logger {
	once.Do(func() {
		instance = &Logger{
			level:         DefaultLogLevel,
			writers:       make(map[string]io.Writer),
			defaultWriter: os.Stdout,
			format:        FormatColored,
			taskWriters:   make(map[string]io.Writer),
			filePerm:      0644,
			closed:        false,
		}
	})

	return instance
}

// SetLogLevel sets the minimum log level
func (l *Logger) SetLogLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutputFormat sets the log output format
func (l *Logger) SetOutputFormat(format OutputFormat) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.format = format
}

// SetDefaultWriter sets the default output writer
func (l *Logger) SetDefaultWriter(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.defaultWriter = w
}

// SetFilePermissions sets the permissions for created log files
func (l *Logger) SetFilePermissions(perm os.FileMode) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.filePerm = perm
}

// AddWriter adds a named writer
func (l *Logger) AddWriter(name string, w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.writers[name] = w
}

// RemoveWriter removes a named writer
func (l *Logger) RemoveWriter(name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if w, ok := l.writers[name]; ok {
		if closer, ok := w.(io.Closer); ok {
			_ = closer.Close()
		}
		delete(l.writers, name)
	}
}

// RegisterTaskLog creates a new log file for a task
func (l *Logger) RegisterTaskLog(taskID, logPath string) error {
	l.taskMu.Lock()
	defer l.taskMu.Unlock()

	// Check if logger is closed
	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return fmt.Errorf("logger is closed")
	}
	l.mu.RUnlock()

	// Close any existing writer
	if w, exists := l.taskWriters[taskID]; exists {
		if closer, ok := w.(io.Closer); ok {
			_ = closer.Close()
		}
	}

	// Create directory if needed
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, l.filePerm)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.taskWriters[taskID] = f
	return nil
}

// CloseTaskLog closes a task's log file
func (l *Logger) CloseTaskLog(taskID string) {
	l.taskMu.Lock()
	defer l.taskMu.Unlock()

	if w, exists := l.taskWriters[taskID]; exists {
		if closer, ok := w.(io.Closer); ok {
			_ = closer.Close()
		}
		delete(l.taskWriters, taskID)
	}
}

// GetTaskLogReader opens a task's log file for reading
func (l *Logger) GetTaskLogReader(taskID string, logPath string) (io.ReadCloser, error) {
	// We don't need to track readers, just open the file
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	return f, nil
}

// writeLogMessage writes a log message directly to all writers
func (l *Logger) writeLogMessage(level LogLevel, taskID, format string, args ...interface{}) {
	l.mu.RLock()
	currentLevel := l.level
	isLoggerClosed := l.closed
	l.mu.RUnlock()

	// Skip if logger is closed or level is too low
	if isLoggerClosed || level < currentLevel {
		return
	}

	// Format the message
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now()
	formatted := l.formatLogMessage(level, taskID, timestamp, msg)

	// Write to task log if specified
	if taskID != "" {
		l.taskMu.RLock()
		if writer, ok := l.taskWriters[taskID]; ok {
			_, _ = fmt.Fprintln(writer, formatted)
		}
		l.taskMu.RUnlock()
	}

	// Write to default writer and all registered writers
	l.mu.RLock()
	_, _ = fmt.Fprintln(l.defaultWriter, formatted)
	for _, w := range l.writers {
		_, _ = fmt.Fprintln(w, formatted)
	}
	l.mu.RUnlock()
}

// formatLogMessage formats a log message according to the configured format
func (l *Logger) formatLogMessage(level LogLevel, taskID string, timestamp time.Time, message string) string {
	l.mu.RLock()
	format := l.format
	l.mu.RUnlock()

	timeStr := timestamp.Format("15:04:05.000")

	// Format the message according to the configured format
	switch format {
	case FormatJSON:
		// Simplified JSON format
		taskStr := ""
		if taskID != "" {
			taskStr = fmt.Sprintf(", \"task_id\": \"%s\"", taskID)
		}
		return fmt.Sprintf("{\"time\": \"%s\", \"level\": \"%s\", \"message\": \"%s\"%s}",
			timestamp.Format(time.RFC3339), l.levelToString(level), message, taskStr)

	case FormatColored:
		prefix := ""
		if taskID != "" {
			prefix = fmt.Sprintf("[%s] ", taskID)
		}

		switch level {
		case LevelDebug:
			return fmt.Sprintf("%s %s%s", pterm.Gray(timeStr), prefix, message)
		case LevelInfo:
			return fmt.Sprintf("%s %s%s", pterm.FgBlue.Sprint(timeStr), prefix, message)
		case LevelWarning:
			return fmt.Sprintf("%s %s%s", pterm.FgYellow.Sprint(timeStr), prefix, pterm.Yellow(message))
		case LevelError:
			return fmt.Sprintf("%s %s%s", pterm.FgRed.Sprint(timeStr), prefix, pterm.Red(message))
		default:
			return fmt.Sprintf("%s %s%s", timeStr, prefix, message)
		}

	default: // FormatPlain
		prefix := ""
		if taskID != "" {
			prefix = fmt.Sprintf("[%s] ", taskID)
		}
		levelStr := l.levelToString(level)
		return fmt.Sprintf("%s [%s] %s%s", timeStr, levelStr, prefix, message)
	}
}

// levelToString converts a LogLevel to its string representation
func (l *Logger) levelToString(level LogLevel) string {
	switch level {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// Debug logs debug messages
func (l *Logger) Debug(format string, args ...interface{}) {
	l.writeLogMessage(LevelDebug, "", format, args...)
}

// Info logs informational messages
func (l *Logger) Info(format string, args ...interface{}) {
	l.writeLogMessage(LevelInfo, "", format, args...)
}

// Warning logs warning messages
func (l *Logger) Warning(format string, args ...interface{}) {
	l.writeLogMessage(LevelWarning, "", format, args...)
}

// Error logs error messages
func (l *Logger) Error(format string, args ...interface{}) {
	l.writeLogMessage(LevelError, "", format, args...)
}

// TaskDebug logs debug messages for a task
func (l *Logger) TaskDebug(taskID, format string, args ...interface{}) {
	l.writeLogMessage(LevelDebug, taskID, format, args...)
}

// TaskInfo logs informational messages for a task
func (l *Logger) TaskInfo(taskID, format string, args ...interface{}) {
	l.writeLogMessage(LevelInfo, taskID, format, args...)
}

// TaskWarning logs warning messages for a task
func (l *Logger) TaskWarning(taskID, format string, args ...interface{}) {
	l.writeLogMessage(LevelWarning, taskID, format, args...)
}

// TaskError logs error messages for a task
func (l *Logger) TaskError(taskID, format string, args ...interface{}) {
	l.writeLogMessage(LevelError, taskID, format, args...)
}

// Flush writes all pending log messages
func (l *Logger) Flush() {
	// Direct writing approach doesn't need flushing
	// This is just a compatibility method
}

// Close shuts down the logger
func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Mark as closed first to prevent new writes
	l.closed = true

	// Close all writers
	for _, w := range l.writers {
		if closer, ok := w.(io.Closer); ok {
			_ = closer.Close()
		}
	}

	// Clear the writers map
	l.writers = make(map[string]io.Writer)

	// Close all task writers
	l.taskMu.Lock()
	defer l.taskMu.Unlock()

	for _, w := range l.taskWriters {
		if closer, ok := w.(io.Closer); ok {
			_ = closer.Close()
		}
	}

	// Clear the task writers map
	l.taskWriters = make(map[string]io.Writer)
}
