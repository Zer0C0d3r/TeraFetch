package utils

import (
	"testing"

	"terafetch/internal"
)

func TestURLValidator_ValidateURL(t *testing.T) {
	validator := NewURLValidator()

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorType   internal.ErrorType
	}{
		{
			name:        "valid_terabox_url",
			url:         "https://terabox.com/s/1AbC123",
			expectError: false,
		},
		{
			name:        "valid_www_terabox_url",
			url:         "https://www.terabox.com/s/1AbC123",
			expectError: false,
		},
		{
			name:        "valid_pan_baidu_url",
			url:         "https://pan.baidu.com/s/1AbC123",
			expectError: false,
		},
		{
			name:        "valid_www_pan_baidu_url",
			url:         "https://www.pan.baidu.com/s/1AbC123",
			expectError: false,
		},
		{
			name:        "valid_http_url",
			url:         "http://terabox.com/s/1AbC123",
			expectError: false,
		},
		{
			name:        "empty_url",
			url:         "",
			expectError: true,
		},
		{
			name:        "invalid_domain",
			url:         "https://example.com/s/1AbC123",
			expectError: true,
			errorType:   internal.ErrInvalidURL,
		},
		{
			name:        "invalid_scheme",
			url:         "ftp://terabox.com/s/1AbC123",
			expectError: true,
		},
		{
			name:        "malformed_url",
			url:         "not-a-url",
			expectError: true,
		},
		{
			name:        "subdomain_not_allowed",
			url:         "https://api.terabox.com/s/1AbC123",
			expectError: true,
			errorType:   internal.ErrInvalidURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateURL(tt.url)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for URL %s, but got none", tt.url)
				}
				
				// Check error type if specified
				if tt.errorType != 0 {
					if teraboxErr, ok := err.(*internal.TeraboxError); ok {
						if teraboxErr.Type != tt.errorType {
							t.Errorf("expected error type %v, got %v", tt.errorType, teraboxErr.Type)
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for valid URL %s: %v", tt.url, err)
				}
			}
		})
	}
}

func TestURLValidator_ParseURL(t *testing.T) {
	validator := NewURLValidator()

	tests := []struct {
		name           string
		url            string
		expectedSurl   string
		expectedShareID string
		expectedPrivate bool
		expectError    bool
	}{
		{
			name:         "standard_terabox_share",
			url:          "https://terabox.com/s/1AbC123def",
			expectedSurl: "1AbC123def",
		},
		{
			name:         "terabox_share_with_query",
			url:          "https://terabox.com/s/1AbC123?pwd=1234",
			expectedSurl: "1AbC123",
		},
		{
			name:         "terabox_sharing_link",
			url:          "https://terabox.com/sharing/link?surl=AbC123&pwd=1234",
			expectedSurl: "AbC123",
		},
		{
			name:         "pan_baidu_share",
			url:          "https://pan.baidu.com/s/1AbC123def",
			expectedSurl: "1AbC123def",
		},
		{
			name:            "pan_baidu_shareid",
			url:             "https://pan.baidu.com/share/link?shareid=123456789&uk=987654321",
			expectedShareID: "123456789",
			expectedPrivate: true,
		},
		{
			name:            "terabox_private_file",
			url:             "https://terabox.com/web/share/link?surl=AbC123&path=/folder/file.txt",
			expectedSurl:    "AbC123",
			expectedPrivate: true,
		},
		{
			name:         "www_subdomain",
			url:          "https://www.terabox.com/s/1AbC123",
			expectedSurl: "1AbC123",
		},
		{
			name:        "invalid_domain",
			url:         "https://example.com/s/1AbC123",
			expectError: true,
		},
		{
			name:        "no_identifier",
			url:         "https://terabox.com/",
			expectError: true,
		},
		{
			name:        "malformed_url",
			url:         "not-a-url",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urlInfo, err := validator.ParseURL(tt.url)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for URL %s, but got none", tt.url)
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error for URL %s: %v", tt.url, err)
				return
			}
			
			if urlInfo.Surl != tt.expectedSurl {
				t.Errorf("expected surl %s, got %s", tt.expectedSurl, urlInfo.Surl)
			}
			
			if urlInfo.ShareID != tt.expectedShareID {
				t.Errorf("expected shareID %s, got %s", tt.expectedShareID, urlInfo.ShareID)
			}
			
			if urlInfo.IsPrivate != tt.expectedPrivate {
				t.Errorf("expected IsPrivate %t, got %t", tt.expectedPrivate, urlInfo.IsPrivate)
			}
			
			if urlInfo.OriginalURL != tt.url {
				t.Errorf("expected OriginalURL %s, got %s", tt.url, urlInfo.OriginalURL)
			}
		})
	}
}

