package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"terafetch/downloader"
	"terafetch/internal"
	"terafetch/utils"
)

var (
	outputPath  string
	cookiesPath string
	threads     int
	rateLimit   string
	quiet       bool
	proxyURL    string
	debug       bool
	logLevel    string
	logFile     string
	bypassAuth  bool
	config      *internal.Config
)

var rootCmd = &cobra.Command{
	Use:     "terafetch [OPTIONS] <URL>",
	Short:   "Download files from Terabox with multi-threaded support",
	Version: "v1.0.0",
	Long: `TeraFetch is a production-grade CLI tool for downloading files from Terabox
with advanced features including multi-threaded downloads, authentication support,
and robust error handling.

Examples:
  terafetch https://terabox.com/s/1AbC123
  terafetch -o /path/to/file.zip -t 16 https://terabox.com/s/1AbC123
  terafetch -c cookies.txt -r 5M --proxy http://proxy:8080 https://terabox.com/s/1AbC123
  terafetch resume /path/to/file.zip.part

Environment Variables:
  TERAFETCH_THREADS     Default number of threads (1-32)
  TERAFETCH_TIMEOUT     HTTP timeout in seconds
  TERAFETCH_COOKIES     Path to cookie file
  TERAFETCH_PROXY       Proxy URL
  TERAFETCH_RATE_LIMIT  Default rate limit (e.g., 5M)

DISCLAIMER: Respect Terabox's Terms of Service and copyright laws.`,
	Args: cobra.ExactArgs(1),
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load and initialize configuration first
		if err := loadConfiguration(); err != nil {
			return fmt.Errorf("configuration error: %v", err)
		}
		
		// Initialize logging system
		if err := internal.InitLogger(config); err != nil {
			return fmt.Errorf("failed to initialize logger: %v", err)
		}
		
		// Log startup information
		internal.LogInfo("TeraFetch starting up")
		internal.LogDebug("Configuration loaded: threads=%d, timeout=%d, debug=%v, quiet=%v", 
			config.DefaultThreads, config.DefaultTimeout, config.EnableDebug, config.QuietMode)
		
		// Display ToS disclaimer
		if !quiet {
			fmt.Fprintln(os.Stderr, "⚠️  DISCLAIMER: Respect Terabox's Terms of Service and copyright laws.")
			fmt.Fprintln(os.Stderr, "")
		}
		
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		
		internal.LogInfo("Processing download request for URL: %s", url)
		
		// Validate arguments
		if err := validateArguments(url); err != nil {
			internal.LogError("Argument validation failed: %v", err)
			return err
		}
		
		// Validate and parse the URL
		validator := utils.NewURLValidator()
		urlInfo, err := validator.ParseURL(url)
		if err != nil {
			validationErr := internal.NewInvalidURLError(url, err.Error())
			internal.LogTeraboxError(validationErr)
			return fmt.Errorf("invalid URL: %v\n\nSupported URL formats:\n  - https://terabox.com/s/[share_id]\n  - https://www.terabox.com/s/[share_id]\n  - https://pan.baidu.com/s/[share_id]", err)
		}
		
		internal.LogDebug("URL parsed successfully: domain=%s, identifier=%s", urlInfo.Domain, urlInfo.GetIdentifier())
		
		// Parse rate limit if provided
		var rateLimitBytes int64
		if rateLimit != "" {
			rateLimitBytes, err = utils.ParseRateLimit(rateLimit)
			if err != nil {
				validationErr := internal.NewValidationErrorWithValue("rate_limit", "invalid format", rateLimit).
					WithSuggestion("Use formats like 1M (1 MB/s), 500K (500 KB/s), 2G (2 GB/s), or 1024 (1024 bytes/s)")
				internal.LogValidationError(validationErr)
				return fmt.Errorf("invalid rate limit format: %v\n\nSupported formats:\n  - 1M (1 MB/s)\n  - 500K (500 KB/s)\n  - 2G (2 GB/s)\n  - 1024 (1024 bytes/s)", err)
			}
			internal.LogDebug("Rate limit parsed: %s = %d bytes/sec", rateLimit, rateLimitBytes)
		}
		
		// Set default output path if not provided
		if outputPath == "" {
			outputPath = generateDefaultOutputPath(urlInfo)
		}
		
		// Validate output path
		if err := validateOutputPath(outputPath); err != nil {
			validationErr := internal.NewValidationErrorWithValue("output_path", err.Error(), outputPath)
			internal.LogValidationError(validationErr)
			return fmt.Errorf("invalid output path: %v", err)
		}
		internal.LogDebug("Output path validated: %s", outputPath)
		
		// Validate cookies file if provided
		if cookiesPath != "" {
			if err := validateCookiesFile(cookiesPath); err != nil {
				validationErr := internal.NewValidationErrorWithValue("cookies_file", err.Error(), cookiesPath).
					WithSuggestion("Ensure the file exists and is in Netscape cookie format")
				internal.LogValidationError(validationErr)
				return fmt.Errorf("invalid cookies file: %v", err)
			}
			internal.LogDebug("Cookies file validated: %s", cookiesPath)
		}
		
		// Validate proxy URL if provided
		if proxyURL != "" {
			if err := validateProxyURL(proxyURL); err != nil {
				validationErr := internal.NewValidationErrorWithValue("proxy_url", err.Error(), proxyURL).
					WithSuggestion("Use formats like http://proxy:8080, socks5://proxy:1080, or http://user:pass@proxy:8080")
				internal.LogValidationError(validationErr)
				return fmt.Errorf("invalid proxy URL: %v\n\nSupported formats:\n  - http://proxy.example.com:8080\n  - socks5://proxy.example.com:1080\n  - http://user:pass@proxy.example.com:8080", err)
			}
			internal.LogDebug("Proxy URL validated: %s", proxyURL)
		}
		
		if !quiet {
			fmt.Printf("📥 Downloading from: %s\n", url)
			fmt.Printf("📁 Output path: %s\n", outputPath)
			fmt.Printf("🧵 Threads: %d\n", threads)
			if rateLimitBytes > 0 {
				fmt.Printf("🚦 Rate limit: %s (%d bytes/sec)\n", rateLimit, rateLimitBytes)
			}
			if cookiesPath != "" {
				fmt.Printf("🍪 Using cookies from: %s\n", cookiesPath)
			}
			if proxyURL != "" {
				fmt.Printf("🌐 Using proxy: %s\n", proxyURL)
			}
			fmt.Println()
		}
		
		internal.LogInfo("Download configuration complete - ready to start download")
		internal.LogDebug("Final config: output=%s, threads=%d, rateLimit=%d, cookies=%s, proxy=%s", 
			outputPath, threads, rateLimitBytes, cookiesPath, proxyURL)
		
		// Execute the complete download workflow
		return executeDownloadWorkflow(url, outputPath, threads, rateLimitBytes, cookiesPath, proxyURL, quiet)
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume <PARTIAL_FILE_PATH>",
	Short: "Resume an interrupted download",
	Long: `Resume an interrupted download from a .part file.

The resume command will automatically detect the associated metadata file
and continue downloading from where it left off.

Examples:
  terafetch resume /path/to/file.zip.part
  terafetch resume -t 16 -r 5M /path/to/file.zip.part`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		partialPath := args[0]
		
		internal.LogInfo("Attempting to resume download from: %s", partialPath)
		
		// Validate partial file path
		if !strings.HasSuffix(partialPath, ".part") {
			return fmt.Errorf("file must have .part extension, got: %s", partialPath)
		}
		
		// Check if partial file exists
		if _, err := os.Stat(partialPath); os.IsNotExist(err) {
			return fmt.Errorf("partial file not found: %s", partialPath)
		}
		
		// Derive output path from partial path
		outputPath = strings.TrimSuffix(partialPath, ".part")
		
		// Parse rate limit if provided
		var rateLimitBytes int64
		var err error
		if rateLimit != "" {
			rateLimitBytes, err = utils.ParseRateLimit(rateLimit)
			if err != nil {
				return fmt.Errorf("invalid rate limit format: %v", err)
			}
		}
		
		if !quiet {
			fmt.Printf("🔄 Resuming download: %s\n", outputPath)
			fmt.Printf("🧵 Threads: %d\n", threads)
			if rateLimitBytes > 0 {
				fmt.Printf("🚦 Rate limit: %s (%d bytes/sec)\n", rateLimit, rateLimitBytes)
			}
			if cookiesPath != "" {
				fmt.Printf("🍪 Using cookies from: %s\n", cookiesPath)
			}
			if proxyURL != "" {
				fmt.Printf("🌐 Using proxy: %s\n", proxyURL)
			}
			fmt.Println()
		}
		
		internal.LogInfo("Resume configuration complete - ready to resume download")
		
		// Execute the resume workflow
		return executeResumeWorkflow(partialPath, threads, rateLimitBytes, cookiesPath, proxyURL, quiet)
	},
}

