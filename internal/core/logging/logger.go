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
	DefaultLogQueueSize = 100
	DefaultLogLevel     = LevelInfo
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
	mu               sync.RWMutex
	level            LogLevel
	writers          map[string]io.Writer
	defaultWriter    io.Writer
	queue            chan logMessage
	wg               sync.WaitGroup
	format           OutputFormat
	taskWriters      map[string]io.Writer
	taskWritersMutex sync.RWMutex
	filePerm         os.FileMode
}

// logMessage represents a message to be logged
type logMessage struct {
	level     LogLevel
	message   string
	taskID    string
	timestamp time.Time
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
			queue:         make(chan logMessage, DefaultLogQueueSize),
			format:        FormatColored,
			taskWriters:   make(map[string]io.Writer),
			filePerm:      0644,
		}

		// Start background worker
		go instance.processLogs()
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
	l.taskWritersMutex.Lock()
	defer l.taskWritersMutex.Unlock()

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
	l.taskWritersMutex.Lock()
	defer l.taskWritersMutex.Unlock()

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

// log sends a message to the log queue
func (l *Logger) log(level LogLevel, taskID, format string, args ...interface{}) {
	l.mu.RLock()
	currentLevel := l.level
	l.mu.RUnlock()

	if level < currentLevel {
		return
	}

	msg := fmt.Sprintf(format, args...)

	l.wg.Add(1)
	select {
	case l.queue <- logMessage{level: level, message: msg, taskID: taskID, timestamp: time.Now()}:
	default:
		// If queue is full, log directly
		l.writeLogMessage(logMessage{level: level, message: msg, taskID: taskID, timestamp: time.Now()})
		l.wg.Done()
	}
}

// Debug logs debug messages
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(LevelDebug, "", format, args...)
}

// Info logs informational messages
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(LevelInfo, "", format, args...)
}

// Warning logs warning messages
func (l *Logger) Warning(format string, args ...interface{}) {
	l.log(LevelWarning, "", format, args...)
}

// Error logs error messages
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(LevelError, "", format, args...)
}

// TaskDebug logs debug messages for a task
func (l *Logger) TaskDebug(taskID, format string, args ...interface{}) {
	l.log(LevelDebug, taskID, format, args...)
}

// TaskInfo logs informational messages for a task
func (l *Logger) TaskInfo(taskID, format string, args ...interface{}) {
	l.log(LevelInfo, taskID, format, args...)
}

// TaskWarning logs warning messages for a task
func (l *Logger) TaskWarning(taskID, format string, args ...interface{}) {
	l.log(LevelWarning, taskID, format, args...)
}

// TaskError logs error messages for a task
func (l *Logger) TaskError(taskID, format string, args ...interface{}) {
	l.log(LevelError, taskID, format, args...)
}

// processLogs processes log messages from the queue
func (l *Logger) processLogs() {
	for msg := range l.queue {
		l.writeLogMessage(msg)
		l.wg.Done()
	}
}

// writeLogMessage writes a log message to all registered writers
func (l *Logger) writeLogMessage(msg logMessage) {
	formatted := l.formatLogMessage(msg)

	// Write to task log if specified
	if msg.taskID != "" {
		l.taskWritersMutex.RLock()
		if writer, ok := l.taskWriters[msg.taskID]; ok {
			_, _ = fmt.Fprintln(writer, formatted)
		}
		l.taskWritersMutex.RUnlock()
	}

	// Write to default writer
	l.mu.RLock()
	_, _ = fmt.Fprintln(l.defaultWriter, formatted)

	// Write to all registered writers
	for _, w := range l.writers {
		_, _ = fmt.Fprintln(w, formatted)
	}
	l.mu.RUnlock()
}

// formatLogMessage formats a log message according to the configured format
func (l *Logger) formatLogMessage(msg logMessage) string {
	l.mu.RLock()
	format := l.format
	l.mu.RUnlock()

	timestamp := msg.timestamp.Format("15:04:05.000")

	// Format the message according to the configured format
	switch format {
	case FormatJSON:
		// Simplified JSON format for example
		taskStr := ""
		if msg.taskID != "" {
			taskStr = fmt.Sprintf(", \"task_id\": \"%s\"", msg.taskID)
		}
		return fmt.Sprintf("{\"time\": \"%s\", \"level\": \"%s\", \"message\": \"%s\"%s}",
			msg.timestamp.Format(time.RFC3339), l.levelToString(msg.level), msg.message, taskStr)

	case FormatColored:
		prefix := ""
		if msg.taskID != "" {
			prefix = fmt.Sprintf("[%s] ", msg.taskID)
		}

		switch msg.level {
		case LevelDebug:
			return fmt.Sprintf("%s %s%s", pterm.Gray(timestamp), prefix, msg.message)
		case LevelInfo:
			return fmt.Sprintf("%s %s%s", pterm.FgBlue.Sprint(timestamp), prefix, msg.message)
		case LevelWarning:
			return fmt.Sprintf("%s %s%s", pterm.FgYellow.Sprint(timestamp), prefix, pterm.Yellow(msg.message))
		case LevelError:
			return fmt.Sprintf("%s %s%s", pterm.FgRed.Sprint(timestamp), prefix, pterm.Red(msg.message))
		default:
			return fmt.Sprintf("%s %s%s", timestamp, prefix, msg.message)
		}

	default: // FormatPlain
		prefix := ""
		if msg.taskID != "" {
			prefix = fmt.Sprintf("[%s] ", msg.taskID)
		}
		levelStr := l.levelToString(msg.level)
		return fmt.Sprintf("%s [%s] %s%s", timestamp, levelStr, prefix, msg.message)
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

// Flush waits for all log messages to be processed
func (l *Logger) Flush() {
	l.wg.Wait()
}

// Close shuts down the logger
func (l *Logger) Close() {
	close(l.queue)
	l.Flush()

	// Close all writers
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, w := range l.writers {
		if closer, ok := w.(io.Closer); ok {
			_ = closer.Close()
		}
	}

	// Close all task writers
	l.taskWritersMutex.Lock()
	defer l.taskWritersMutex.Unlock()

	for _, w := range l.taskWriters {
		if closer, ok := w.(io.Closer); ok {
			_ = closer.Close()
		}
	}
}
