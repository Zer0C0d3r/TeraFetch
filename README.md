# TeraFetch

[![CI](https://github.com/Zer0C0d3r/terafetch/workflows/CI/badge.svg)](https://github.com/Zer0C0d3r/terafetch/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Zer0C0d3r/terafetch/workflows/CodeQL/badge.svg)](https://github.com/Zer0C0d3r/terafetch/actions/workflows/codeql.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Zer0C0d3r/terafetch)](https://goreportcard.com/report/github.com/Zer0C0d3r/terafetch)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Release](https://img.shields.io/github/release/Zer0C0d3r/terafetch.svg)](https://github.com/Zer0C0d3r/terafetch/releases/latest)

A production-grade CLI tool for downloading files from Terabox with advanced features including multi-threaded downloads, authentication bypass, and comprehensive domain support.

## 🚀 Features

### Core Functionality
- **Multi-threaded Downloads**: Configurable concurrent segments (1-32 threads)
- **Resume Support**: Automatic resume of interrupted downloads with integrity verification
- **Authentication Bypass**: Advanced bypass techniques for accessing public content
- **Rate Limiting**: Intelligent bandwidth control and throttling
- **Progress Tracking**: Real-time progress bars with ETA and statistics

### Advanced Capabilities
- **Comprehensive Domain Support**: 20+ Terabox subdomains including regional variants
- **Multiple Resolution Methods**: API calls, web scraping, and alternative endpoints
- **Robust Error Handling**: Automatic retry with exponential backoff and jitter
- **Cross-Platform**: Native binaries for Linux, macOS, Windows, and FreeBSD
- **Proxy Support**: HTTP/SOCKS proxy compatibility with authentication

### Security & Reliability
- **Secure Cookie Handling**: Safe authentication with automatic cleanup
- **URL Validation**: Comprehensive validation for supported domains
- **Memory Efficient**: Streaming downloads with optimized memory usage
- **Thread Safety**: Concurrent operations with proper synchronization

## Installation

### Pre-built Binaries

Download the latest release for your platform from the [releases page](https://github.com/Zer0C0d3r/terafetch/releases).

#### Linux/macOS
```bash
# Download and extract
curl -L https://github.com/Zer0C0d3r/terafetch/releases/latest/download/terafetch-linux-amd64.tar.gz | tar xz
sudo mv terafetch /usr/local/bin/
```

#### Windows
Download `terafetch-windows-amd64.zip` and extract to a directory in your PATH.

### Build from Source

#### Prerequisites
- Go 1.20 or later
- Git

#### Build Steps
```bash
git clone https://github.com/Zer0C0d3r/terafetch.git
cd terafetch
make build
# or
go build -o terafetch .
```

## Usage

### Basic Usage

```bash
# Download a public Terabox file
terafetch https://terabox.com/s/1AbC123DefG456

# Download with custom output path
terafetch -o /path/to/download/ https://terabox.com/s/1AbC123DefG456

# Download with 16 threads
terafetch -t 16 https://terabox.com/s/1AbC123DefG456
```

### Advanced Usage

```bash
# Download with authentication bypass (recommended for public files)
terafetch --bypass https://terabox.com/s/1AbC123DefG456

# Download private files with authentication
terafetch -c cookies.txt https://terabox.com/s/1AbC123DefG456

# Download with rate limiting (5MB/s)
terafetch -r 5M https://terabox.com/s/1AbC123DefG456

# Download through proxy
terafetch --proxy http://proxy.example.com:8080 https://terabox.com/s/1AbC123DefG456

# High-performance download with 16 threads
terafetch -t 16 --bypass https://terabox.com/s/1AbC123DefG456

# Quiet mode (no progress bar)
terafetch -q https://terabox.com/s/1AbC123DefG456

# Combine multiple options
terafetch --bypass -t 16 -r 10M -o ./downloads/ https://terabox.com/s/1AbC123DefG456

# Force bypass mode via environment variable
TERAFETCH_BYPASS=true terafetch https://terabox.com/s/1AbC123DefG456
```

### Supported Domains

TeraFetch supports all major Terabox domains and subdomains:

```bash
# Primary domains
terafetch https://terabox.com/s/1AbC123DefG456
terafetch https://www.terabox.com/s/1AbC123DefG456
terafetch https://terabox.app/s/1AbC123DefG456

# Regional and specialized subdomains
terafetch https://us.terabox.com/s/1AbC123DefG456
terafetch https://jp.terabox.com/s/1AbC123DefG456
terafetch https://ca.terabox.com/s/1AbC123DefG456

# Alternative domains
terafetch https://1024terabox.com/s/1AbC123DefG456
terafetch https://dm.terabox.com/s/1AbC123DefG456
terafetch https://v1.terabox.com/s/1AbC123DefG456

# And 15+ more subdomains supported...
```

### Command Line Options

```
Usage:
  terafetch [OPTIONS] <URL>

Core Options:
  -o, --output string      Output directory or file path
  -t, --threads int        Number of download threads (1-32) (default 8)
  -r, --limit-rate string  Limit download rate (e.g., 5M, 1G)
  -q, --quiet             Suppress progress output

Authentication & Bypass:
  -c, --cookies string     Path to Netscape-format cookie file
      --bypass            Force bypass mode without authentication

Network & Proxy:
      --proxy string      HTTP/SOCKS proxy URL

Logging & Debug:
  -d, --debug             Enable debug logging with file and line information
      --log-level string  Set log level (debug, info, warn, error)
      --log-file string   Write logs to file instead of stderr

Information:
  -h, --help              Show help information
  -v, --version           Show version information

Environment Variables:
  TERAFETCH_THREADS       Default number of threads (1-32)
  TERAFETCH_TIMEOUT       HTTP timeout in seconds
  TERAFETCH_COOKIES       Path to cookie file
  TERAFETCH_PROXY         Proxy URL
  TERAFETCH_RATE_LIMIT    Default rate limit (e.g., 5M)
  TERAFETCH_BYPASS        Enable bypass mode (true/false)
  TERAFETCH_DEBUG         Enable debug logging (true/false)
```

## Authentication

For downloading private files, you need to provide authentication cookies:

### Obtaining Cookies

1. **Browser Extension Method** (Recommended):
   - Install a cookie export extension (e.g., "Get cookies.txt")
   - Login to Terabox in your browser
   - Export cookies in Netscape format
   - Save as `cookies.txt`

2. **Manual Method**:
   - Login to Terabox in your browser
   - Open Developer Tools (F12)
   - Go to Application/Storage → Cookies → terabox.com
   - Copy BDUSS and STOKEN values
   - Create a Netscape-format cookie file

### Cookie File Format

```
# Netscape HTTP Cookie File
.terabox.com	TRUE	/	FALSE	1234567890	BDUSS	your_bduss_value_here
.terabox.com	TRUE	/	FALSE	1234567890	STOKEN	your_stoken_value_here
```

### Environment Variables

You can also set cookies via environment variable:
```bash
export TERABOX_COOKIES="/path/to/cookies.txt"
terafetch https://terabox.com/s/1AbC123DefG456
```
## Configuration

### Rate Limiting

TeraFetch supports human-readable rate limiting formats:

- `5M` or `5MB` - 5 megabytes per second
- `1G` or `1GB` - 1 gigabyte per second
- `500K` or `500KB` - 500 kilobytes per second
- `1000` - 1000 bytes per second

### Proxy Support

Supported proxy formats:
- HTTP: `http://proxy.example.com:8080`
- HTTPS: `https://proxy.example.com:8080`
- SOCKS5: `socks5://proxy.example.com:1080`
- With authentication: `http://user:pass@proxy.example.com:8080`

## Resume Functionality

TeraFetch automatically resumes interrupted downloads:

1. **Automatic Detection**: Detects existing `.part` files
2. **Metadata Persistence**: Stores download progress in JSON format
3. **Segment Recovery**: Resumes from the last completed segment
4. **Integrity Verification**: Validates file size after completion

### Manual Resume

If automatic resume fails, you can manually resume:
```bash
# Resume a specific .part file
terafetch --resume /path/to/file.part https://terabox.com/s/1AbC123DefG456
```

## Troubleshooting

### Common Issues

#### 1. "Invalid URL" Error
**Problem**: URL format not recognized
**Solution**: 
- Ensure URL is from `terabox.com` or `pan.baidu.com`
- Check URL format: `https://terabox.com/s/1AbC123DefG456`

#### 2. "Authentication Required" Error
**Problem**: Private file requires login
**Solution**:
- Obtain cookies from your browser
- Use `-c cookies.txt` flag
- Verify BDUSS and STOKEN are present and valid

#### 3. "Rate Limited" Error
**Problem**: Too many requests to Terabox
**Solution**:
- Wait a few minutes before retrying
- Use fewer threads (`-t 4` instead of `-t 16`)
- Add delays between requests

#### 4. "Download Failed" Error
**Problem**: Network or server issues
**Solution**:
- Check internet connection
- Try using a proxy (`--proxy`)
- Reduce thread count
- Retry the download (automatic resume will work)

#### 5. Slow Download Speeds
**Problem**: Downloads slower than expected
**Solution**:
- Increase thread count (`-t 16`)
- Check if rate limiting is applied (`-r`)
- Try different proxy servers
- Verify network bandwidth##
# Debug Mode

Enable debug logging for troubleshooting:
```bash
# Set debug environment variable
export TERAFETCH_DEBUG=1
terafetch https://terabox.com/s/1AbC123DefG456

# Or use verbose flag (if implemented)
terafetch --verbose https://terabox.com/s/1AbC123DefG456
```

### Log Files

TeraFetch creates log files in:
- Linux/macOS: `~/.local/share/terafetch/logs/`
- Windows: `%APPDATA%\terafetch\logs\`

### Network Issues

If experiencing network problems:

1. **Test connectivity**:
   ```bash
   curl -I https://terabox.com
   ```

2. **Check DNS resolution**:
   ```bash
   nslookup terabox.com
   ```

3. **Test with proxy**:
   ```bash
   terafetch --proxy http://proxy.example.com:8080 <URL>
   ```

## System Requirements

### Minimum Requirements
- **OS**: Linux (kernel 3.2+), macOS (10.12+), Windows (7+), FreeBSD (11+)
- **RAM**: 64MB available memory
- **Storage**: 10MB for binary + download space
- **Network**: Internet connection

### Recommended Requirements
- **RAM**: 256MB+ for large files with many threads
- **Storage**: SSD for better I/O performance
- **Network**: Stable broadband connection (10+ Mbps)

### Supported Architectures
- **x86_64** (amd64) - Primary support
- **ARM64** (aarch64) - Full support
- **ARMv7** - Limited testing

## Performance Tips

### Optimal Thread Count
- **Small files** (< 100MB): 1-4 threads
- **Medium files** (100MB - 1GB): 4-8 threads
- **Large files** (> 1GB): 8-16 threads
- **Very large files** (> 10GB): 16-32 threads

### Memory Usage
- Base usage: ~10-20MB
- Per thread: ~1-2MB additional
- Large files: Consider available RAM

### Network Optimization
- Use wired connection when possible
- Close other bandwidth-intensive applications
- Consider time of day (Terabox server load)

## Legal and Compliance

**Important**: This tool is for personal use only. Users must:

- Respect Terabox's Terms of Service
- Comply with copyright laws
- Only download content you have permission to access
- Use responsibly and ethically

The developers are not responsible for misuse of this tool.#
# Development

### Building from Source

```bash
# Clone repository
git clone https://github.com/Zer0C0d3r/terafetch.git
cd terafetch

# Install dependencies
go mod tidy

# Build for current platform
make build

# Cross-compile for all platforms
make cross-compile

# Run tests
make test

# Run with coverage
make test-coverage
```

### Project Structure

```
terafetch/
├── main.go                    # CLI entry point
├── cmd/                       # Cobra CLI commands
├── downloader/                # Core download logic
│   ├── resolver.go           # URL resolution
│   ├── engine.go             # Download engine
│   ├── auth.go               # Authentication
│   └── planner.go            # Download planning
├── utils/                     # Utility functions
│   ├── http.go               # HTTP client
│   ├── fs.go                 # File operations
│   ├── progress.go           # Progress tracking
│   └── ratelimit.go          # Rate limiting
└── internal/                  # Internal packages
    ├── config.go             # Configuration
    ├── errors.go             # Error types
    └── types.go              # Data structures
```

### Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for version history and changes.

## Support

- **Issues**: [GitHub Issues](https://github.com/Zer0C0d3r/terafetch/issues)
- **Discussions**: [GitHub Discussions](https://github.com/Zer0C0d3r/terafetch/discussions)
- **Documentation**: This README and inline code documentation

## Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [pb](https://github.com/cheggaaa/pb) - Progress bars

---

**Disclaimer**: This tool is not affiliated with or endorsed by Terabox. Use at your own risk and in compliance with applicable laws and terms of service.