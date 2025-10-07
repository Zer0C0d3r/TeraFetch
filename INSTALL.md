# Installation Guide

This guide provides detailed installation instructions for TeraFetch across different platforms and installation methods.

## Quick Install

### Linux (x86_64)
```bash
curl -L https://github.com/Zer0C0d3r/terafetch/releases/latest/download/terafetch-linux-amd64.tar.gz | tar xz
sudo mv terafetch /usr/local/bin/
```

### macOS (Intel)
```bash
curl -L https://github.com/Zer0C0d3r/terafetch/releases/latest/download/terafetch-darwin-amd64.tar.gz | tar xz
sudo mv terafetch /usr/local/bin/
```

### macOS (Apple Silicon)
```bash
curl -L https://github.com/Zer0C0d3r/terafetch/releases/latest/download/terafetch-darwin-arm64.tar.gz | tar xz
sudo mv terafetch /usr/local/bin/
```

### Windows
1. Download `terafetch-windows-amd64.zip` from [releases](https://github.com/Zer0C0d3r/terafetch/releases)
2. Extract to a folder (e.g., `C:\Program Files\TeraFetch\`)
3. Add the folder to your PATH environment variable

## Installation Methods

### Method 1: Pre-built Binaries (Recommended)

#### Step 1: Download
Visit the [releases page](https://github.com/Zer0C0d3r/terafetch/releases) and download the appropriate binary for your system:

- **Linux x86_64**: `terafetch-linux-amd64.tar.gz`
- **Linux ARM64**: `terafetch-linux-arm64.tar.gz`
- **macOS Intel**: `terafetch-darwin-amd64.tar.gz`
- **macOS Apple Silicon**: `terafetch-darwin-arm64.tar.gz`
- **Windows x86_64**: `terafetch-windows-amd64.zip`
- **Windows ARM64**: `terafetch-windows-arm64.zip`
- **FreeBSD x86_64**: `terafetch-freebsd-amd64.tar.gz`

#### Step 2: Extract and Install

**Linux/macOS/FreeBSD:**
```bash
# Extract the archive
tar -xzf terafetch-*.tar.gz

# Make executable (if needed)
chmod +x terafetch

# Install system-wide (optional)
sudo mv terafetch /usr/local/bin/

# Or install to user directory
mkdir -p ~/.local/bin
mv terafetch ~/.local/bin/
# Add ~/.local/bin to PATH if not already done
```

**Windows:**
```cmd
# Extract the ZIP file
# Move terafetch.exe to desired location
# Add location to PATH environment variable
```

#### Step 3: Verify Installation
```bash
terafetch --version
```

### Method 2: Build from Source

#### Prerequisites
- Go 1.25 or later
- Git
- Make (optional, for using Makefile)

#### Step 1: Install Go
**Linux (Arch Linux):**
```bash
sudo pacman -S go
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt update
sudo apt install golang-go
# Or install latest from official source
wget https://go.dev/dl/go1.25.1.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.1.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

**macOS:**
```bash
# Using Homebrew
brew install go

# Or download from https://golang.org/dl/
```

**Windows:**
Download and install from [https://golang.org/dl/](https://golang.org/dl/)

#### Step 2: Clone and Build
```bash
# Clone the repository
git clone https://github.com/Zer0C0d3r/terafetch.git
cd terafetch

# Install dependencies
go mod tidy

# Build using Makefile (recommended)
make build

# Or build directly with go
go build -o terafetch .

# Install to GOPATH/bin
make install
# Or: go install .
```

#### Step 3: Cross-compile (Optional)
```bash
# Build for all supported platforms
make cross-compile

# Binaries will be in dist/ directory
```



## System Requirements

### Minimum Requirements

| Component | Requirement |
|-----------|-------------|
| **Operating System** | Linux (kernel 6.0+), macOS (14.0+), Windows (10+), FreeBSD (14+) |
| **Architecture** | x86_64 (amd64), ARM64 |
| **RAM** | 64MB available memory |
| **Storage** | 10MB for binary + download space |
| **Network** | Internet connection |

### Recommended Requirements

| Component | Recommendation |
|-----------|----------------|
| **RAM** | 256MB+ (for large files with many threads) |
| **Storage** | SSD for better I/O performance |
| **Network** | Stable broadband (10+ Mbps) |
| **CPU** | Multi-core for concurrent downloads |

### Platform-Specific Notes

#### Linux
- **Kernel**: 6.0 or later (tested on 6.16.10-zen1-1-zen)
- **Distributions**: Arch Linux, Ubuntu 24.04+, Fedora 40+, Debian 12+
- **glibc**: 2.38 or later (for pre-built binaries)
- **Dependencies**: None (statically linked)

#### macOS
- **Version**: 14.0 (Sonoma) or later
- **Architecture**: Intel x86_64 or Apple Silicon (ARM64)
- **Permissions**: May require allowing in Security & Privacy settings

#### Windows
- **Version**: Windows 11 (22H2) or later
- **Architecture**: x86_64 or ARM64
- **Runtime**: No additional runtime required

#### FreeBSD
- **Version**: 14.0 or later
- **Architecture**: x86_64
- **Compatibility**: Community supported

## Post-Installation Setup

### 1. Verify Installation
```bash
terafetch --version
terafetch --help
```

### 2. Set Up PATH (if needed)
**Linux/macOS (.bashrc, .zshrc, etc.):**
```bash
export PATH="$PATH:$HOME/.local/bin"
```

**Windows (PowerShell profile):**
```powershell
$env:PATH += ";C:\Path\To\TeraFetch"
```

### 3. Create Configuration Directory (Optional)
```bash
# Linux/macOS
mkdir -p ~/.config/terafetch

# Windows
mkdir %APPDATA%\terafetch
```

### 4. Test Basic Functionality
```bash
# Test with a public Terabox link
terafetch --help
```

## Troubleshooting Installation

### Common Issues

#### 1. "Command not found"
**Problem**: Binary not in PATH
**Solution**: 
- Add installation directory to PATH
- Use full path to binary
- Reinstall to standard location

#### 2. "Permission denied"
**Problem**: Binary not executable
**Solution**:
```bash
chmod +x terafetch
```

#### 3. "Cannot execute binary file"
**Problem**: Wrong architecture
**Solution**: Download correct binary for your system architecture

#### 4. macOS "Cannot be opened" Error
**Problem**: Gatekeeper blocking unsigned binary
**Solution**:
```bash
# Remove quarantine attribute
xattr -d com.apple.quarantine terafetch

# Or allow in System Preferences > Security & Privacy
```

#### 5. Windows SmartScreen Warning
**Problem**: Windows blocking unsigned executable
**Solution**: Click "More info" → "Run anyway" or build from source

### Getting Help

If you encounter issues:

1. Check the [troubleshooting section](README.md#troubleshooting) in README
2. Search [existing issues](https://github.com/Zer0C0d3r/terafetch/issues)
3. Create a [new issue](https://github.com/Zer0C0d3r/terafetch/issues/new) with:
   - Operating system and version
   - Architecture (x86_64, ARM64, etc.)
   - Installation method used
   - Error messages or logs

## Uninstallation

### Remove Binary
```bash
# If installed to /usr/local/bin
sudo rm /usr/local/bin/terafetch

# If installed to ~/.local/bin
rm ~/.local/bin/terafetch
```

### Remove Configuration (Optional)
```bash
# Linux/macOS
rm -rf ~/.config/terafetch
rm -rf ~/.local/share/terafetch

# Windows
rmdir /s %APPDATA%\terafetch
```

## Updating

### Manual Update
1. Download latest release
2. Replace existing binary
3. Verify new version: `terafetch --version`

### Automated Update (Future)
```bash
# Self-update command (planned feature)
terafetch update
```