// loadConfiguration loads configuration from environment variables and merges with CLI flags
func loadConfiguration() error {
	config = internal.DefaultConfig()
	config.LoadFromEnv()
	
	// Override with environment variables if CLI flags are not set
	if threads == 8 { // Default value, check if env var should override
		if envThreads := os.Getenv("TERAFETCH_THREADS"); envThreads != "" {
			threads = config.DefaultThreads
		}
	}
	
	if cookiesPath == "" {
		cookiesPath = os.Getenv("TERAFETCH_COOKIES")
	}
	
	if proxyURL == "" {
		proxyURL = os.Getenv("TERAFETCH_PROXY")
	}
	
	if rateLimit == "" {
		rateLimit = os.Getenv("TERAFETCH_RATE_LIMIT")
	}
	
	if !bypassAuth {
		if envBypass := os.Getenv("TERAFETCH_BYPASS"); envBypass != "" {
			bypassAuth = strings.ToLower(envBypass) == "true" || envBypass == "1"
		}
	}
	
	// Update logging configuration based on CLI flags
	if debug {
		config.EnableDebug = true
		config.LogLevel = "debug"
	}
	
	if quiet {
		config.QuietMode = true
	}
	
	if logLevel != "" {
		config.LogLevel = logLevel
	}
	
	if logFile != "" {
		config.LogFile = logFile
	}
	
	// Validate configuration
	if err := config.ValidateConfig(); err != nil {
		return err
	}
	
	return nil
}

