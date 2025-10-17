package rabbitmq

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// ConsoleLogger is a simple logger that outputs to stdout/stderr
// It's useful for debugging and development
type ConsoleLogger struct {
	level LogLevel
}

// LogLevel represents the logging level
type LogLevel int

const (
	// LogLevelDebug enables all log messages
	LogLevelDebug LogLevel = iota
	// LogLevelInfo enables info, warn, and error messages
	LogLevelInfo
	// LogLevelWarn enables warn and error messages
	LogLevelWarn
	// LogLevelError enables only error messages
	LogLevelError
)

// NewConsoleLogger creates a new console logger with the specified log level
func NewConsoleLogger(level LogLevel) Logger {
	return &ConsoleLogger{level: level}
}

// Debug logs a debug message with optional structured fields
func (c *ConsoleLogger) Debug(msg string, fields ...any) {
	if c.level <= LogLevelDebug {
		c.log("DEBUG", msg, fields...)
	}
}

// Info logs an informational message with optional structured fields
func (c *ConsoleLogger) Info(msg string, fields ...any) {
	if c.level <= LogLevelInfo {
		c.log("INFO", msg, fields...)
	}
}

// Warn logs a warning message with optional structured fields
func (c *ConsoleLogger) Warn(msg string, fields ...any) {
	if c.level <= LogLevelWarn {
		c.log("WARN", msg, fields...)
	}
}

// Error logs an error message with optional structured fields
func (c *ConsoleLogger) Error(msg string, fields ...any) {
	if c.level <= LogLevelError {
		c.log("ERROR", msg, fields...)
	}
}

// log formats and outputs the log message
func (c *ConsoleLogger) log(level, msg string, fields ...any) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	// Build the log message
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s: %s", timestamp, level, msg))

	// Add structured fields
	if len(fields) > 0 {
		sb.WriteString(" |")
		for i := 0; i < len(fields); i += 2 {
			if i+1 < len(fields) {
				key := fields[i]
				value := fields[i+1]
				sb.WriteString(fmt.Sprintf(" %v=%v", key, value))
			}
		}
	}

	log.Println(sb.String())
}
