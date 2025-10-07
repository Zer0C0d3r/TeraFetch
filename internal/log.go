package internal

import (
	"io"
	"os"
	"strings"
	"sync"
)

var (
	// Global logger instance
	globalLogger *SecureLogger
	loggerMutex  sync.RWMutex
)

// InitLogger initializes the global logger with the given configuration
func InitLogger(config *Config) error {
	loggerMutex.Lock()
	defer loggerMutex.Unlock()
	
	// Determine log level
	level := parseLogLevel(config.LogLevel)
	
	// Determine output destination
	var output io.Writer = os.Stderr
	if config.LogFile != "" {
		file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return NewValidationError("log_file", "failed to open log file").
				WithSuggestion("Check file permissions and path validity").
				WithContext("file", config.LogFile).
				WithContext("error", err.Error())
		}
		output = file
	}
	
	// Create the logger
	globalLogger = NewSecureLogger(output, level, config.EnableDebug, config.QuietMode)
	
	return nil
}

// GetLogger returns the global logger instance
func GetLogger() *SecureLogger {
	loggerMutex.RLock()
	defer loggerMutex.RUnlock()
	
	if globalLogger == nil {
		// Create a default logger if none exists
		globalLogger = NewDefaultLogger(false, false)
	}
	
	return globalLogger
}

// parseLogLevel converts string log level to LogLevel enum
func parseLogLevel(level string) LogLevel {
	switch strings.ToLower(level) {
	case "debug":
		return LogLevelDebug
	case "info":
		return LogLevelInfo
	case "warn", "warning":
		return LogLevelWarn
	case "error":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

// Convenience functions for global logging

// LogError logs an error message using the global logger
func LogError(format string, args ...interface{}) {
	GetLogger().Error(format, args...)
}

// LogWarn logs a warning message using the global logger
func LogWarn(format string, args ...interface{}) {
	GetLogger().Warn(format, args...)
}

// LogInfo logs an info message using the global logger
func LogInfo(format string, args ...interface{}) {
	GetLogger().Info(format, args...)
}

// LogDebug logs a debug message using the global logger
func LogDebug(format string, args ...interface{}) {
	GetLogger().Debug(format, args...)
}

// LogTeraboxError logs a TeraboxError with appropriate level and detail
func LogTeraboxError(err *TeraboxError) {
	logger := GetLogger()
	
	switch err.Severity {
	case SeverityCritical:
		logger.Error("CRITICAL: %s", err.DetailedError())
	case SeverityError:
		logger.Error("%s", err.DetailedError())
	case SeverityWarning:
		logger.Warn("%s", err.DetailedError())
	case SeverityInfo:
		logger.Info("%s", err.DetailedError())
	default:
		logger.Error("%s", err.DetailedError())
	}
}

// LogValidationError logs a ValidationError
func LogValidationError(err *ValidationError) {
	GetLogger().Error("Validation Error: %s", err.DetailedError())
}

// SetLogLevel updates the global logger's log level
func SetLogLevel(level LogLevel) {
	logger := GetLogger()
	logger.SetLevel(level)
}

// SetDebugMode enables or disables debug mode on the global logger
func SetDebugMode(debug bool) {
	logger := GetLogger()
	logger.SetDebug(debug)
}

// SetQuietMode enables or disables quiet mode on the global logger
func SetQuietMode(quiet bool) {
	logger := GetLogger()
	logger.SetQuiet(quiet)
}