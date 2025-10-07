package internal

import (
	"fmt"
	"strings"
)

// ErrorType represents different types of errors
type ErrorType int

const (
	ErrInvalidURL ErrorType = iota
	ErrAuthRequired
	ErrRateLimit
	ErrNetworkTimeout
	ErrFileNotFound
	ErrQuotaExceeded
	ErrInvalidResponse
	ErrDownloadFailed
	ErrPermissionDenied
	ErrDiskSpace
	ErrCorruptedFile
	ErrUnsupportedFormat
	ErrResumeDataCorrupted
	ErrResumeIncompatible
	ErrPartialFileInvalid
)

// ErrorSeverity represents the severity of an error
type ErrorSeverity int

const (
	SeverityInfo ErrorSeverity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

// TeraboxError represents a Terabox-specific error with detailed information
type TeraboxError struct {
	Code        int           `json:"errno"`
	Message     string        `json:"errmsg"`
	Type        ErrorType     `json:"type"`
	Severity    ErrorSeverity `json:"severity"`
	URL         string        `json:"url,omitempty"`
	Suggestion  string        `json:"suggestion,omitempty"`
	RetryAfter  int           `json:"retry_after,omitempty"` // seconds
	Context     map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *TeraboxError) Error() string {
	var parts []string
	
	// Add basic error information
	parts = append(parts, fmt.Sprintf("terabox error (code: %d, type: %s)", e.Code, e.Type.String()))
	
	// Add message
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	
	// Add suggestion if available
	if e.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("Suggestion: %s", e.Suggestion))
	}
	
	return strings.Join(parts, " - ")
}

// DetailedError returns a detailed error message with all available information
func (e *TeraboxError) DetailedError() string {
	var parts []string
	
	// Add severity and type
	parts = append(parts, fmt.Sprintf("[%s] %s Error", e.Severity.String(), e.Type.String()))
	
	// Add code and message
	if e.Code != 0 {
		parts = append(parts, fmt.Sprintf("Code: %d", e.Code))
	}
	if e.Message != "" {
		parts = append(parts, fmt.Sprintf("Message: %s", e.Message))
	}
	
	// Add URL if available (redacted)
	if e.URL != "" {
		redactedURL := redactSensitiveURL(e.URL)
		parts = append(parts, fmt.Sprintf("URL: %s", redactedURL))
	}
	
	// Add context information
	if len(e.Context) > 0 {
		contextParts := make([]string, 0, len(e.Context))
		for k, v := range e.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("Context: %s", strings.Join(contextParts, ", ")))
	}
	
	// Add suggestion
	if e.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("\nSuggestion: %s", e.Suggestion))
	}
	
	// Add retry information
	if e.RetryAfter > 0 {
		parts = append(parts, fmt.Sprintf("Retry after: %d seconds", e.RetryAfter))
	}
	
	return strings.Join(parts, "\n")
}

// String returns the string representation of ErrorType
func (et ErrorType) String() string {
	switch et {
	case ErrInvalidURL:
		return "InvalidURL"
	case ErrAuthRequired:
		return "AuthRequired"
	case ErrRateLimit:
		return "RateLimit"
	case ErrNetworkTimeout:
		return "NetworkTimeout"
	case ErrFileNotFound:
		return "FileNotFound"
	case ErrQuotaExceeded:
		return "QuotaExceeded"
	case ErrInvalidResponse:
		return "InvalidResponse"
	case ErrDownloadFailed:
		return "DownloadFailed"
	case ErrPermissionDenied:
		return "PermissionDenied"
	case ErrDiskSpace:
		return "DiskSpace"
	case ErrCorruptedFile:
		return "CorruptedFile"
	case ErrUnsupportedFormat:
		return "UnsupportedFormat"
	case ErrResumeDataCorrupted:
		return "ResumeDataCorrupted"
	case ErrResumeIncompatible:
		return "ResumeIncompatible"
	case ErrPartialFileInvalid:
		return "PartialFileInvalid"
	default:
		return "Unknown"
	}
}