// validateArguments validates all CLI arguments and flags
func validateArguments(url string) error {
	// Validate URL format (basic check before detailed parsing)
	if url == "" {
		return fmt.Errorf("URL is required")
	}
	
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("URL must start with http:// or https://")
	}
	
	// Validate thread count
	if threads < 1 || threads > 32 {
		return fmt.Errorf("thread count must be between 1 and 32, got %d", threads)
	}
	
	return nil
}

// generateDefaultOutputPath generates a default output filename based on the URL
func generateDefaultOutputPath(urlInfo *utils.URLInfo) string {
	// Use share identifier as base filename
	identifier := urlInfo.GetIdentifier()
	if identifier == "" {
		identifier = "terabox_download"
	}
	
	// Add timestamp to make it unique
	return fmt.Sprintf("%s_download", identifier)
}

// validateOutputPath validates the output path
func validateOutputPath(path string) error {
	if path == "" {
		return fmt.Errorf("output path cannot be empty")
	}
	
	// Check if directory exists and is writable
	dir := filepath.Dir(path)
	if dir != "." {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("output directory does not exist: %s", dir)
		}
	}
	
	// Check if we can write to the directory
	testFile := filepath.Join(dir, ".terafetch_write_test")
	if f, err := os.Create(testFile); err != nil {
		return fmt.Errorf("cannot write to output directory: %v", err)
	} else {
		f.Close()
		os.Remove(testFile)
	}
	
	return nil
}

// validateCookiesFile validates the cookies file path and format
func validateCookiesFile(path string) error {
	if path == "" {
		return fmt.Errorf("cookies file path cannot be empty")
	}
	
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("cookies file does not exist: %s", path)
	}
	
	// Check if file is readable
	if f, err := os.Open(path); err != nil {
		return fmt.Errorf("cannot read cookies file: %v", err)
	} else {
		f.Close()
	}
	
	return nil
}

// validateProxyURL validates the proxy URL format
func validateProxyURL(proxyURL string) error {
	if proxyURL == "" {
		return fmt.Errorf("proxy URL cannot be empty")
	}
	
	// Check for supported schemes
	if !strings.HasPrefix(proxyURL, "http://") && 
	   !strings.HasPrefix(proxyURL, "https://") && 
	   !strings.HasPrefix(proxyURL, "socks5://") {
		return fmt.Errorf("unsupported proxy scheme, use http://, https://, or socks5://")
	}
	
	return nil
}

