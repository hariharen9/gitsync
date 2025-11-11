#!/bin/bash

# Build script for GitSync
# Builds binaries for multiple platforms

set -e

VERSION="1.0.0"
BINARY="gitsync"

echo "🔨 Building GitSync v${VERSION}"
echo ""

# Clean previous builds
rm -rf dist
mkdir -p dist

# Build for current platform
echo "Building for current platform..."
go build -ldflags "-s -w" -o "dist/${BINARY}" ./src
echo "✓ Built: dist/${BINARY}"

# Build for other platforms (optional)
if [ "$1" == "all" ]; then
    echo ""
    echo "Building for all platforms..."
    
    # macOS (Intel)
    GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o "dist/${BINARY}-darwin-amd64" ./src
    echo "✓ Built: dist/${BINARY}-darwin-amd64"
    
    # macOS (Apple Silicon)
    GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o "dist/${BINARY}-darwin-arm64" ./src
    echo "✓ Built: dist/${BINARY}-darwin-arm64"
    
    # Linux (amd64)
    GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o "dist/${BINARY}-linux-amd64" ./src
    echo "✓ Built: dist/${BINARY}-linux-amd64"
    
    # Linux (arm64)
    GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o "dist/${BINARY}-linux-arm64" ./src
    echo "✓ Built: dist/${BINARY}-linux-arm64"
    
    # Windows (amd64)
    GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o "dist/${BINARY}-windows-amd64.exe" ./src
    echo "✓ Built: dist/${BINARY}-windows-amd64.exe"
fi

echo ""
echo "✨ Build complete!"
echo ""
echo "To install locally:"
echo "  sudo mv dist/${BINARY} /usr/local/bin/"
echo ""
echo "To build for all platforms:"
echo "  ./build.sh all"