// String returns the string representation of ErrorSeverity
func (es ErrorSeverity) String() string {
	switch es {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// NewTeraboxError creates a new TeraboxError with detailed information
func NewTeraboxError(code int, message string, errorType ErrorType) *TeraboxError {
	err := &TeraboxError{
		Code:     code,
		Message:  message,
		Type:     errorType,
		Severity: SeverityError,
		Context:  make(map[string]interface{}),
	}
	
	// Set default suggestions based on error type
	err.Suggestion = getDefaultSuggestion(errorType, code)
	err.Severity = getDefaultSeverity(errorType)
	
	return err
}

// NewTeraboxErrorWithContext creates a TeraboxError with additional context
func NewTeraboxErrorWithContext(code int, message string, errorType ErrorType, context map[string]interface{}) *TeraboxError {
	err := NewTeraboxError(code, message, errorType)
	if context != nil {
		for k, v := range context {
			err.Context[k] = v
		}
	}
	return err
}

// WithSuggestion adds a custom suggestion to the error
func (e *TeraboxError) WithSuggestion(suggestion string) *TeraboxError {
	e.Suggestion = suggestion
	return e
}

// WithURL adds URL context to the error (will be redacted in logs)
func (e *TeraboxError) WithURL(url string) *TeraboxError {
	e.URL = url
	return e
}

// WithRetryAfter sets the retry delay for rate limit errors
func (e *TeraboxError) WithRetryAfter(seconds int) *TeraboxError {
	e.RetryAfter = seconds
	return e
}

// WithContext adds context information to the error
func (e *TeraboxError) WithContext(key string, value interface{}) *TeraboxError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// IsRetryable returns true if the error is retryable
func (e *TeraboxError) IsRetryable() bool {
	switch e.Type {
	case ErrNetworkTimeout, ErrRateLimit:
		return true
	case ErrInvalidResponse:
		// Some invalid responses might be temporary
		return e.Code >= 500
	default:
		return false
	}
}

// IsCritical returns true if the error is critical and should stop execution
func (e *TeraboxError) IsCritical() bool {
	return e.Severity == SeverityCritical
}

// ValidationError represents input validation errors
type ValidationError struct {
	Field      string                 `json:"field"`
	Message    string                 `json:"message"`
	Value      interface{}            `json:"value,omitempty"`
	Suggestion string                 `json:"suggestion,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	parts := []string{fmt.Sprintf("validation error for %s: %s", e.Field, e.Message)}
	
	if e.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("Suggestion: %s", e.Suggestion))
	}
	
	return strings.Join(parts, " - ")
}

// DetailedError returns a detailed validation error message
func (e *ValidationError) DetailedError() string {
	var parts []string
	
	parts = append(parts, fmt.Sprintf("Validation Error for field '%s'", e.Field))
	parts = append(parts, fmt.Sprintf("Message: %s", e.Message))
	
	if e.Value != nil {
		parts = append(parts, fmt.Sprintf("Provided value: %v", e.Value))
	}
	
	if len(e.Context) > 0 {
		contextParts := make([]string, 0, len(e.Context))
		for k, v := range e.Context {
			contextParts = append(contextParts, fmt.Sprintf("%s=%v", k, v))
		}
		parts = append(parts, fmt.Sprintf("Context: %s", strings.Join(contextParts, ", ")))
	}
	
	if e.Suggestion != "" {
		parts = append(parts, fmt.Sprintf("\nSuggestion: %s", e.Suggestion))
	}
	
	return strings.Join(parts, "\n")
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Context: make(map[string]interface{}),
	}
}

// NewValidationErrorWithValue creates a ValidationError with the invalid value
func NewValidationErrorWithValue(field, message string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
		Context: make(map[string]interface{}),
	}
}

// WithSuggestion adds a suggestion to the validation error
func (e *ValidationError) WithSuggestion(suggestion string) *ValidationError {
	e.Suggestion = suggestion
	return e
}

// WithContext adds context to the validation error
func (e *ValidationError) WithContext(key string, value interface{}) *ValidationError {
	if e.Context == nil {
		e.Context = make(map[string]interface{})
	}
	e.Context[key] = value
	return e
}

// getDefaultSuggestion returns a default suggestion based on error type and code
func getDefaultSuggestion(errorType ErrorType, code int) string {
	switch errorType {
	case ErrInvalidURL:
		return "Please ensure the URL is a valid Terabox or Baidu Pan share link (e.g., https://terabox.com/s/...)"
	case ErrAuthRequired:
		return "Please provide valid cookies using --cookies flag or TERABOX_COOKIES environment variable"
	case ErrRateLimit:
		return "Please wait before retrying. Consider using --limit-rate to reduce bandwidth usage"
	case ErrNetworkTimeout:
		return "Check your internet connection and try again. Consider using a proxy if needed"
	case ErrFileNotFound:
		return "Verify the share link is still valid and the file hasn't been removed"
	case ErrQuotaExceeded:
		return "Your download quota has been exceeded. Try again later or use a different account"
	case ErrInvalidResponse:
		if code >= 500 {
			return "Server error occurred. Please try again later"
		}
		return "Invalid response from server. The API might have changed or the link is invalid"
	case ErrDownloadFailed:
		return "Download failed. Check available disk space and network connection"
	case ErrPermissionDenied:
		return "Permission denied. Check file/directory permissions or try running with appropriate privileges"
	case ErrDiskSpace:
		return "Insufficient disk space. Free up space or choose a different output directory"
	case ErrCorruptedFile:
		return "File appears to be corrupted. Try downloading again or check the source"
	case ErrUnsupportedFormat:
		return "File format is not supported or recognized"
	case ErrResumeDataCorrupted:
		return "Resume metadata is corrupted. Delete the .terafetch.json file and restart the download"
	case ErrResumeIncompatible:
		return "Resume data is incompatible with current download. Delete resume files and restart"
	case ErrPartialFileInvalid:
		return "Partial download file is invalid. Delete the .part file and restart the download"
	default:
		return "Please check the error details and try again"
	}
}

// getDefaultSeverity returns the default severity for an error type
func getDefaultSeverity(errorType ErrorType) ErrorSeverity {
	switch errorType {
	case ErrRateLimit, ErrNetworkTimeout:
		return SeverityWarning
	case ErrInvalidURL, ErrAuthRequired, ErrFileNotFound:
		return SeverityError
	case ErrQuotaExceeded, ErrPermissionDenied, ErrDiskSpace:
		return SeverityCritical
	default:
		return SeverityError
	}
}

// redactSensitiveURL redacts sensitive information from URLs
func redactSensitiveURL(url string) string {
	// Simple redaction - replace query parameters that might contain sensitive data
	if strings.Contains(url, "?") {
		parts := strings.Split(url, "?")
		return parts[0] + "?[REDACTED]"
	}
	return url
}

// Common error constructors for frequently used errors

// NewInvalidURLError creates an error for invalid URLs
func NewInvalidURLError(url string, reason string) *TeraboxError {
	return NewTeraboxError(400, fmt.Sprintf("Invalid URL: %s", reason), ErrInvalidURL).
		WithURL(url).
		WithSuggestion("Please provide a valid Terabox share URL (https://terabox.com/s/...)")
}

// NewAuthRequiredError creates an error for authentication requirements
func NewAuthRequiredError(message string) *TeraboxError {
	return NewTeraboxError(401, message, ErrAuthRequired).
		WithSuggestion("Please provide valid cookies using --cookies flag or set TERABOX_COOKIES environment variable")
}

// NewRateLimitError creates an error for rate limiting
func NewRateLimitError(retryAfter int) *TeraboxError {
	return NewTeraboxError(429, "Rate limit exceeded", ErrRateLimit).
		WithRetryAfter(retryAfter).
		WithSuggestion(fmt.Sprintf("Please wait %d seconds before retrying", retryAfter))
}

// NewNetworkTimeoutError creates an error for network timeouts
func NewNetworkTimeoutError(operation string) *TeraboxError {
	return NewTeraboxError(408, fmt.Sprintf("Network timeout during %s", operation), ErrNetworkTimeout).
		WithSuggestion("Check your internet connection and try again. Consider using a proxy if needed")
}

// NewFileNotFoundError creates an error for missing files
func NewFileNotFoundError(url string) *TeraboxError {
	return NewTeraboxError(404, "File not found or share link is invalid", ErrFileNotFound).
		WithURL(url).
		WithSuggestion("Verify the share link is still valid and the file hasn't been removed")
}

// NewResumeDataCorruptedError creates an error for corrupted resume metadata
func NewResumeDataCorruptedError(path string, reason string) *TeraboxError {
	return NewTeraboxError(500, fmt.Sprintf("Resume metadata corrupted: %s", reason), ErrResumeDataCorrupted).
		WithContext("metadata_path", path).
		WithSuggestion("Delete the .terafetch.json file and restart the download")
}

// NewResumeIncompatibleError creates an error for incompatible resume data
func NewResumeIncompatibleError(reason string) *TeraboxError {
	return NewTeraboxError(409, fmt.Sprintf("Resume data incompatible: %s", reason), ErrResumeIncompatible).
		WithSuggestion("Delete resume files (.part and .terafetch.json) and restart the download")
}

// NewPartialFileInvalidError creates an error for invalid partial files
func NewPartialFileInvalidError(path string, reason string) *TeraboxError {
	return NewTeraboxError(422, fmt.Sprintf("Partial file invalid: %s", reason), ErrPartialFileInvalid).
		WithContext("partial_file", path).
		WithSuggestion("Delete the .part file and restart the download")
}