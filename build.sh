#!/bin/bash

# TeraFetch Build Script
# Builds cross-platform binaries for major operating systems and architectures

set -e

VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "v1.0.0")}
LDFLAGS="-s -w -X main.version=${VERSION}"
OUTPUT_DIR="dist"

echo "Building TeraFetch v${VERSION}..."

# Clean previous builds
rm -rf ${OUTPUT_DIR}
mkdir -p ${OUTPUT_DIR}

# Build targets: OS/ARCH
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
    "freebsd/amd64"
)

for target in "${TARGETS[@]}"; do
    IFS='/' read -r GOOS GOARCH <<< "$target"
    
    output_name="terafetch"
    if [ "$GOOS" = "windows" ]; then
        output_name="terafetch.exe"
    fi
    
    output_path="${OUTPUT_DIR}/terafetch-${GOOS}-${GOARCH}"
    if [ "$GOOS" = "windows" ]; then
        output_path="${output_path}.exe"
    fi
    
    echo "Building for ${GOOS}/${GOARCH}..."
    
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="${LDFLAGS}" \
        -o "${output_path}" \
        .
    
    # Create compressed archives
    if [ "$GOOS" = "windows" ]; then
        zip -j "${OUTPUT_DIR}/terafetch-${GOOS}-${GOARCH}.zip" "${output_path}"
    else
        tar -czf "${OUTPUT_DIR}/terafetch-${GOOS}-${GOARCH}.tar.gz" -C "${OUTPUT_DIR}" "$(basename "${output_path}")"
    fi
done

# Generate checksums
cd ${OUTPUT_DIR}
sha256sum * > checksums.txt
cd ..

echo "Build complete! Binaries available in ${OUTPUT_DIR}/"
echo "Checksums generated in ${OUTPUT_DIR}/checksums.txt"