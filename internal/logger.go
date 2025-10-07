package internal

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

// LogLevel represents different logging levels
type LogLevel int

const (
	LogLevelError LogLevel = iota
	LogLevelWarn
	LogLevelInfo
	LogLevelDebug
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case LogLevelError:
		return "ERROR"
	case LogLevelWarn:
		return "WARN"
	case LogLevelInfo:
		return "INFO"
	case LogLevelDebug:
		return "DEBUG"
	default:
		return "UNKNOWN"
	}
}

// SecureLogger provides secure logging with sensitive data redaction
type SecureLogger struct {
	logger    *log.Logger
	level     LogLevel
	debug     bool
	quiet     bool
	redactors []Redactor
}

// Redactor defines an interface for redacting sensitive information
type Redactor interface {
	Redact(input string) string
}

// CookieRedactor redacts cookie values from strings
type CookieRedactor struct{}

func (r *CookieRedactor) Redact(input string) string {
	// Redact common cookie patterns
	patterns := []string{
		"BDUSS=",
		"STOKEN=",
		"Cookie:",
		"Set-Cookie:",
		"Authorization:",
		"Bearer ",
	}
	
	result := input
	for _, pattern := range patterns {
		if strings.Contains(strings.ToLower(result), strings.ToLower(pattern)) {
			// Find the pattern and redact everything after it until whitespace or semicolon
			lower := strings.ToLower(result)
			index := strings.Index(lower, strings.ToLower(pattern))
			if index != -1 {
				start := index + len(pattern)
				end := start
				for end < len(result) && result[end] != ' ' && result[end] != ';' && result[end] != '\n' && result[end] != '\r' {
					end++
				}
				if end > start {
					result = result[:start] + "[REDACTED]" + result[end:]
				}
			}
		}
	}
	return result
}

// URLRedactor redacts sensitive URL parameters
type URLRedactor struct{}

func (r *URLRedactor) Redact(input string) string {
	// Redact sensitive URL parameters
	sensitiveParams := []string{
		"access_token=",
		"token=",
		"key=",
		"secret=",
		"password=",
		"pwd=",
	}
	
	result := input
	for _, param := range sensitiveParams {
		if strings.Contains(strings.ToLower(result), param) {
			lower := strings.ToLower(result)
			index := strings.Index(lower, param)
			if index != -1 {
				start := index + len(param)
				end := start
				for end < len(result) && result[end] != '&' && result[end] != ' ' && result[end] != '\n' {
					end++
				}
				if end > start {
					result = result[:start] + "[REDACTED]" + result[end:]
				}
			}
		}
	}
	return result
}

// NewSecureLogger creates a new secure logger
func NewSecureLogger(output io.Writer, level LogLevel, debug, quiet bool) *SecureLogger {
	logger := log.New(output, "", 0) // We'll handle our own formatting
	
	sl := &SecureLogger{
		logger: logger,
		level:  level,
		debug:  debug,
		quiet:  quiet,
		redactors: []Redactor{
			&CookieRedactor{},
			&URLRedactor{},
		},
	}
	
	return sl
}

// NewDefaultLogger creates a logger with default settings
func NewDefaultLogger(debug, quiet bool) *SecureLogger {
	level := LogLevelInfo
	if debug {
		level = LogLevelDebug
	}
	if quiet {
		level = LogLevelError
	}
	
	return NewSecureLogger(os.Stderr, level, debug, quiet)
}

// redactSensitiveData applies all redactors to the input string
func (sl *SecureLogger) redactSensitiveData(input string) string {
	result := input
	for _, redactor := range sl.redactors {
		result = redactor.Redact(result)
	}
	return result
}

// formatMessage formats a log message with timestamp and caller information
func (sl *SecureLogger) formatMessage(level LogLevel, message string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	
	if sl.debug {
		// Add caller information for debug mode
		// Try different caller depths to find the actual caller
		for depth := 3; depth <= 5; depth++ {
			_, file, line, ok := runtime.Caller(depth)
			if ok && !strings.Contains(file, "logger.go") {
				// Get just the filename, not the full path
				parts := strings.Split(file, "/")
				filename := parts[len(parts)-1]
				return fmt.Sprintf("[%s] %s %s:%d %s", timestamp, level.String(), filename, line, message)
			}
		}
	}
	
	return fmt.Sprintf("[%s] %s %s", timestamp, level.String(), message)
}

