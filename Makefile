# TeraFetch Makefile

.PHONY: build clean test lint install dev-deps cross-compile help

# Variables
BINARY_NAME=terafetch
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")
LDFLAGS=-s -w -X main.version=$(VERSION)
BUILD_DIR=dist

# Default target
all: build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) v$(VERSION) for current platform..."
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	rm -f $(BINARY_NAME)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint code
lint:
	@echo "Running linter..."
	golangci-lint run

# Install development dependencies
dev-deps:
	@echo "Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Cross-compile for all platforms
cross-compile:
	@echo "Cross-compiling for all platforms..."
	./build.sh

# Install binary to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install -ldflags="$(LDFLAGS)" .

# Development build with debug info
dev:
	@echo "Building development version..."
	go build -race -o $(BINARY_NAME) .

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	go mod tidy

# Verify dependencies
verify:
	@echo "Verifying dependencies..."
	go mod verify

# Run security scan
security:
	@echo "Running security scan..."
	gosec ./...

# Help
help:
	@echo "Available targets:"
	@echo "  build         - Build for current platform"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  lint          - Run linter"
	@echo "  dev-deps      - Install development dependencies"
	@echo "  cross-compile - Cross-compile for all platforms"
	@echo "  install       - Install binary to GOPATH/bin"
	@echo "  dev           - Development build with debug info"
	@echo "  fmt           - Format code"
	@echo "  tidy          - Tidy dependencies"
	@echo "  verify        - Verify dependencies"
	@echo "  security      - Run security scan"
	@echo "  help          - Show this help"