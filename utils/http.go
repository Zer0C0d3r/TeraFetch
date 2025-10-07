package utils

import (
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"terafetch/internal"
)

// RetryConfig defines retry behavior configuration
type RetryConfig struct {
	MaxAttempts   int
	BaseDelay     time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	JitterPercent float64
}

// DefaultRetryConfig returns the default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:   3,
		BaseDelay:     1 * time.Second,
		MaxDelay:      30 * time.Second,
		Multiplier:    2.0,
		JitterPercent: 0.1,
	}
}

// HTTPClientConfig contains configuration for the HTTP client
type HTTPClientConfig struct {
	Timeout     time.Duration
	ProxyURL    string
	RetryConfig *RetryConfig
}

// HTTPClient provides a custom HTTP client with retry logic and user-agent rotation
type HTTPClient struct {
	client       *http.Client
	userAgent    string
	userAgents   []string
	userAgentIdx int
	mutex        sync.RWMutex
	retryConfig  *RetryConfig
}

// Predefined user agent strings for rotation
var defaultUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/120.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:109.0) Gecko/20100101 Firefox/120.0",
	"Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/120.0",
}

// NewHTTPClient creates a new HTTP client with default configuration
func NewHTTPClient() *HTTPClient {
	return NewHTTPClientWithConfig(&HTTPClientConfig{
		Timeout:     30 * time.Second,
		RetryConfig: DefaultRetryConfig(),
	})
}

// NewHTTPClientWithConfig creates a new HTTP client with custom configuration
func NewHTTPClientWithConfig(config *HTTPClientConfig) *HTTPClient {
	if config.RetryConfig == nil {
		config.RetryConfig = DefaultRetryConfig()
	}

	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}

	// Configure proxy if provided
	if config.ProxyURL != "" {
		if err := configureProxy(transport, config.ProxyURL); err != nil {
			// Log error but continue without proxy
			fmt.Printf("Warning: Failed to configure proxy %s: %v\n", config.ProxyURL, err)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	return &HTTPClient{
		client:      client,
		userAgents:  make([]string, len(defaultUserAgents)),
		userAgent:   defaultUserAgents[0],
		retryConfig: config.RetryConfig,
	}
}

// configureProxy sets up proxy configuration for the transport
func configureProxy(transport *http.Transport, proxyURL string) error {
	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return fmt.Errorf("invalid proxy URL: %w", err)
	}

	switch parsedURL.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(parsedURL)
	case "socks5":
		// Create SOCKS5 dialer
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, nil, proxy.Direct)
		if err != nil {
			return fmt.Errorf("failed to create SOCKS5 proxy: %w", err)
		}
		transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		}
	default:
		return fmt.Errorf("unsupported proxy scheme: %s", parsedURL.Scheme)
	}

	return nil
}

// Get performs a GET request with retry logic
func (c *HTTPClient) Get(url string) (*http.Response, error) {
	return c.GetWithHeaders(url, nil)
}

// GetWithHeaders performs a GET request with custom headers and retry logic
func (c *HTTPClient) GetWithHeaders(url string, headers map[string]string) (*http.Response, error) {
	return c.executeWithRetry(func() (*http.Response, error) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set User-Agent
		c.mutex.RLock()
		req.Header.Set("User-Agent", c.userAgent)
		c.mutex.RUnlock()

		// Set custom headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Set default headers for Terabox compatibility
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		// Don't set Accept-Encoding explicitly to allow Go's automatic gzip handling
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")

		return c.client.Do(req)
	})
}

// GetWithContext performs a GET request with context and retry logic
func (c *HTTPClient) GetWithContext(ctx context.Context, url string, headers map[string]string) (*http.Response, error) {
	return c.executeWithRetryContext(ctx, func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		// Set User-Agent
		c.mutex.RLock()
		req.Header.Set("User-Agent", c.userAgent)
		c.mutex.RUnlock()

		// Set custom headers
		for key, value := range headers {
			req.Header.Set(key, value)
		}

		// Set default headers for Terabox compatibility
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		// Don't set Accept-Encoding explicitly to allow Go's automatic gzip handling
		req.Header.Set("Connection", "keep-alive")

		return c.client.Do(req)
	})
}

// RotateUserAgent rotates to the next user agent string
func (c *HTTPClient) RotateUserAgent() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.userAgentIdx = (c.userAgentIdx + 1) % len(defaultUserAgents)
	c.userAgent = defaultUserAgents[c.userAgentIdx]
}

