package internal

import (
	"strings"
	"testing"
)

func TestTeraboxError_Error(t *testing.T) {
	err := NewTeraboxError(404, "File not found", ErrFileNotFound)
	
	result := err.Error()
	
	if !strings.Contains(result, "terabox error") {
		t.Error("Error message should contain 'terabox error'")
	}
	if !strings.Contains(result, "404") {
		t.Error("Error message should contain error code")
	}
	if !strings.Contains(result, "FileNotFound") {
		t.Error("Error message should contain error type")
	}
	if !strings.Contains(result, "File not found") {
		t.Error("Error message should contain the message")
	}
}

func TestTeraboxError_DetailedError(t *testing.T) {
	err := NewTeraboxError(429, "Rate limit exceeded", ErrRateLimit).
		WithURL("https://terabox.com/api/download").
		WithRetryAfter(60).
		WithContext("attempts", 3)
	
	result := err.DetailedError()
	
	// Check that all components are present
	if !strings.Contains(result, "WARNING") {
		t.Error("Detailed error should contain severity")
	}
	if !strings.Contains(result, "RateLimit Error") {
		t.Error("Detailed error should contain error type")
	}
	if !strings.Contains(result, "Code: 429") {
		t.Error("Detailed error should contain error code")
	}
	if !strings.Contains(result, "Rate limit exceeded") {
		t.Error("Detailed error should contain message")
	}
	if !strings.Contains(result, "Retry after: 60 seconds") {
		t.Error("Detailed error should contain retry information")
	}
	if !strings.Contains(result, "attempts=3") {
		t.Error("Detailed error should contain context")
	}
	if !strings.Contains(result, "Suggestion:") {
		t.Error("Detailed error should contain suggestion")
	}
	
	// Check that URL is present but potentially redacted
	if !strings.Contains(result, "terabox.com/api/download") {
		t.Error("URL should be present in detailed error")
	}
}

func TestTeraboxError_IsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		code      int
		retryable bool
	}{
		{"network_timeout", ErrNetworkTimeout, 408, true},
		{"rate_limit", ErrRateLimit, 429, true},
		{"invalid_url", ErrInvalidURL, 400, false},
		{"auth_required", ErrAuthRequired, 401, false},
		{"file_not_found", ErrFileNotFound, 404, false},
		{"server_error", ErrInvalidResponse, 500, true},
		{"client_error", ErrInvalidResponse, 400, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewTeraboxError(tt.code, "test message", tt.errorType)
			result := err.IsRetryable()
			if result != tt.retryable {
				t.Errorf("IsRetryable() = %v, want %v for error type %v", result, tt.retryable, tt.errorType)
			}
		})
	}
}

func TestTeraboxError_IsCritical(t *testing.T) {
	// Test critical error
	criticalErr := NewTeraboxError(403, "Permission denied", ErrPermissionDenied)
	if !criticalErr.IsCritical() {
		t.Error("Permission denied error should be critical")
	}
	
	// Test non-critical error
	nonCriticalErr := NewTeraboxError(408, "Timeout", ErrNetworkTimeout)
	if nonCriticalErr.IsCritical() {
		t.Error("Network timeout error should not be critical")
	}
}

