package utils

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient()
	
	if client == nil {
		t.Fatal("NewHTTPClient returned nil")
	}
	
	if client.client.Timeout != 30*time.Second {
		t.Errorf("Expected timeout of 30s, got %v", client.client.Timeout)
	}
	
	userAgent := client.GetCurrentUserAgent()
	if userAgent == "" {
		t.Error("User agent should not be empty")
	}
}

func TestNewHTTPClientWithConfig(t *testing.T) {
	config := &HTTPClientConfig{
		Timeout: 10 * time.Second,
		RetryConfig: &RetryConfig{
			MaxAttempts: 5,
			BaseDelay:   500 * time.Millisecond,
		},
	}
	
	client := NewHTTPClientWithConfig(config)
	
	if client == nil {
		t.Fatal("NewHTTPClientWithConfig returned nil")
	}
	
	if client.client.Timeout != 10*time.Second {
		t.Errorf("Expected timeout of 10s, got %v", client.client.Timeout)
	}
	
	if client.retryConfig.MaxAttempts != 5 {
		t.Errorf("Expected max attempts of 5, got %d", client.retryConfig.MaxAttempts)
	}
}

func TestUserAgentRotation(t *testing.T) {
	client := NewHTTPClient()
	
	initialUA := client.GetCurrentUserAgent()
	client.RotateUserAgent()
	rotatedUA := client.GetCurrentUserAgent()
	
	if initialUA == rotatedUA {
		t.Error("User agent should change after rotation")
	}
	
	// Test that we cycle through all user agents
	seenUAs := make(map[string]bool)
	seenUAs[initialUA] = true
	
	for i := 0; i < len(defaultUserAgents); i++ {
		client.RotateUserAgent()
		ua := client.GetCurrentUserAgent()
		seenUAs[ua] = true
	}
	
	if len(seenUAs) < 3 { // Should see at least a few different user agents
		t.Errorf("Expected to see multiple user agents, only saw %d", len(seenUAs))
	}
}

func TestSetUserAgent(t *testing.T) {
	client := NewHTTPClient()
	customUA := "Custom User Agent"
	
	client.SetUserAgent(customUA)
	
	if client.GetCurrentUserAgent() != customUA {
		t.Errorf("Expected user agent %q, got %q", customUA, client.GetCurrentUserAgent())
	}
}

func TestHTTPClientGet(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		
		userAgent := r.Header.Get("User-Agent")
		if userAgent == "" {
			t.Error("User-Agent header should be set")
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()
	
	client := NewHTTPClient()
	resp, err := client.Get(server.URL)
	
	if err != nil {
		t.Fatalf("GET request failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHTTPClientGetWithHeaders(t *testing.T) {
	expectedHeaders := map[string]string{
		"X-Custom-Header": "test-value",
		"Referer":         "https://terabox.com/",
	}
	
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for key, expectedValue := range expectedHeaders {
			actualValue := r.Header.Get(key)
			if actualValue != expectedValue {
				t.Errorf("Expected header %s: %s, got %s", key, expectedValue, actualValue)
			}
		}
		
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	}))
	defer server.Close()
	
	client := NewHTTPClient()
	resp, err := client.GetWithHeaders(server.URL, expectedHeaders)
	
	if err != nil {
		t.Fatalf("GET request with headers failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestHTTPClientRetryLogic(t *testing.T) {
	attempts := 0
	
	// Create a test server that fails twice then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()
	
	config := &HTTPClientConfig{
		Timeout: 5 * time.Second,
		RetryConfig: &RetryConfig{
			MaxAttempts: 3,
			BaseDelay:   10 * time.Millisecond, // Short delay for testing
			MaxDelay:    100 * time.Millisecond,
			Multiplier:  2.0,
		},
	}
	
	client := NewHTTPClientWithConfig(config)
	resp, err := client.Get(server.URL)
	
	if err != nil {
		t.Fatalf("Request should have succeeded after retries: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestHTTPClientUserAgentRotationOnForbidden(t *testing.T) {
	attempts := 0
	
	// Create a test server that returns 403 twice then succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()
	
	client := NewHTTPClient()
	initialUA := client.GetCurrentUserAgent()
	
	resp, err := client.Get(server.URL)
	
	if err != nil {
		t.Fatalf("Request should have succeeded after retries: %v", err)
	}
	defer resp.Body.Close()
	
	finalUA := client.GetCurrentUserAgent()
	if initialUA == finalUA {
		t.Error("User agent should have been rotated after 403 responses")
	}
}

func TestHTTPClientContext(t *testing.T) {
	// Create a test server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()
	
	client := NewHTTPClient()
	
	// Test with context that times out
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	_, err := client.GetWithContext(ctx, server.URL, nil)
	
	if err == nil {
		t.Error("Request should have failed due to context timeout")
	}
	
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

func TestCalculateDelay(t *testing.T) {
	config := &RetryConfig{
		BaseDelay:     1 * time.Second,
		MaxDelay:      10 * time.Second,
		Multiplier:    2.0,
		JitterPercent: 0.1,
	}
	
	client := &HTTPClient{retryConfig: config}
	
	// Test first retry (attempt 1)
	delay1 := client.calculateDelay(1)
	expectedMin := time.Duration(float64(config.BaseDelay) * 0.9) // Base - 10% jitter
	expectedMax := time.Duration(float64(config.BaseDelay) * 1.1) // Base + 10% jitter
	
	if delay1 < expectedMin || delay1 > expectedMax {
		t.Errorf("First retry delay %v should be between %v and %v", delay1, expectedMin, expectedMax)
	}
	
	// Test that delay increases with attempts
	delay2 := client.calculateDelay(2)
	delay3 := client.calculateDelay(3)
	
	// Note: Due to jitter, we can't guarantee strict ordering, but the base should increase
	if delay2 < config.BaseDelay || delay3 < config.BaseDelay*2 {
		t.Error("Delays should generally increase with attempt number")
	}
	
	// Test max delay cap
	delay10 := client.calculateDelay(10)
	if delay10 > config.MaxDelay*11/10 { // Allow for jitter
		t.Errorf("Delay %v should not exceed max delay %v (with jitter)", delay10, config.MaxDelay)
	}
}

func TestIsRetryableError(t *testing.T) {
	client := NewHTTPClient()
	
	// Test non-retryable errors
	nonRetryableErrors := []error{
		nil,
	}
	
	// Test with a network timeout error
	httpClient := &http.Client{Timeout: 1 * time.Nanosecond}
	// Create a request that will timeout
	req, _ := http.NewRequest("GET", "http://192.0.2.1:1", nil) // Non-routable IP
	_, netErr := httpClient.Do(req)
	
	if netErr != nil && !client.isRetryableError(netErr) {
		t.Errorf("Network error should be retryable: %v", netErr)
	}
	
	for _, err := range nonRetryableErrors {
		if client.isRetryableError(err) {
			t.Errorf("Error should not be retryable: %v", err)
		}
	}
}