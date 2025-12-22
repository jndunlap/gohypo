package internal

import (
	"log"
	"os"
)

// LogLevel represents different logging verbosity levels
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
	LogLevelTrace
)

// Logger provides leveled logging
type Logger struct {
	level LogLevel
}

// NewLogger creates a new logger with the specified level
func NewLogger(level LogLevel) *Logger {
	return &Logger{level: level}
}

// NewDefaultLogger creates a logger based on LOG_LEVEL environment variable
func NewDefaultLogger() *Logger {
	level := LogLevelInfo // default
	if levelStr := os.Getenv("LOG_LEVEL"); levelStr != "" {
		switch levelStr {
		case "ERROR":
			level = LogLevelError
		case "WARN":
			level = LogLevelWarn
		case "INFO":
			level = LogLevelInfo
		case "DEBUG":
			level = LogLevelDebug
		case "TRACE":
			level = LogLevelTrace
		}
	}
	return &Logger{level: level}
}

// Error logs error messages
func (l *Logger) Error(format string, args ...interface{}) {
	if l.level >= LogLevelError {
		log.Printf("[ERROR] "+format, args...)
	}
}

// Warn logs warning messages
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.level >= LogLevelWarn {
		log.Printf("[WARN] "+format, args...)
	}
}

// Info logs info messages
func (l *Logger) Info(format string, args ...interface{}) {
	if l.level >= LogLevelInfo {
		log.Printf("[INFO] "+format, args...)
	}
}

// Debug logs debug messages
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.level >= LogLevelDebug {
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Trace logs trace messages
func (l *Logger) Trace(format string, args ...interface{}) {
	if l.level >= LogLevelTrace {
		log.Printf("[TRACE] "+format, args...)
	}
}

// GetLevel returns the current log level
func (l *Logger) GetLevel() LogLevel {
	return l.level
}

// Global logger instance
var DefaultLogger = NewDefaultLogger()