func TestURLValidator_GetShareURL(t *testing.T) {
	validator := NewURLValidator()

	tests := []struct {
		name        string
		urlInfo     *URLInfo
		expectedURL string
	}{
		{
			name: "surl_normalization",
			urlInfo: &URLInfo{
				Surl: "AbC123",
			},
			expectedURL: "https://terabox.com/s/AbC123",
		},
		{
			name: "shareid_normalization",
			urlInfo: &URLInfo{
				ShareID: "123456789",
			},
			expectedURL: "https://pan.baidu.com/share/link?shareid=123456789",
		},
		{
			name: "fallback_to_original",
			urlInfo: &URLInfo{
				OriginalURL: "https://example.com/custom/path",
			},
			expectedURL: "https://example.com/custom/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.GetShareURL(tt.urlInfo)
			if result != tt.expectedURL {
				t.Errorf("expected %s, got %s", tt.expectedURL, result)
			}
		})
	}
}

func TestURLInfo_IsPublicShare(t *testing.T) {
	tests := []struct {
		name     string
		urlInfo  *URLInfo
		expected bool
	}{
		{
			name: "public_surl",
			urlInfo: &URLInfo{
				Surl:      "AbC123",
				IsPrivate: false,
			},
			expected: true,
		},
		{
			name: "public_shareid",
			urlInfo: &URLInfo{
				ShareID:   "123456789",
				IsPrivate: false,
			},
			expected: true,
		},
		{
			name: "private_surl",
			urlInfo: &URLInfo{
				Surl:      "AbC123",
				IsPrivate: true,
			},
			expected: false,
		},
		{
			name: "no_identifier",
			urlInfo: &URLInfo{
				IsPrivate: false,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.urlInfo.IsPublicShare()
			if result != tt.expected {
				t.Errorf("expected %t, got %t", tt.expected, result)
			}
		})
	}
}