// shouldLog determines if a message should be logged based on level
func (sl *SecureLogger) shouldLog(level LogLevel) bool {
	if sl.quiet && level > LogLevelError {
		return false
	}
	return level <= sl.level
}

// Error logs an error message
func (sl *SecureLogger) Error(format string, args ...interface{}) {
	if !sl.shouldLog(LogLevelError) {
		return
	}
	
	message := fmt.Sprintf(format, args...)
	message = sl.redactSensitiveData(message)
	formatted := sl.formatMessage(LogLevelError, message)
	sl.logger.Print(formatted)
}

// Warn logs a warning message
func (sl *SecureLogger) Warn(format string, args ...interface{}) {
	if !sl.shouldLog(LogLevelWarn) {
		return
	}
	
	message := fmt.Sprintf(format, args...)
	message = sl.redactSensitiveData(message)
	formatted := sl.formatMessage(LogLevelWarn, message)
	sl.logger.Print(formatted)
}

// Info logs an info message
func (sl *SecureLogger) Info(format string, args ...interface{}) {
	if !sl.shouldLog(LogLevelInfo) {
		return
	}
	
	message := fmt.Sprintf(format, args...)
	message = sl.redactSensitiveData(message)
	formatted := sl.formatMessage(LogLevelInfo, message)
	sl.logger.Print(formatted)
}

// Debug logs a debug message
func (sl *SecureLogger) Debug(format string, args ...interface{}) {
	if !sl.shouldLog(LogLevelDebug) {
		return
	}
	
	message := fmt.Sprintf(format, args...)
	message = sl.redactSensitiveData(message)
	formatted := sl.formatMessage(LogLevelDebug, message)
	sl.logger.Print(formatted)
}

// LogHTTPRequest logs an HTTP request with sensitive data redacted
func (sl *SecureLogger) LogHTTPRequest(req *http.Request) {
	if !sl.shouldLog(LogLevelDebug) {
		return
	}
	
	// Create sanitized headers
	sanitizedHeaders := make(map[string]string)
	for name, values := range req.Header {
		if sl.isSensitiveHeader(name) {
			sanitizedHeaders[name] = "[REDACTED]"
		} else {
			sanitizedHeaders[name] = strings.Join(values, ", ")
		}
	}
	
	// Redact URL if it contains sensitive parameters
	url := sl.redactSensitiveData(req.URL.String())
	
	sl.Debug("HTTP Request: %s %s Headers: %v", req.Method, url, sanitizedHeaders)
}

// LogHTTPResponse logs an HTTP response with sensitive data redacted
func (sl *SecureLogger) LogHTTPResponse(resp *http.Response) {
	if !sl.shouldLog(LogLevelDebug) {
		return
	}
	
	// Create sanitized headers
	sanitizedHeaders := make(map[string]string)
	for name, values := range resp.Header {
		if sl.isSensitiveHeader(name) {
			sanitizedHeaders[name] = "[REDACTED]"
		} else {
			sanitizedHeaders[name] = strings.Join(values, ", ")
		}
	}
	
	sl.Debug("HTTP Response: %d %s Headers: %v", resp.StatusCode, resp.Status, sanitizedHeaders)
}

// isSensitiveHeader checks if a header contains sensitive information
func (sl *SecureLogger) isSensitiveHeader(name string) bool {
	sensitiveHeaders := []string{
		"authorization",
		"cookie",
		"set-cookie",
		"x-auth-token",
		"x-api-key",
		"bearer",
		"token",
	}
	
	lowerName := strings.ToLower(name)
	for _, sensitive := range sensitiveHeaders {
		if strings.Contains(lowerName, sensitive) {
			return true
		}
	}
	return false
}

// SetLevel sets the logging level
func (sl *SecureLogger) SetLevel(level LogLevel) {
	sl.level = level
}

// SetDebug enables or disables debug mode
func (sl *SecureLogger) SetDebug(debug bool) {
	sl.debug = debug
	if debug && sl.level > LogLevelDebug {
		sl.level = LogLevelDebug
	}
}

// SetQuiet enables or disables quiet mode
func (sl *SecureLogger) SetQuiet(quiet bool) {
	sl.quiet = quiet
	if quiet {
		sl.level = LogLevelError
	}
}

// AddRedactor adds a custom redactor
func (sl *SecureLogger) AddRedactor(redactor Redactor) {
	sl.redactors = append(sl.redactors, redactor)
}