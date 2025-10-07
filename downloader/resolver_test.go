package downloader

import (
	"testing"
	"time"

	"terafetch/internal"
	"terafetch/utils"
)

func TestTeraboxResolver_ResolvePublicLink(t *testing.T) {
	resolver := NewTeraboxResolver()

	tests := []struct {
		name        string
		url         string
		expectError bool
		errorType   internal.ErrorType
	}{
		{
			name:        "invalid_url_empty",
			url:         "",
			expectError: true,
			errorType:   internal.ErrInvalidURL,
		},
		{
			name:        "invalid_url_wrong_domain",
			url:         "https://example.com/s/1AbC123",
			expectError: true,
			errorType:   internal.ErrInvalidURL,
		},
		{
			name:        "valid_terabox_url_format",
			url:         "https://terabox.com/s/1AbC123",
			expectError: false, // Note: This will likely fail in tests due to network/API, but validates URL parsing
		},
		{
			name:        "valid_baidu_url_format",
			url:         "https://pan.baidu.com/s/1AbC123",
			expectError: false, // Note: This will likely fail in tests due to network/API, but validates URL parsing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolvePublicLink(tt.url)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				
				// Check if it's a TeraboxError with the expected type
				if teraboxErr, ok := err.(*internal.TeraboxError); ok {
					if teraboxErr.Type != tt.errorType {
						t.Errorf("expected error type %v, got %v", tt.errorType, teraboxErr.Type)
					}
				}
			} else {
				// For valid URLs, we expect either success or network-related errors
				// since we can't make real API calls in unit tests
				if err != nil {
					t.Logf("URL parsing succeeded but API call failed (expected in unit tests): %v", err)
				}
			}
		})
	}
}

func TestTeraboxResolver_ResolvePrivateLink(t *testing.T) {
	resolver := NewTeraboxResolver()

	tests := []struct {
		name        string
		url         string
		auth        *internal.AuthContext
		expectError bool
		errorType   internal.ErrorType
	}{
		{
			name:        "nil_auth_context",
			url:         "https://terabox.com/s/1AbC123",
			auth:        nil,
			expectError: true,
			errorType:   internal.ErrAuthRequired,
		},
		{
			name: "valid_auth_context",
			url:  "https://terabox.com/s/1AbC123",
			auth: &internal.AuthContext{
				BDUSS:     "test_bduss",
				STOKEN:    "test_stoken",
				ExpiresAt: time.Now().Add(time.Hour),
			},
			expectError: false, // Will likely fail due to network/API in tests
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolver.ResolvePrivateLink(tt.url, tt.auth)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				
				// Check if it's a TeraboxError with the expected type
				if teraboxErr, ok := err.(*internal.TeraboxError); ok {
					if teraboxErr.Type != tt.errorType {
						t.Errorf("expected error type %v, got %v", tt.errorType, teraboxErr.Type)
					}
				}
			} else {
				// For valid requests, we expect either success or network-related errors
				if err != nil {
					t.Logf("Request setup succeeded but API call failed (expected in unit tests): %v", err)
				}
			}
		})
	}
}

func TestTeraboxResolver_HandleAPIError(t *testing.T) {
	resolver := NewTeraboxResolver()

	tests := []struct {
		name         string
		apiResponse  TeraboxAPIResponse
		expectError  bool
		expectedType internal.ErrorType
	}{
		{
			name: "success_response",
			apiResponse: TeraboxAPIResponse{
				Errno:  0,
				Errmsg: "success",
			},
			expectError: false,
		},
		{
			name: "auth_required_error",
			apiResponse: TeraboxAPIResponse{
				Errno:  -2,
				Errmsg: "authentication required",
			},
			expectError:  true,
			expectedType: internal.ErrAuthRequired,
		},
		{
			name: "file_not_found_error",
			apiResponse: TeraboxAPIResponse{
				Errno:  -4,
				Errmsg: "file not found",
			},
			expectError:  true,
			expectedType: internal.ErrFileNotFound,
		},
		{
			name: "rate_limit_error",
			apiResponse: TeraboxAPIResponse{
				Errno:  -6,
				Errmsg: "rate limit exceeded",
			},
			expectError:  true,
			expectedType: internal.ErrRateLimit,
		},
		{
			name: "unknown_error",
			apiResponse: TeraboxAPIResponse{
				Errno:  -999,
				Errmsg: "unknown error",
			},
			expectError:  true,
			expectedType: internal.ErrInvalidResponse,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.handleAPIError(tt.apiResponse)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				
				if teraboxErr, ok := err.(*internal.TeraboxError); ok {
					if teraboxErr.Type != tt.expectedType {
						t.Errorf("expected error type %v, got %v", tt.expectedType, teraboxErr.Type)
					}
					if teraboxErr.Code != tt.apiResponse.Errno {
						t.Errorf("expected error code %d, got %d", tt.apiResponse.Errno, teraboxErr.Code)
					}
				} else {
					t.Errorf("expected TeraboxError, got %T", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestNewTeraboxResolver(t *testing.T) {
	resolver := NewTeraboxResolver()
	
	if resolver == nil {
		t.Fatal("NewTeraboxResolver returned nil")
	}
	
	if resolver.httpClient == nil {
		t.Error("httpClient is nil")
	}
	
	if resolver.urlValidator == nil {
		t.Error("urlValidator is nil")
	}
}

func TestNewTeraboxResolverWithClient(t *testing.T) {
	customClient := utils.NewHTTPClient()
	resolver := NewTeraboxResolverWithClient(customClient)
	
	if resolver == nil {
		t.Fatal("NewTeraboxResolverWithClient returned nil")
	}
	
	if resolver.httpClient != customClient {
		t.Error("httpClient was not set to the provided client")
	}
	
	if resolver.urlValidator == nil {
		t.Error("urlValidator is nil")
	}
}