func TestURLInfo_GetIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		urlInfo  *URLInfo
		expected string
	}{
		{
			name: "surl_priority",
			urlInfo: &URLInfo{
				Surl:    "AbC123",
				ShareID: "987654321",
			},
			expected: "AbC123",
		},
		{
			name: "shareid_fallback",
			urlInfo: &URLInfo{
				ShareID: "987654321",
			},
			expected: "987654321",
		},
		{
			name:     "no_identifier",
			urlInfo:  &URLInfo{},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.urlInfo.GetIdentifier()
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestURLValidator_EdgeCases(t *testing.T) {
	validator := NewURLValidator()

	// Test case-insensitive domain matching
	t.Run("case_insensitive_domain", func(t *testing.T) {
		urls := []string{
			"https://TERABOX.COM/s/1AbC123",
			"https://TeraBox.Com/s/1AbC123",
			"https://PAN.BAIDU.COM/s/1AbC123",
		}
		
		for _, url := range urls {
			err := validator.ValidateURL(url)
			if err != nil {
				t.Errorf("expected case-insensitive validation to pass for %s, got error: %v", url, err)
			}
		}
	})

	// Test URL with port numbers
	t.Run("url_with_port", func(t *testing.T) {
		err := validator.ValidateURL("https://terabox.com:443/s/1AbC123")
		if err != nil {
			t.Errorf("expected URL with port to be valid, got error: %v", err)
		}
	})

	// Test complex query parameters
	t.Run("complex_query_params", func(t *testing.T) {
		url := "https://terabox.com/sharing/link?surl=AbC123&pwd=1234&from=share&extra=param"
		urlInfo, err := validator.ParseURL(url)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if urlInfo.Surl != "AbC123" {
			t.Errorf("expected surl AbC123, got %s", urlInfo.Surl)
		}
	})
}

// TestURLValidator_ComprehensiveURLFormats tests various Terabox URL formats and edge cases
func TestURLValidator_ComprehensiveURLFormats(t *testing.T) {
	validator := NewURLValidator()

	tests := []struct {
		name            string
		url             string
		expectedSurl    string
		expectedShareID string
		expectedPrivate bool
		expectError     bool
		description     string
	}{
		// Standard Terabox formats
		{
			name:         "terabox_short_surl",
			url:          "https://terabox.com/s/1a",
			expectedSurl: "1a",
			description:  "Minimum length surl",
		},
		{
			name:         "terabox_long_surl",
			url:          "https://terabox.com/s/1AbCdEfGhIjKlMnOpQrStUvWxYz123456789_-",
			expectedSurl: "1AbCdEfGhIjKlMnOpQrStUvWxYz123456789_-",
			description:  "Maximum length surl with special characters",
		},
		{
			name:        "terabox_with_fragment",
			url:         "https://terabox.com/s/1AbC123#section",
			expectError: true,
			description: "URL with fragment identifier (not supported by regex patterns)",
		},
		{
			name:         "terabox_with_multiple_query_params",
			url:          "https://terabox.com/s/1AbC123?pwd=test&lang=en&ref=direct",
			expectedSurl: "1AbC123",
			description:  "URL with multiple query parameters",
		},
		
		// Baidu Pan formats
		{
			name:            "baidu_numeric_shareid",
			url:             "https://pan.baidu.com/share/link?shareid=1234567890123456&uk=9876543210",
			expectedShareID: "1234567890123456",
			expectedPrivate: true,
			description:     "Baidu pan with long numeric shareid",
		},
		{
			name:         "baidu_alphanumeric_surl",
			url:          "https://pan.baidu.com/s/1AbC123_-xyz",
			expectedSurl: "1AbC123_-xyz",
			description:  "Baidu pan with alphanumeric surl containing underscores and hyphens",
		},
		
		// Private/authenticated URLs
		{
			name:            "terabox_private_with_path",
			url:             "https://terabox.com/web/share/link?surl=AbC123&path=/Documents/file.pdf",
			expectedSurl:    "AbC123",
			expectedPrivate: true,
			description:     "Private Terabox URL with file path",
		},
		{
			name:            "terabox_private_root_path",
			url:             "https://terabox.com/web/share/link?surl=AbC123&path=/",
			expectedSurl:    "AbC123",
			expectedPrivate: false,
			description:     "Terabox URL with root path (not considered private)",
		},
		
		// Edge cases and error conditions
		{
			name:        "empty_surl",
			url:         "https://terabox.com/s/",
			expectError: true,
			description: "URL with empty surl parameter",
		},
		{
			name:        "invalid_characters_in_surl",
			url:         "https://terabox.com/s/1AbC@#$%",
			expectError: true,
			description: "URL with invalid characters in surl",
		},
		{
			name:        "missing_surl_and_shareid",
			url:         "https://terabox.com/sharing/link?pwd=1234",
			expectError: true,
			description: "URL missing both surl and shareid parameters",
		},
		{
			name:            "non_numeric_shareid_accepted",
			url:             "https://pan.baidu.com/share/link?shareid=abc123&uk=456",
			expectedShareID: "abc123",
			expectedPrivate: true,
			description:     "Baidu pan URL with non-numeric shareid (accepted by current implementation)",
		},
		
		// Protocol variations
		{
			name:         "http_protocol",
			url:          "http://terabox.com/s/1AbC123",
			expectedSurl: "1AbC123",
			description:  "HTTP protocol (non-HTTPS)",
		},
		
		// Subdomain variations
		{
			name:         "www_terabox",
			url:          "https://www.terabox.com/s/1AbC123",
			expectedSurl: "1AbC123",
			description:  "WWW subdomain for Terabox",
		},
		{
			name:         "www_baidu",
			url:          "https://www.pan.baidu.com/s/1AbC123",
			expectedSurl: "1AbC123",
			description:  "WWW subdomain for Baidu Pan",
		},
		
		// URL encoding
		{
			name:         "url_encoded_params",
			url:          "https://terabox.com/sharing/link?surl=AbC123&pwd=test%20password",
			expectedSurl: "AbC123",
			description:  "URL with encoded query parameters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urlInfo, err := validator.ParseURL(tt.url)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for %s (%s), but got none", tt.url, tt.description)
				}
				return
			}
			
			if err != nil {
				t.Errorf("unexpected error for %s (%s): %v", tt.url, tt.description, err)
				return
			}
			
			if urlInfo.Surl != tt.expectedSurl {
				t.Errorf("expected surl '%s', got '%s' for %s", tt.expectedSurl, urlInfo.Surl, tt.description)
			}
			
			if urlInfo.ShareID != tt.expectedShareID {
				t.Errorf("expected shareID '%s', got '%s' for %s", tt.expectedShareID, urlInfo.ShareID, tt.description)
			}
			
			if urlInfo.IsPrivate != tt.expectedPrivate {
				t.Errorf("expected IsPrivate %t, got %t for %s", tt.expectedPrivate, urlInfo.IsPrivate, tt.description)
			}
		})
	}
}