func init() {
	// Initialize configuration
	config = internal.DefaultConfig()
	
	// Add resume command
	rootCmd.AddCommand(resumeCmd)
	
	// Define CLI flags with environment variable fallbacks
	rootCmd.Flags().StringVarP(&outputPath, "output", "o", "", "Custom output file path")
	rootCmd.Flags().StringVarP(&cookiesPath, "cookies", "c", "", "Path to Netscape-format cookie file (env: TERAFETCH_COOKIES)")
	rootCmd.Flags().IntVarP(&threads, "threads", "t", config.DefaultThreads, fmt.Sprintf("Number of download threads (1-32) (env: TERAFETCH_THREADS) (default %d)", config.DefaultThreads))
	rootCmd.Flags().StringVarP(&rateLimit, "limit-rate", "r", "", "Bandwidth limit (e.g., 5M for 5MB/s) (env: TERAFETCH_RATE_LIMIT)")
	rootCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress bar output")
	rootCmd.Flags().StringVar(&proxyURL, "proxy", "", "HTTP/SOCKS proxy URL (env: TERAFETCH_PROXY)")
	rootCmd.Flags().BoolVar(&bypassAuth, "bypass", false, "Force bypass mode without authentication (env: TERAFETCH_BYPASS)")
	
	// Add flags to resume command as well
	resumeCmd.Flags().StringVarP(&cookiesPath, "cookies", "c", "", "Path to Netscape-format cookie file (env: TERAFETCH_COOKIES)")
	resumeCmd.Flags().IntVarP(&threads, "threads", "t", config.DefaultThreads, fmt.Sprintf("Number of download threads (1-32) (env: TERAFETCH_THREADS) (default %d)", config.DefaultThreads))
	resumeCmd.Flags().StringVarP(&rateLimit, "limit-rate", "r", "", "Bandwidth limit (e.g., 5M for 5MB/s) (env: TERAFETCH_RATE_LIMIT)")
	resumeCmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Suppress progress bar output")
	resumeCmd.Flags().StringVar(&proxyURL, "proxy", "", "HTTP/SOCKS proxy URL (env: TERAFETCH_PROXY)")
	
	// Logging flags
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "Enable debug logging with file and line information (env: TERAFETCH_DEBUG)")
	rootCmd.Flags().StringVar(&logLevel, "log-level", "", "Set log level (debug, info, warn, error) (env: TERAFETCH_LOG_LEVEL)")
	rootCmd.Flags().StringVar(&logFile, "log-file", "", "Write logs to file instead of stderr (env: TERAFETCH_LOG_FILE)")
	
	// Add usage examples
	rootCmd.SetUsageTemplate(`Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)
}

func Execute() error {
	return rootCmd.Execute()
}

// executeDownloadWorkflow implements the complete download workflow
func executeDownloadWorkflow(url, outputPath string, threads int, rateLimitBytes int64, cookiesPath, proxyURL string, quiet bool) error {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		internal.LogInfo("Received signal %v, initiating graceful shutdown...", sig)
		if !quiet {
			fmt.Printf("\n🛑 Received %v signal, shutting down gracefully...\n", sig)
		}
		cancel()
	}()

	// Initialize components
	resolver := downloader.NewTeraboxResolver()
	authManager := downloader.NewCookieAuthManager()
	engine := downloader.NewMultiThreadEngine()

	// Load authentication if cookies provided
	var authContext *internal.AuthContext
	if cookiesPath != "" {
		internal.LogInfo("Loading authentication from cookies file: %s", cookiesPath)
		var err error
		authContext, err = authManager.LoadCookies(cookiesPath)
		if err != nil {
			return fmt.Errorf("failed to load cookies: %w", err)
		}

		// Validate session
		if err := authManager.ValidateSession(authContext); err != nil {
			internal.LogWarn("Session validation failed: %v", err)
			if !quiet {
				fmt.Printf("⚠️  Warning: Session validation failed: %v\n", err)
				fmt.Printf("   Attempting to continue with potentially expired credentials...\n")
			}
		} else {
			internal.LogInfo("Authentication session validated successfully")
			if !quiet {
				fmt.Printf("✅ Authentication validated (expires: %s)\n", authContext.ExpiresAt.Format(time.RFC3339))
			}
		}
	}

	// Step 1: Resolve the URL to get file metadata
	internal.LogInfo("Resolving URL: %s", url)
	if !quiet {
		fmt.Printf("🔍 Resolving download link...\n")
	}

	var fileMetadata *internal.FileMetadata
	var err error

	// Check if bypass mode is forced
	if bypassAuth {
		internal.LogInfo("Bypass mode forced, skipping authentication")
		if !quiet {
			fmt.Printf("🔓 Bypass mode enabled - attempting without authentication...\n")
		}
		fileMetadata, err = resolver.ResolveWithBypass(url)
	} else if authContext != nil {
		// Try private link resolution first
		fileMetadata, err = resolver.ResolvePrivateLink(url, authContext)
		if err != nil {
			internal.LogWarn("Private link resolution failed: %v, trying public resolution", err)
			// Fallback to public resolution
			fileMetadata, err = resolver.ResolvePublicLink(url)
		}
	} else {
		// Public link resolution
		fileMetadata, err = resolver.ResolvePublicLink(url)
	}

	// If all standard methods failed and bypass wasn't forced, try bypass mode
	if err != nil && !bypassAuth {
		internal.LogWarn("Standard resolution failed: %v, attempting bypass mode", err)
		if !quiet {
			fmt.Printf("⚠️  Standard resolution failed, trying bypass mode...\n")
		}
		
		fileMetadata, err = resolver.ResolveWithBypass(url)
		if err != nil {
			internal.LogError("All resolution methods failed: %v", err)
			return fmt.Errorf("failed to resolve download URL: %w", err)
		}
		
		if !quiet {
			fmt.Printf("✅ Bypass mode successful!\n")
		}
	}

	if err != nil {
		internal.LogError("URL resolution failed: %v", err)
		return fmt.Errorf("failed to resolve download URL: %w", err)
	}

	internal.LogInfo("URL resolved successfully: filename=%s, size=%d bytes", fileMetadata.Filename, fileMetadata.Size)
	if !quiet {
		fmt.Printf("✅ Download link resolved\n")
		fmt.Printf("📄 File: %s\n", fileMetadata.Filename)
		fmt.Printf("📏 Size: %s\n", formatFileSize(fileMetadata.Size))
		if fileMetadata.Checksum != "" {
			fmt.Printf("🔐 Checksum: %s\n", fileMetadata.Checksum)
		}
		fmt.Println()
	}

	// Step 2: Create download configuration
	downloadConfig := &internal.DownloadConfig{
		OutputPath: outputPath,
		Threads:    threads,
		RateLimit:  rateLimitBytes,
		ProxyURL:   proxyURL,
		Quiet:      quiet,
	}

	// Step 3: Execute the download
	internal.LogInfo("Starting download with %d threads", threads)
	if !quiet {
		fmt.Printf("🚀 Starting download...\n")
	}

	// Monitor context for cancellation during download
	downloadErr := make(chan error, 1)
	go func() {
		downloadErr <- engine.Download(fileMetadata, downloadConfig)
	}()

	// Wait for download completion or cancellation
	select {
	case err := <-downloadErr:
		if err != nil {
			internal.LogError("Download failed: %v", err)
			return fmt.Errorf("download failed: %w", err)
		}
		
		internal.LogInfo("Download completed successfully: %s", outputPath)
		if !quiet {
			fmt.Printf("✅ Download completed successfully!\n")
			fmt.Printf("📁 File saved to: %s\n", outputPath)
		}
		return nil

	case <-ctx.Done():
		internal.LogInfo("Download cancelled by user")
		if !quiet {
			fmt.Printf("⏸️  Download cancelled. Resume data has been saved.\n")
			fmt.Printf("   Use 'terafetch resume %s.part' to continue later.\n", outputPath)
		}
		return fmt.Errorf("download cancelled by user")
	}
}

// executeResumeWorkflow implements the resume workflow
func executeResumeWorkflow(partialPath string, threads int, rateLimitBytes int64, cookiesPath, proxyURL string, quiet bool) error {
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		internal.LogInfo("Received signal %v during resume, initiating graceful shutdown...", sig)
		if !quiet {
			fmt.Printf("\n🛑 Received %v signal, shutting down gracefully...\n", sig)
		}
		cancel()
	}()

	// Initialize components
	engine := downloader.NewMultiThreadEngine()

	// Create download configuration
	downloadConfig := &internal.DownloadConfig{
		Threads:   threads,
		RateLimit: rateLimitBytes,
		ProxyURL:  proxyURL,
		Quiet:     quiet,
	}

	// Execute the resume
	internal.LogInfo("Resuming download from: %s", partialPath)
	if !quiet {
		fmt.Printf("🔄 Resuming download...\n")
	}

	// Monitor context for cancellation during resume
	resumeErr := make(chan error, 1)
	go func() {
		resumeErr <- engine.Resume(partialPath, downloadConfig)
	}()

	// Wait for resume completion or cancellation
	select {
	case err := <-resumeErr:
		if err != nil {
			internal.LogError("Resume failed: %v", err)
			return fmt.Errorf("resume failed: %w", err)
		}
		
		outputPath := strings.TrimSuffix(partialPath, ".part")
		internal.LogInfo("Resume completed successfully: %s", outputPath)
		if !quiet {
			fmt.Printf("✅ Resume completed successfully!\n")
			fmt.Printf("📁 File saved to: %s\n", outputPath)
		}
		return nil

	case <-ctx.Done():
		internal.LogInfo("Resume cancelled by user")
		if !quiet {
			fmt.Printf("⏸️  Resume cancelled. Resume data has been saved.\n")
			fmt.Printf("   Use 'terafetch resume %s' to continue later.\n", partialPath)
		}
		return fmt.Errorf("resume cancelled by user")
	}
}

// formatFileSize formats a file size in bytes to a human-readable string
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}