// GetCurrentUserAgent returns the current user agent string
func (c *HTTPClient) GetCurrentUserAgent() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.userAgent
}

// SetUserAgent sets a custom user agent string
func (c *HTTPClient) SetUserAgent(userAgent string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.userAgent = userAgent
}

// executeWithRetry executes a function with retry logic
func (c *HTTPClient) executeWithRetry(fn func() (*http.Response, error)) (*http.Response, error) {
	return c.executeWithRetryContext(context.Background(), fn)
}

// executeWithRetryContext executes a function with retry logic and context
func (c *HTTPClient) executeWithRetryContext(ctx context.Context, fn func() (*http.Response, error)) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt < c.retryConfig.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate delay with exponential backoff and jitter
			delay := c.calculateDelay(attempt)
			
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		resp, err := fn()
		if err != nil {
			lastErr = err
			
			// Check if error is retryable
			if !c.isRetryableError(err) {
				return nil, err
			}
			
			// Rotate user agent on certain errors
			if c.shouldRotateUserAgent(err, resp) {
				c.RotateUserAgent()
			}
			
			continue
		}

		// Check HTTP status codes
		if resp != nil {
			switch resp.StatusCode {
			case http.StatusOK, http.StatusPartialContent:
				return resp, nil
			case http.StatusForbidden:
				// Rotate user agent and retry
				resp.Body.Close()
				c.RotateUserAgent()
				lastErr = internal.NewTeraboxError(resp.StatusCode, "Forbidden - rotating user agent", internal.ErrRateLimit)
				continue
			case http.StatusTooManyRequests:
				// Rate limited - use longer delay
				resp.Body.Close()
				lastErr = internal.NewTeraboxError(resp.StatusCode, "Rate limited", internal.ErrRateLimit)
				continue
			case http.StatusNotFound:
				resp.Body.Close()
				return nil, internal.NewTeraboxError(resp.StatusCode, "File not found", internal.ErrFileNotFound)
			case http.StatusUnauthorized:
				resp.Body.Close()
				return nil, internal.NewTeraboxError(resp.StatusCode, "Authentication required", internal.ErrAuthRequired)
			default:
				if resp.StatusCode >= 500 {
					// Server error - retry
					resp.Body.Close()
					lastErr = internal.NewTeraboxError(resp.StatusCode, "Server error", internal.ErrNetworkTimeout)
					continue
				}
				// Client error - don't retry
				resp.Body.Close()
				return nil, internal.NewTeraboxError(resp.StatusCode, "Client error", internal.ErrInvalidResponse)
			}
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.retryConfig.MaxAttempts, lastErr)
	}

	return nil, fmt.Errorf("request failed after %d attempts", c.retryConfig.MaxAttempts)
}

// calculateDelay calculates the delay for the next retry attempt
func (c *HTTPClient) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * multiplier^(attempt-1)
	delay := float64(c.retryConfig.BaseDelay) * math.Pow(c.retryConfig.Multiplier, float64(attempt-1))
	
	// Apply jitter (random variation)
	jitter := delay * c.retryConfig.JitterPercent * (rand.Float64()*2 - 1) // -jitterPercent to +jitterPercent
	delay += jitter
	
	// Ensure delay doesn't exceed maximum
	if delay > float64(c.retryConfig.MaxDelay) {
		delay = float64(c.retryConfig.MaxDelay)
	}
	
	// Ensure delay is not negative
	if delay < 0 {
		delay = float64(c.retryConfig.BaseDelay)
	}
	
	return time.Duration(delay)
}

// isRetryableError determines if an error should trigger a retry
func (c *HTTPClient) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for Terabox-specific errors
	if teraboxErr, ok := err.(*internal.TeraboxError); ok {
		return teraboxErr.IsRetryable()
	}

	// Check for network-related errors
	errStr := strings.ToLower(err.Error())
	retryableErrors := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"temporary failure",
		"i/o timeout",
		"context deadline exceeded",
	}

	for _, retryableErr := range retryableErrors {
		if strings.Contains(errStr, retryableErr) {
			return true
		}
	}

	return false
}

// shouldRotateUserAgent determines if user agent should be rotated based on error/response
func (c *HTTPClient) shouldRotateUserAgent(err error, resp *http.Response) bool {
	if resp != nil && resp.StatusCode == http.StatusForbidden {
		return true
	}

	if err != nil {
		errStr := strings.ToLower(err.Error())
		if strings.Contains(errStr, "forbidden") || strings.Contains(errStr, "blocked") {
			return true
		}
	}

	return false
}