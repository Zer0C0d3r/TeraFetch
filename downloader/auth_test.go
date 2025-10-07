package downloader

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"terafetch/internal"
)

func TestCookieAuthManager_LoadCookies(t *testing.T) {
	// Create a temporary cookie file
	tmpDir := t.TempDir()
	cookieFile := filepath.Join(tmpDir, "cookies.txt")

	// Sample Netscape cookie file content
	cookieContent := `# Netscape HTTP Cookie File
# This is a generated file!  Do not edit.

.terabox.com	TRUE	/	FALSE	1735689600	BDUSS	abcdef1234567890abcdef1234567890abcdef12
.terabox.com	TRUE	/	FALSE	1735689600	STOKEN	xyz789xyz789xyz789xyz789xyz789xyz789
.terabox.com	TRUE	/	TRUE	1735689600	session_id	sess_123456789
`

	err := os.WriteFile(cookieFile, []byte(cookieContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test cookie file: %v", err)
	}

	authManager := NewCookieAuthManager()
	authContext, err := authManager.LoadCookies(cookieFile)

	if err != nil {
		t.Fatalf("LoadCookies failed: %v", err)
	}

	if authContext == nil {
		t.Fatal("AuthContext is nil")
	}

	// Verify BDUSS was extracted
	expectedBDUSS := "abcdef1234567890abcdef1234567890abcdef12"
	if authContext.BDUSS != expectedBDUSS {
		t.Errorf("Expected BDUSS %s, got %s", expectedBDUSS, authContext.BDUSS)
	}

	// Verify STOKEN was extracted
	expectedSTOKEN := "xyz789xyz789xyz789xyz789xyz789xyz789"
	if authContext.STOKEN != expectedSTOKEN {
		t.Errorf("Expected STOKEN %s, got %s", expectedSTOKEN, authContext.STOKEN)
	}

	// Verify cookies were loaded
	if len(authContext.Cookies) != 3 {
		t.Errorf("Expected 3 cookies, got %d", len(authContext.Cookies))
	}

	// Verify specific cookie properties
	if bdussCookie, exists := authContext.Cookies["BDUSS"]; exists {
		if bdussCookie.Domain != ".terabox.com" {
			t.Errorf("Expected domain .terabox.com, got %s", bdussCookie.Domain)
		}
		if bdussCookie.Path != "/" {
			t.Errorf("Expected path /, got %s", bdussCookie.Path)
		}
		if bdussCookie.Secure {
			t.Error("Expected BDUSS cookie to not be secure based on test data")
		}
	} else {
		t.Error("BDUSS cookie not found in loaded cookies")
	}
}

func TestCookieAuthManager_ValidateSession(t *testing.T) {
	authManager := NewCookieAuthManager()

	tests := []struct {
		name        string
		authContext *internal.AuthContext
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil auth context",
			authContext: nil,
			expectError: true,
			errorMsg:    "auth context is nil",
		},
		{
			name: "missing BDUSS",
			authContext: &internal.AuthContext{
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: true,
			errorMsg:    "BDUSS cookie is required",
		},
		{
			name: "invalid BDUSS too short",
			authContext: &internal.AuthContext{
				BDUSS:     "short",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: true,
			errorMsg:    "BDUSS cookie appears to be invalid",
		},
		{
			name: "missing STOKEN",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: true,
			errorMsg:    "STOKEN cookie is required",
		},
		{
			name: "expired session",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(-time.Hour),
			},
			expectError: true,
			errorMsg:    "session has expired",
		},
		{
			name: "valid session",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authManager.ValidateSession(tt.authContext)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

func TestCookieAuthManager_RefreshSession(t *testing.T) {
	authManager := NewCookieAuthManager()

	tests := []struct {
		name        string
		authContext *internal.AuthContext
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil auth context",
			authContext: nil,
			expectError: true,
			errorMsg:    "auth context is nil",
		},
		{
			name: "missing STOKEN",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: true,
			errorMsg:    "STOKEN is required",
		},
		{
			name: "session can be extended",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(2 * time.Hour),
			},
			expectError: false,
		},
		{
			name: "session too close to expiry",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(30 * time.Minute),
			},
			expectError: true,
			errorMsg:    "session cannot be refreshed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalExpiry := time.Time{}
			if tt.authContext != nil {
				originalExpiry = tt.authContext.ExpiresAt
			}

			err := authManager.RefreshSession(tt.authContext)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				// Verify that expiry was extended
				if tt.authContext != nil && !tt.authContext.ExpiresAt.After(originalExpiry) {
					t.Error("Expected session expiry to be extended")
				}
			}
		})
	}
}

