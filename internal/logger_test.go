package internal

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
)

func TestSecureLogger_RedactSensitiveData(t *testing.T) {
	logger := NewDefaultLogger(false, false)
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "redact_bduss_cookie",
			input:    "Cookie: BDUSS=abc123def456; other=value",
			expected: "Cookie: BDUSS=[REDACTED]; other=value",
		},
		{
			name:     "redact_stoken_cookie",
			input:    "Set-Cookie: STOKEN=xyz789; Path=/",
			expected: "Set-Cookie: STOKEN=[REDACTED] Path=/",
		},
		{
			name:     "redact_authorization_header",
			input:    "Authorization: Bearer token123",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "redact_url_parameters",
			input:    "https://example.com/api?access_token=secret123&other=param",
			expected: "https://example.com/api?access_token=[REDACTED]&other=param",
		},
		{
			name:     "no_sensitive_data",
			input:    "This is a normal log message",
			expected: "This is a normal log message",
		},
		{
			name:     "multiple_sensitive_items",
			input:    "Cookie: BDUSS=secret1; Authorization: Bearer secret2",
			expected: "Cookie: BDUSS=[REDACTED]; Authorization: Bearer [REDACTED]",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := logger.redactSensitiveData(tt.input)
			if result != tt.expected {
				t.Errorf("redactSensitiveData() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestSecureLogger_LogLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecureLogger(&buf, LogLevelWarn, false, false)
	
	// Test that debug and info messages are not logged when level is WARN
	logger.Debug("debug message")
	logger.Info("info message")
	
	output := buf.String()
	if strings.Contains(output, "debug message") {
		t.Error("Debug message should not be logged when level is WARN")
	}
	if strings.Contains(output, "info message") {
		t.Error("Info message should not be logged when level is WARN")
	}
	
	// Test that warn and error messages are logged
	buf.Reset()
	logger.Warn("warn message")
	logger.Error("error message")
	
	output = buf.String()
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message should be logged when level is WARN")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message should be logged when level is WARN")
	}
}

func TestSecureLogger_QuietMode(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecureLogger(&buf, LogLevelDebug, false, true) // quiet mode enabled
	
	// In quiet mode, only error messages should be logged
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	
	output := buf.String()
	if output != "" {
		t.Errorf("No messages should be logged in quiet mode except errors, got: %s", output)
	}
	
	// Error messages should still be logged
	logger.Error("error message")
	output = buf.String()
	if !strings.Contains(output, "error message") {
		t.Error("Error messages should be logged even in quiet mode")
	}
}

func TestSecureLogger_DebugMode(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecureLogger(&buf, LogLevelDebug, true, false) // debug mode enabled
	
	logger.Info("test message")
	
	output := buf.String()
	// In debug mode, messages should include file and line information
	// Check for either the test file or any file:line pattern
	hasFileInfo := strings.Contains(output, ".go:") || strings.Contains(output, "logger_test.go:")
	if !hasFileInfo {
		t.Errorf("Debug mode should include file and line information, got: %s", output)
	}
}

func TestSecureLogger_HTTPRequestLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewSecureLogger(&buf, LogLevelDebug, true, false)
	
	req, _ := http.NewRequest("GET", "https://example.com/api?token=secret123", nil)
	req.Header.Set("Authorization", "Bearer secret456")
	req.Header.Set("User-Agent", "TestAgent/1.0")
	req.Header.Set("Cookie", "BDUSS=secret789")
	
	logger.LogHTTPRequest(req)
	
	output := buf.String()
	
	// Check that sensitive data is redacted
	if strings.Contains(output, "secret123") {
		t.Error("URL token should be redacted")
	}
	if strings.Contains(output, "secret456") {
		t.Error("Authorization header should be redacted")
	}
	if strings.Contains(output, "secret789") {
		t.Error("Cookie should be redacted")
	}
	
	// Check that non-sensitive data is preserved
	if !strings.Contains(output, "TestAgent/1.0") {
		t.Error("User-Agent should be preserved")
	}
	if !strings.Contains(output, "[REDACTED]") {
		t.Error("Redacted placeholder should be present")
	}
}

func TestSecureLogger_IsSensitiveHeader(t *testing.T) {
	logger := NewDefaultLogger(false, false)
	
	tests := []struct {
		header    string
		sensitive bool
	}{
		{"Authorization", true},
		{"Cookie", true},
		{"Set-Cookie", true},
		{"X-Auth-Token", true},
		{"X-API-Key", true},
		{"User-Agent", false},
		{"Content-Type", false},
		{"Accept", false},
		{"bearer", true}, // case insensitive
		{"COOKIE", true}, // case insensitive
	}
	
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			result := logger.isSensitiveHeader(tt.header)
			if result != tt.sensitive {
				t.Errorf("isSensitiveHeader(%q) = %v, want %v", tt.header, result, tt.sensitive)
			}
		})
	}
}

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{LogLevelError, "ERROR"},
		{LogLevelWarn, "WARN"},
		{LogLevelInfo, "INFO"},
		{LogLevelDebug, "DEBUG"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.level.String()
			if result != tt.expected {
				t.Errorf("LogLevel.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestCookieRedactor_Redact(t *testing.T) {
	redactor := &CookieRedactor{}
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bduss_cookie",
			input:    "BDUSS=1234567890abcdef",
			expected: "BDUSS=[REDACTED]",
		},
		{
			name:     "stoken_cookie",
			input:    "STOKEN=abcdef1234567890",
			expected: "STOKEN=[REDACTED]",
		},
		{
			name:     "cookie_with_semicolon",
			input:    "Cookie: BDUSS=secret123; Path=/",
			expected: "Cookie: BDUSS=[REDACTED]; Path=/",
		},
		{
			name:     "authorization_bearer",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "Authorization: Bearer [REDACTED]",
		},
		{
			name:     "no_sensitive_data",
			input:    "User-Agent: Mozilla/5.0",
			expected: "User-Agent: Mozilla/5.0",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("CookieRedactor.Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestURLRedactor_Redact(t *testing.T) {
	redactor := &URLRedactor{}
	
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "access_token_parameter",
			input:    "https://api.example.com/data?access_token=secret123&other=value",
			expected: "https://api.example.com/data?access_token=[REDACTED]&other=value",
		},
		{
			name:     "token_parameter",
			input:    "https://api.example.com/data?token=abc123",
			expected: "https://api.example.com/data?token=[REDACTED]",
		},
		{
			name:     "key_parameter",
			input:    "https://api.example.com/data?key=mykey123&format=json",
			expected: "https://api.example.com/data?key=[REDACTED]&format=json",
		},
		{
			name:     "no_sensitive_parameters",
			input:    "https://api.example.com/data?format=json&limit=10",
			expected: "https://api.example.com/data?format=json&limit=10",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if result != tt.expected {
				t.Errorf("URLRedactor.Redact() = %q, want %q", result, tt.expected)
			}
		})
	}
}