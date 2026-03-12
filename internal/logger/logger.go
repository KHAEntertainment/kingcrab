package logger

import (
	"encoding/json"
	"os"
	"time"
)

// Level represents log severity
type Level string

const (
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Logger is a simple JSON logger
type Logger struct {
	output *os.File
}

// New creates a new logger writing to stdout
func New() *Logger {
	return &Logger{
		output: os.Stdout,
	}
}

// LogEntry represents a single log entry
type LogEntry struct {
	Timestamp  string `json:"timestamp"`
	Level      string `json:"level"`
	Message    string `json:"message"`
	RequestID  string `json:"request_id,omitempty"`
	Command    string `json:"command,omitempty"`
	Status     string `json:"status,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
}

// Log writes a log entry
func (l *Logger) Log(level Level, message string, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     string(level),
		Message:   message,
	}

	if v, ok := fields["request_id"].(string); ok && v != "" {
		entry.RequestID = v
	}
	if v, ok := fields["command"].(string); ok && v != "" {
		entry.Command = v
	}
	if v, ok := fields["status"].(string); ok && v != "" {
		entry.Status = v
	}
	if v, ok := fields["duration_ms"].(int64); ok && v > 0 {
		entry.DurationMs = v
	}

	jsonBytes, _ := json.Marshal(entry)
	l.output.Write(append(jsonBytes, '\n'))
}

// Info logs an info message
func (l *Logger) Info(message string, fields map[string]interface{}) {
	l.Log(LevelInfo, message, fields)
}

// Warn logs a warning message
func (l *Logger) Warn(message string, fields map[string]interface{}) {
	l.Log(LevelWarn, message, fields)
}

// Error logs an error message
func (l *Logger) Error(message string, fields map[string]interface{}) {
	l.Log(LevelError, message, fields)
}

// Global logger instance
var defaultLogger = New()

// Info is a convenience function using the default logger
func Info(message string, fields map[string]interface{}) {
	defaultLogger.Info(message, fields)
}

// Warn is a convenience function using the default logger
func Warn(message string, fields map[string]interface{}) {
	defaultLogger.Warn(message, fields)
}

// Error is a convenience function using the default logger
func Error(message string, fields map[string]interface{}) {
	defaultLogger.Error(message, fields)
}