func TestParseNetscapeCookieLine(t *testing.T) {
	authManager := NewCookieAuthManager()

	tests := []struct {
		name        string
		line        string
		expectError bool
		expected    *http.Cookie
	}{
		{
			name:        "invalid format - too few fields",
			line:        ".terabox.com\tTRUE\t/",
			expectError: true,
		},
		{
			name: "valid cookie line",
			line: ".terabox.com\tTRUE\t/\tFALSE\t1735689600\tBDUSS\tabcdef123456",
			expected: &http.Cookie{
				Name:     "BDUSS",
				Value:    "abcdef123456",
				Domain:   ".terabox.com",
				Path:     "/",
				Expires:  time.Unix(1735689600, 0),
				Secure:   false,
				HttpOnly: true,
			},
		},
		{
			name: "secure cookie",
			line: ".terabox.com\tTRUE\t/\tTRUE\t0\tsession\tsecure_value",
			expected: &http.Cookie{
				Name:     "session",
				Value:    "secure_value",
				Domain:   ".terabox.com",
				Path:     "/",
				Expires:  time.Time{},
				Secure:   true,
				HttpOnly: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cookie, err := authManager.parseNetscapeCookieLine(tt.line)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if cookie == nil {
					t.Fatal("Expected cookie, got nil")
				}

				if cookie.Name != tt.expected.Name {
					t.Errorf("Expected name %s, got %s", tt.expected.Name, cookie.Name)
				}
				if cookie.Value != tt.expected.Value {
					t.Errorf("Expected value %s, got %s", tt.expected.Value, cookie.Value)
				}
				if cookie.Domain != tt.expected.Domain {
					t.Errorf("Expected domain %s, got %s", tt.expected.Domain, cookie.Domain)
				}
				if cookie.Secure != tt.expected.Secure {
					t.Errorf("Expected secure %v, got %v", tt.expected.Secure, cookie.Secure)
				}
			}
		})
	}
}

func TestIsValidBDUSS(t *testing.T) {
	tests := []struct {
		name     string
		bduss    string
		expected bool
	}{
		{
			name:     "too short",
			bduss:    "short",
			expected: false,
		},
		{
			name:     "valid alphanumeric",
			bduss:    "abcdef1234567890ABCDEF1234567890abcdef12",
			expected: true,
		},
		{
			name:     "valid with allowed special chars",
			bduss:    "abc-def_123~456789012345678901234567890",
			expected: true,
		},
		{
			name:     "invalid special chars",
			bduss:    "abcdef123456789012345678901234567890@#$%",
			expected: false,
		},
		{
			name:     "empty string",
			bduss:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidBDUSS(tt.bduss)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for BDUSS: %s", tt.expected, result, tt.bduss)
			}
		})
	}
}

func TestCookieAuthManager_Cleanup(t *testing.T) {
	authManager := NewCookieAuthManager()

	// Add some test cookies
	authManager.cookieStore["test1"] = &http.Cookie{Name: "test1", Value: "value1"}
	authManager.cookieStore["test2"] = &http.Cookie{Name: "test2", Value: "value2"}

	if len(authManager.cookieStore) != 2 {
		t.Errorf("Expected 2 cookies before cleanup, got %d", len(authManager.cookieStore))
	}

	authManager.Cleanup()

	if len(authManager.cookieStore) != 0 {
		t.Errorf("Expected 0 cookies after cleanup, got %d", len(authManager.cookieStore))
	}
}

