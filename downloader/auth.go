package downloader

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"terafetch/internal"
)

// CookieAuthManager implements the AuthManager interface with secure cookie handling
type CookieAuthManager struct {
	// Secure in-memory cookie storage
	cookieStore map[string]*http.Cookie
	mutex       sync.RWMutex
}

// NewCookieAuthManager creates a new instance of CookieAuthManager
func NewCookieAuthManager() *CookieAuthManager {
	return &CookieAuthManager{
		cookieStore: make(map[string]*http.Cookie),
	}
}

// LoadCookies loads cookies from a Netscape-format file
func (a *CookieAuthManager) LoadCookies(path string) (*internal.AuthContext, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open cookie file: %w", err)
	}
	defer file.Close()

	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Clear existing cookies for security
	a.clearCookies()

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		cookie, err := a.parseNetscapeCookieLine(line)
		if err != nil {
			return nil, fmt.Errorf("invalid cookie format at line %d: %w", lineNum, err)
		}

		// Store cookie securely in memory
		a.cookieStore[cookie.Name] = cookie
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading cookie file: %w", err)
	}

	// Create AuthContext from loaded cookies
	authContext := &internal.AuthContext{
		Cookies: make(map[string]*http.Cookie),
	}

	// Copy cookies to AuthContext and extract BDUSS/STOKEN
	for name, cookie := range a.cookieStore {
		authContext.Cookies[name] = cookie

		switch name {
		case "BDUSS":
			authContext.BDUSS = cookie.Value
		case "STOKEN":
			authContext.STOKEN = cookie.Value
		}
	}

	// Set expiration time based on cookie expiration
	if bdussCookie, exists := authContext.Cookies["BDUSS"]; exists && !bdussCookie.Expires.IsZero() {
		authContext.ExpiresAt = bdussCookie.Expires
	} else {
		// Default expiration if no explicit expiry
		authContext.ExpiresAt = time.Now().Add(24 * time.Hour)
	}

	return authContext, nil
}

// parseNetscapeCookieLine parses a single line from Netscape cookie format
// Format: domain	flag	path	secure	expiration	name	value
func (a *CookieAuthManager) parseNetscapeCookieLine(line string) (*http.Cookie, error) {
	fields := strings.Split(line, "\t")
	if len(fields) != 7 {
		return nil, fmt.Errorf("expected 7 fields, got %d", len(fields))
	}

	domain := fields[0]
	path := fields[2]
	secureStr := fields[3]
	expirationStr := fields[4]
	name := fields[5]
	value := fields[6]

	// Parse secure flag
	secure := secureStr == "TRUE"

	// Parse expiration timestamp
	var expires time.Time
	if expirationStr != "0" {
		timestamp, err := strconv.ParseInt(expirationStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expiration timestamp: %w", err)
		}
		expires = time.Unix(timestamp, 0)
	}

	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Domain:   domain,
		Path:     path,
		Expires:  expires,
		Secure:   secure,
		HttpOnly: true, // Default to HttpOnly for security
	}

	return cookie, nil
}

// ValidateSession checks if the current session is valid by verifying BDUSS/STOKEN cookies
func (a *CookieAuthManager) ValidateSession(auth *internal.AuthContext) error {
	if auth == nil {
		return fmt.Errorf("auth context is nil")
	}

	// Check for required BDUSS cookie
	if auth.BDUSS == "" {
		return fmt.Errorf("BDUSS cookie is required for authentication")
	}

	// BDUSS should be at least 32 characters (typical Baidu session token length)
	if len(auth.BDUSS) < 32 {
		return fmt.Errorf("BDUSS cookie appears to be invalid (too short)")
	}

	// Check for STOKEN cookie (required for some operations)
	if auth.STOKEN == "" {
		return fmt.Errorf("STOKEN cookie is required for authentication")
	}

	// Check if session has expired
	if !auth.ExpiresAt.IsZero() && time.Now().After(auth.ExpiresAt) {
		return fmt.Errorf("session has expired at %v", auth.ExpiresAt)
	}

	// Validate cookie format - BDUSS should contain only alphanumeric and specific characters
	if !isValidBDUSS(auth.BDUSS) {
		return fmt.Errorf("BDUSS cookie format is invalid")
	}

	return nil
}

// RefreshSession attempts to refresh an expired session using available tokens
func (a *CookieAuthManager) RefreshSession(auth *internal.AuthContext) error {
	if auth == nil {
		return fmt.Errorf("auth context is nil")
	}

	// Check if we have the necessary tokens for refresh
	if auth.STOKEN == "" {
		return fmt.Errorf("STOKEN is required for session refresh")
	}

	// For Terabox/Baidu, session refresh typically requires making a request
	// to their token refresh endpoint with the current STOKEN
	// This is a simplified implementation - in practice, you'd need to:
	// 1. Make HTTP request to Baidu's token refresh endpoint
	// 2. Parse the response to get new tokens
	// 3. Update the AuthContext with new tokens and expiration

	// Since we don't have access to the actual refresh endpoint in this implementation,
	// we'll extend the current session if it's still within a reasonable timeframe
	if time.Now().Before(auth.ExpiresAt.Add(-1*time.Hour)) {
		// Session is still valid for at least 1 hour, extend it
		auth.ExpiresAt = time.Now().Add(24 * time.Hour)
		return nil
	}

	// If session is too close to expiry or already expired, we can't refresh it
	// without making actual API calls to Terabox
	return fmt.Errorf("session cannot be refreshed - please provide fresh cookies")
}

// CreateBypassAuthContext creates a minimal auth context for bypass attempts
func (a *CookieAuthManager) CreateBypassAuthContext() *internal.AuthContext {
	return &internal.AuthContext{
		BDUSS:     "bypass_mode",
		STOKEN:    "bypass_mode", 
		ExpiresAt: time.Now().Add(24 * time.Hour),
		UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		Bypass:    true,
	}
}

// clearCookies securely clears all stored cookies from memory
func (a *CookieAuthManager) clearCookies() {
	for name := range a.cookieStore {
		// Overwrite cookie value for security
		if cookie := a.cookieStore[name]; cookie != nil {
			cookie.Value = ""
		}
		delete(a.cookieStore, name)
	}
}

// isValidBDUSS validates the format of a BDUSS cookie value
func isValidBDUSS(bduss string) bool {
	// BDUSS should be alphanumeric with possible dashes and underscores
	// and should be at least 32 characters long
	if len(bduss) < 32 {
		return false
	}

	for _, char := range bduss {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' || char == '~') {
			return false
		}
	}

	return true
}

// Cleanup securely clears all sensitive data from memory
func (a *CookieAuthManager) Cleanup() {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	a.clearCookies()
}