func TestErrorType_String(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		expected  string
	}{
		{ErrInvalidURL, "InvalidURL"},
		{ErrAuthRequired, "AuthRequired"},
		{ErrRateLimit, "RateLimit"},
		{ErrNetworkTimeout, "NetworkTimeout"},
		{ErrFileNotFound, "FileNotFound"},
		{ErrQuotaExceeded, "QuotaExceeded"},
		{ErrInvalidResponse, "InvalidResponse"},
		{ErrDownloadFailed, "DownloadFailed"},
		{ErrPermissionDenied, "PermissionDenied"},
		{ErrDiskSpace, "DiskSpace"},
		{ErrCorruptedFile, "CorruptedFile"},
		{ErrUnsupportedFormat, "UnsupportedFormat"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.errorType.String()
			if result != tt.expected {
				t.Errorf("ErrorType.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestErrorSeverity_String(t *testing.T) {
	tests := []struct {
		severity ErrorSeverity
		expected string
	}{
		{SeverityInfo, "INFO"},
		{SeverityWarning, "WARNING"},
		{SeverityError, "ERROR"},
		{SeverityCritical, "CRITICAL"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.severity.String()
			if result != tt.expected {
				t.Errorf("ErrorSeverity.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	err := NewValidationError("url", "invalid format").
		WithSuggestion("Use https://terabox.com/s/... format")
	
	result := err.Error()
	
	if !strings.Contains(result, "validation error for url") {
		t.Error("Error should contain field name")
	}
	if !strings.Contains(result, "invalid format") {
		t.Error("Error should contain message")
	}
	if !strings.Contains(result, "Suggestion:") {
		t.Error("Error should contain suggestion")
	}
}

func TestValidationError_DetailedError(t *testing.T) {
	err := NewValidationErrorWithValue("threads", "must be between 1 and 32", 50).
		WithSuggestion("Use a value between 1 and 32").
		WithContext("max_allowed", 32).
		WithContext("min_allowed", 1)
	
	result := err.DetailedError()
	
	if !strings.Contains(result, "Validation Error for field 'threads'") {
		t.Error("Detailed error should contain field name")
	}
	if !strings.Contains(result, "Provided value: 50") {
		t.Error("Detailed error should contain provided value")
	}
	if !strings.Contains(result, "max_allowed=32") {
		t.Error("Detailed error should contain context")
	}
	if !strings.Contains(result, "Suggestion:") {
		t.Error("Detailed error should contain suggestion")
	}
}

func TestCommonErrorConstructors(t *testing.T) {
	t.Run("NewInvalidURLError", func(t *testing.T) {
		err := NewInvalidURLError("https://invalid.com", "unsupported domain")
		
		if err.Type != ErrInvalidURL {
			t.Error("Should create InvalidURL error type")
		}
		if err.Code != 400 {
			t.Error("Should set appropriate error code")
		}
		if !strings.Contains(err.Suggestion, "valid Terabox share URL") {
			t.Error("Should provide helpful suggestion")
		}
	})
	
	t.Run("NewAuthRequiredError", func(t *testing.T) {
		err := NewAuthRequiredError("Authentication required for private files")
		
		if err.Type != ErrAuthRequired {
			t.Error("Should create AuthRequired error type")
		}
		if err.Code != 401 {
			t.Error("Should set appropriate error code")
		}
		if !strings.Contains(err.Suggestion, "cookies") {
			t.Error("Should suggest using cookies")
		}
	})
	
	t.Run("NewRateLimitError", func(t *testing.T) {
		err := NewRateLimitError(120)
		
		if err.Type != ErrRateLimit {
			t.Error("Should create RateLimit error type")
		}
		if err.Code != 429 {
			t.Error("Should set appropriate error code")
		}
		if err.RetryAfter != 120 {
			t.Error("Should set retry after value")
		}
		if !strings.Contains(err.Suggestion, "120 seconds") {
			t.Error("Should include retry time in suggestion")
		}
	})
	
	t.Run("NewNetworkTimeoutError", func(t *testing.T) {
		err := NewNetworkTimeoutError("file download")
		
		if err.Type != ErrNetworkTimeout {
			t.Error("Should create NetworkTimeout error type")
		}
		if err.Code != 408 {
			t.Error("Should set appropriate error code")
		}
		if !strings.Contains(err.Message, "file download") {
			t.Error("Should include operation in message")
		}
	})
	
	t.Run("NewFileNotFoundError", func(t *testing.T) {
		err := NewFileNotFoundError("https://terabox.com/s/test123")
		
		if err.Type != ErrFileNotFound {
			t.Error("Should create FileNotFound error type")
		}
		if err.Code != 404 {
			t.Error("Should set appropriate error code")
		}
		if err.URL == "" {
			t.Error("Should set URL context")
		}
	})
}

func TestGetDefaultSuggestion(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		code      int
		contains  string
	}{
		{ErrInvalidURL, 400, "valid Terabox"},
		{ErrAuthRequired, 401, "cookies"},
		{ErrRateLimit, 429, "wait before retrying"},
		{ErrNetworkTimeout, 408, "internet connection"},
		{ErrFileNotFound, 404, "share link is still valid"},
		{ErrQuotaExceeded, 403, "quota has been exceeded"},
		{ErrInvalidResponse, 500, "Server error"},
		{ErrInvalidResponse, 400, "API might have changed"},
		{ErrDownloadFailed, 0, "disk space"},
		{ErrPermissionDenied, 0, "Permission denied"},
		{ErrDiskSpace, 0, "disk space"},
		{ErrCorruptedFile, 0, "corrupted"},
		{ErrUnsupportedFormat, 0, "not supported"},
	}
	
	for _, tt := range tests {
		t.Run(tt.errorType.String(), func(t *testing.T) {
			suggestion := getDefaultSuggestion(tt.errorType, tt.code)
			if !strings.Contains(strings.ToLower(suggestion), strings.ToLower(tt.contains)) {
				t.Errorf("Suggestion %q should contain %q", suggestion, tt.contains)
			}
		})
	}
}

func TestGetDefaultSeverity(t *testing.T) {
	tests := []struct {
		errorType ErrorType
		severity  ErrorSeverity
	}{
		{ErrRateLimit, SeverityWarning},
		{ErrNetworkTimeout, SeverityWarning},
		{ErrInvalidURL, SeverityError},
		{ErrAuthRequired, SeverityError},
		{ErrFileNotFound, SeverityError},
		{ErrQuotaExceeded, SeverityCritical},
		{ErrPermissionDenied, SeverityCritical},
		{ErrDiskSpace, SeverityCritical},
	}
	
	for _, tt := range tests {
		t.Run(tt.errorType.String(), func(t *testing.T) {
			severity := getDefaultSeverity(tt.errorType)
			if severity != tt.severity {
				t.Errorf("getDefaultSeverity(%v) = %v, want %v", tt.errorType, severity, tt.severity)
			}
		})
	}
}

func TestRedactSensitiveURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "url_with_query_params",
			input:    "https://api.terabox.com/download?token=secret123&file=test.zip",
			expected: "https://api.terabox.com/download?[REDACTED]",
		},
		{
			name:     "url_without_query_params",
			input:    "https://api.terabox.com/download",
			expected: "https://api.terabox.com/download",
		},
		{
			name:     "empty_url",
			input:    "",
			expected: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactSensitiveURL(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveURL() = %q, want %q", result, tt.expected)
			}
		})
	}
}