// TestCookieAuthManager_LoadCookiesEdgeCases tests edge cases for cookie loading
func TestCookieAuthManager_LoadCookiesEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		cookieContent string
		expectError  bool
		errorMsg     string
		expectedBDUSS string
		expectedSTOKEN string
	}{
		{
			name: "empty file",
			cookieContent: "",
			expectError: false,
			expectedBDUSS: "",
			expectedSTOKEN: "",
		},
		{
			name: "only comments",
			cookieContent: `# Netscape HTTP Cookie File
# This is a generated file!  Do not edit.
`,
			expectError: false,
			expectedBDUSS: "",
			expectedSTOKEN: "",
		},
		{
			name: "malformed cookie line - insufficient fields",
			cookieContent: `.terabox.com	TRUE	/	FALSE`,
			expectError: true,
			errorMsg: "invalid cookie format",
		},
		{
			name: "malformed cookie line - too many fields",
			cookieContent: `.terabox.com	TRUE	/	FALSE	1735689600	BDUSS	value	extra	field`,
			expectError: true,
			errorMsg: "invalid cookie format",
		},
		{
			name: "invalid expiration timestamp",
			cookieContent: `.terabox.com	TRUE	/	FALSE	invalid_timestamp	BDUSS	value123456789012345678901234567890`,
			expectError: true,
			errorMsg: "invalid expiration timestamp",
		},
		{
			name: "mixed valid and invalid lines",
			cookieContent: `# Valid comment
.terabox.com	TRUE	/	FALSE	1735689600	BDUSS	valid_bduss_1234567890123456789012345678901234567890
invalid_line_with_insufficient_fields
.terabox.com	TRUE	/	FALSE	1735689600	STOKEN	valid_stoken_123456789012345678901234567890`,
			expectError: true,
			errorMsg: "invalid cookie format",
		},
		{
			name: "cookies with zero expiration",
			cookieContent: `.terabox.com	TRUE	/	FALSE	0	BDUSS	session_bduss_1234567890123456789012345678901234567890
.terabox.com	TRUE	/	FALSE	0	STOKEN	session_stoken_123456789012345678901234567890`,
			expectError: false,
			expectedBDUSS: "session_bduss_1234567890123456789012345678901234567890",
			expectedSTOKEN: "session_stoken_123456789012345678901234567890",
		},
		{
			name: "cookies with different domains",
			cookieContent: `.terabox.com	TRUE	/	FALSE	1735689600	BDUSS	terabox_bduss_1234567890123456789012345678901234567890
.pan.baidu.com	TRUE	/	FALSE	1735689600	STOKEN	baidu_stoken_123456789012345678901234567890
.example.com	TRUE	/	FALSE	1735689600	OTHER	other_value`,
			expectError: false,
			expectedBDUSS: "terabox_bduss_1234567890123456789012345678901234567890",
			expectedSTOKEN: "baidu_stoken_123456789012345678901234567890",
		},
		{
			name: "cookies with special characters in values",
			cookieContent: `.terabox.com	TRUE	/	FALSE	1735689600	BDUSS	bduss_with-special_chars~1234567890123456789012345678901234567890
.terabox.com	TRUE	/	FALSE	1735689600	STOKEN	stoken_with-special_chars~123456789012345678901234567890`,
			expectError: false,
			expectedBDUSS: "bduss_with-special_chars~1234567890123456789012345678901234567890",
			expectedSTOKEN: "stoken_with-special_chars~123456789012345678901234567890",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cookieFile := filepath.Join(tmpDir, tt.name+"_cookies.txt")
			err := os.WriteFile(cookieFile, []byte(tt.cookieContent), 0644)
			if err != nil {
				t.Fatalf("Failed to create test cookie file: %v", err)
			}

			authManager := NewCookieAuthManager()
			authContext, err := authManager.LoadCookies(cookieFile)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
					return
				}

				if authContext.BDUSS != tt.expectedBDUSS {
					t.Errorf("Expected BDUSS '%s', got '%s'", tt.expectedBDUSS, authContext.BDUSS)
				}

				if authContext.STOKEN != tt.expectedSTOKEN {
					t.Errorf("Expected STOKEN '%s', got '%s'", tt.expectedSTOKEN, authContext.STOKEN)
				}
			}
		})
	}
}

