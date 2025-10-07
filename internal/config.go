package internal

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds application configuration
type Config struct {
	DefaultThreads   int
	DefaultTimeout   int
	MaxRetries      int
	UserAgentList   []string
	AllowedDomains  []string
	
	// Logging configuration
	LogLevel        string
	EnableDebug     bool
	QuietMode       bool
	LogFile         string
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		DefaultThreads: 8,
		DefaultTimeout: 30,
		MaxRetries:     3,
		UserAgentList: []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
		AllowedDomains: []string{
			"terabox.com",
			"www.terabox.com",
			"pan.baidu.com",
			"www.pan.baidu.com",
		},
		
		// Logging defaults
		LogLevel:    "info",
		EnableDebug: false,
		QuietMode:   false,
		LogFile:     "", // Empty means stderr
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() {
	if threads := os.Getenv("TERAFETCH_THREADS"); threads != "" {
		if t, err := strconv.Atoi(threads); err == nil && t > 0 && t <= 32 {
			c.DefaultThreads = t
		}
	}
	
	if timeout := os.Getenv("TERAFETCH_TIMEOUT"); timeout != "" {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			c.DefaultTimeout = t
		}
	}
	
	// Load logging configuration from environment
	if logLevel := os.Getenv("TERAFETCH_LOG_LEVEL"); logLevel != "" {
		c.LogLevel = logLevel
	}
	
	if debug := os.Getenv("TERAFETCH_DEBUG"); debug != "" {
		c.EnableDebug = debug == "true" || debug == "1"
	}
	
	if quiet := os.Getenv("TERAFETCH_QUIET"); quiet != "" {
		c.QuietMode = quiet == "true" || quiet == "1"
	}
	
	if logFile := os.Getenv("TERAFETCH_LOG_FILE"); logFile != "" {
		c.LogFile = logFile
	}
}

// GetEnvWithDefault returns environment variable value or default
func GetEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ValidateConfig validates the configuration values
func (c *Config) ValidateConfig() error {
	if c.DefaultThreads < 1 || c.DefaultThreads > 32 {
		return fmt.Errorf("invalid default threads: %d (must be 1-32)", c.DefaultThreads)
	}
	
	if c.DefaultTimeout < 1 {
		return fmt.Errorf("invalid default timeout: %d (must be > 0)", c.DefaultTimeout)
	}
	
	if c.MaxRetries < 0 {
		return fmt.Errorf("invalid max retries: %d (must be >= 0)", c.MaxRetries)
	}
	
	if len(c.UserAgentList) == 0 {
		return fmt.Errorf("user agent list cannot be empty")
	}
	
	if len(c.AllowedDomains) == 0 {
		return fmt.Errorf("allowed domains list cannot be empty")
	}
	
	return nil
}