// TestURLValidator_DomainValidation tests comprehensive domain validation
func TestURLValidator_DomainValidation(t *testing.T) {
	validator := NewURLValidator()

	tests := []struct {
		name        string
		url         string
		expectError bool
		description string
	}{
		// Valid domains
		{
			name:        "terabox_com",
			url:         "https://terabox.com/s/1AbC123",
			expectError: false,
			description: "Standard terabox.com domain",
		},
		{
			name:        "www_terabox_com",
			url:         "https://www.terabox.com/s/1AbC123",
			expectError: false,
			description: "WWW subdomain for terabox.com",
		},
		{
			name:        "pan_baidu_com",
			url:         "https://pan.baidu.com/s/1AbC123",
			expectError: false,
			description: "Standard pan.baidu.com domain",
		},
		{
			name:        "www_pan_baidu_com",
			url:         "https://www.pan.baidu.com/s/1AbC123",
			expectError: false,
			description: "WWW subdomain for pan.baidu.com",
		},
		
		// Invalid domains
		{
			name:        "invalid_subdomain_terabox",
			url:         "https://api.terabox.com/s/1AbC123",
			expectError: true,
			description: "Invalid subdomain for terabox.com",
		},
		{
			name:        "invalid_subdomain_baidu",
			url:         "https://drive.baidu.com/s/1AbC123",
			expectError: true,
			description: "Invalid subdomain for baidu.com",
		},
		{
			name:        "completely_different_domain",
			url:         "https://dropbox.com/s/1AbC123",
			expectError: true,
			description: "Completely different domain",
		},
		{
			name:        "similar_domain_typo",
			url:         "https://terrabox.com/s/1AbC123",
			expectError: true,
			description: "Typo in domain name",
		},
		{
			name:        "baidu_without_pan",
			url:         "https://baidu.com/s/1AbC123",
			expectError: true,
			description: "Baidu domain without pan subdomain",
		},
		
		// Protocol validation
		{
			name:        "ftp_protocol",
			url:         "ftp://terabox.com/s/1AbC123",
			expectError: true,
			description: "FTP protocol not allowed",
		},
		{
			name:        "file_protocol",
			url:         "file://terabox.com/s/1AbC123",
			expectError: true,
			description: "File protocol not allowed",
		},
		{
			name:        "no_protocol",
			url:         "terabox.com/s/1AbC123",
			expectError: true,
			description: "Missing protocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateURL(tt.url)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error for %s (%s), but got none", tt.url, tt.description)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for %s (%s): %v", tt.url, tt.description, err)
				}
			}
		})
	}
}

// TestURLValidator_ParameterExtraction tests parameter extraction edge cases
func TestURLValidator_ParameterExtraction(t *testing.T) {
	validator := NewURLValidator()

	tests := []struct {
		name            string
		url             string
		expectedSurl    string
		expectedShareID string
		expectedPrivate bool
		description     string
	}{
		{
			name:         "surl_in_path_and_query",
			url:          "https://terabox.com/s/PathSurl?surl=QuerySurl",
			expectedSurl: "QuerySurl",
			description:  "Surl in both path and query - query parameter overrides path",
		},
		{
			name:            "both_surl_and_shareid",
			url:             "https://pan.baidu.com/share/link?surl=AbC123&shareid=987654321",
			expectedSurl:    "AbC123",
			expectedShareID: "987654321",
			description:     "Both surl and shareid present - both are extracted",
		},
		{
			name:            "uk_parameter_makes_private",
			url:             "https://pan.baidu.com/share/link?shareid=123456789&uk=user123",
			expectedShareID: "123456789",
			expectedPrivate: true,
			description:     "UK parameter indicates private access",
		},
		{
			name:            "empty_path_not_private",
			url:             "https://terabox.com/web/share/link?surl=AbC123&path=",
			expectedSurl:    "AbC123",
			expectedPrivate: false,
			description:     "Empty path parameter should not mark as private",
		},
		{
			name:            "root_path_not_private",
			url:             "https://terabox.com/web/share/link?surl=AbC123&path=/",
			expectedSurl:    "AbC123",
			expectedPrivate: false,
			description:     "Root path should not mark as private",
		},
		{
			name:            "nested_path_is_private",
			url:             "https://terabox.com/web/share/link?surl=AbC123&path=/folder/subfolder",
			expectedSurl:    "AbC123",
			expectedPrivate: true,
			description:     "Nested path should mark as private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urlInfo, err := validator.ParseURL(tt.url)
			if err != nil {
				t.Errorf("unexpected error for %s (%s): %v", tt.url, tt.description, err)
				return
			}
			
			if urlInfo.Surl != tt.expectedSurl {
				t.Errorf("expected surl '%s', got '%s' for %s", tt.expectedSurl, urlInfo.Surl, tt.description)
			}
			
			if urlInfo.ShareID != tt.expectedShareID {
				t.Errorf("expected shareID '%s', got '%s' for %s", tt.expectedShareID, urlInfo.ShareID, tt.description)
			}
			
			if urlInfo.IsPrivate != tt.expectedPrivate {
				t.Errorf("expected IsPrivate %t, got %t for %s", tt.expectedPrivate, urlInfo.IsPrivate, tt.description)
			}
		})
	}
}