// TestCookieAuthManager_LoadCookiesFileErrors tests file-related error conditions
func TestCookieAuthManager_LoadCookiesFileErrors(t *testing.T) {
	authManager := NewCookieAuthManager()

	// Test non-existent file
	t.Run("non_existent_file", func(t *testing.T) {
		_, err := authManager.LoadCookies("/non/existent/path/cookies.txt")
		if err == nil {
			t.Error("Expected error for non-existent file, got nil")
		}
		if !contains(err.Error(), "failed to open cookie file") {
			t.Errorf("Expected 'failed to open cookie file' error, got: %v", err)
		}
	})

	// Test directory instead of file
	t.Run("directory_instead_of_file", func(t *testing.T) {
		tmpDir := t.TempDir()
		_, err := authManager.LoadCookies(tmpDir)
		if err == nil {
			t.Error("Expected error when trying to read directory as file, got nil")
		}
	})
}

// TestCookieAuthManager_ValidateSessionComprehensive tests comprehensive session validation
func TestCookieAuthManager_ValidateSessionComprehensive(t *testing.T) {
	authManager := NewCookieAuthManager()

	tests := []struct {
		name        string
		authContext *internal.AuthContext
		expectError bool
		errorMsg    string
	}{
		{
			name: "BDUSS with invalid characters",
			authContext: &internal.AuthContext{
				BDUSS:     "invalid@bduss#with$special%chars^1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: true,
			errorMsg:    "BDUSS cookie format is invalid",
		},
		{
			name: "BDUSS exactly 32 characters",
			authContext: &internal.AuthContext{
				BDUSS:     "abcdef1234567890ABCDEF1234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: false,
		},
		{
			name: "BDUSS with 31 characters (too short)",
			authContext: &internal.AuthContext{
				BDUSS:     "abcdef1234567890ABCDEF123456789",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: true,
			errorMsg:    "BDUSS cookie appears to be invalid (too short)",
		},
		{
			name: "session expired exactly now",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now(),
			},
			expectError: true,
			errorMsg:    "session has expired",
		},
		{
			name: "session with zero expiration time (never expires)",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Time{},
			},
			expectError: false,
		},
		{
			name: "BDUSS with all allowed special characters",
			authContext: &internal.AuthContext{
				BDUSS:     "abc-DEF_123~456789012345678901234567890abcdef",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := authManager.ValidateSession(tt.authContext)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
			}
		})
	}
}

// TestCookieAuthManager_RefreshSessionEdgeCases tests edge cases for session refresh
func TestCookieAuthManager_RefreshSessionEdgeCases(t *testing.T) {
	authManager := NewCookieAuthManager()

	tests := []struct {
		name        string
		authContext *internal.AuthContext
		expectError bool
		errorMsg    string
		checkExpiry bool
	}{
		{
			name: "session expires in exactly 1 hour",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: true,
			errorMsg:    "session cannot be refreshed",
		},
		{
			name: "session expires in 1 hour and 1 minute",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(time.Hour + time.Minute),
			},
			expectError: false,
			checkExpiry: true,
		},
		{
			name: "already expired session",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Now().Add(-time.Hour),
			},
			expectError: true,
			errorMsg:    "session cannot be refreshed",
		},
		{
			name: "session with zero expiration time",
			authContext: &internal.AuthContext{
				BDUSS:     "valid_bduss_1234567890123456789012345678901234567890",
				STOKEN:    "valid_stoken_123456789012345678901234567890",
				ExpiresAt: time.Time{},
			},
			expectError: true,
			errorMsg:    "session cannot be refreshed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalExpiry := tt.authContext.ExpiresAt
			err := authManager.RefreshSession(tt.authContext)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.errorMsg)
				} else if !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if tt.checkExpiry && !tt.authContext.ExpiresAt.After(originalExpiry) {
					t.Error("Expected session expiry to be extended")
				}
			}
		})
	}
}

// TestCookieAuthManager_ConcurrentAccess tests thread safety
func TestCookieAuthManager_ConcurrentAccess(t *testing.T) {
	authManager := NewCookieAuthManager()
	tmpDir := t.TempDir()

	// Create test cookie files
	cookieContent := `# Netscape HTTP Cookie File
.terabox.com	TRUE	/	FALSE	1735689600	BDUSS	concurrent_bduss_1234567890123456789012345678901234567890
.terabox.com	TRUE	/	FALSE	1735689600	STOKEN	concurrent_stoken_123456789012345678901234567890
`

	// Test concurrent loading of cookies
	t.Run("concurrent_cookie_loading", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				cookieFile := filepath.Join(tmpDir, fmt.Sprintf("concurrent_%d_cookies.txt", id))
				err := os.WriteFile(cookieFile, []byte(cookieContent), 0644)
				if err != nil {
					errors <- err
					return
				}

				_, err = authManager.LoadCookies(cookieFile)
				if err != nil {
					errors <- err
					return
				}
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
				// Success
			case err := <-errors:
				t.Errorf("Concurrent access error: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for concurrent operations")
			}
		}
	})

	// Test concurrent cleanup
	t.Run("concurrent_cleanup", func(t *testing.T) {
		const numGoroutines = 5
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				authManager.Cleanup()
				done <- true
			}()
		}

		for i := 0; i < numGoroutines; i++ {
			select {
			case <-done:
				// Success
			case <-time.After(2 * time.Second):
				t.Fatal("Timeout waiting for concurrent cleanup operations")
			}
		}
	})
}

// TestIsValidBDUSSComprehensive tests comprehensive BDUSS validation
func TestIsValidBDUSSComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		bduss    string
		expected bool
	}{
		{
			name:     "minimum valid length (32 chars)",
			bduss:    "abcdefghijklmnopqrstuvwxyz123456",
			expected: true,
		},
		{
			name:     "one char too short (31 chars)",
			bduss:    "abcdefghijklmnopqrstuvwxyz12345",
			expected: false,
		},
		{
			name:     "very long valid BDUSS",
			bduss:    "abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890abcdefghijklmnopqrstuvwxyz",
			expected: true,
		},
		{
			name:     "contains space",
			bduss:    "abcdefghijklmnopqrstuvwxyz 123456",
			expected: false,
		},
		{
			name:     "contains tab",
			bduss:    "abcdefghijklmnopqrstuvwxyz\t123456",
			expected: false,
		},
		{
			name:     "contains newline",
			bduss:    "abcdefghijklmnopqrstuvwxyz\n123456",
			expected: false,
		},
		{
			name:     "contains plus sign",
			bduss:    "abcdefghijklmnopqrstuvwxyz+123456",
			expected: false,
		},
		{
			name:     "contains equals sign",
			bduss:    "abcdefghijklmnopqrstuvwxyz=123456",
			expected: false,
		},
		{
			name:     "contains forward slash",
			bduss:    "abcdefghijklmnopqrstuvwxyz/123456",
			expected: false,
		},
		{
			name:     "all uppercase",
			bduss:    "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456",
			expected: true,
		},
		{
			name:     "all lowercase",
			bduss:    "abcdefghijklmnopqrstuvwxyz123456",
			expected: true,
		},
		{
			name:     "all numbers",
			bduss:    "12345678901234567890123456789012",
			expected: true,
		},
		{
			name:     "mixed case with all allowed special chars",
			bduss:    "aBc-DeF_123~456XyZ789012345678901234567890",
			expected: true,
		},
		{
			name:     "unicode characters",
			bduss:    "abcdefghijklmnopqrstuvwxyz123456ñ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidBDUSS(tt.bduss)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v for BDUSS: %s (length: %d)", tt.expected, result, tt.bduss, len(tt.bduss))
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}