#!/bin/bash

# GitSync Installation Script
# Builds and installs gitsync to /usr/local/bin

set -e

BINARY="gitsync"
INSTALL_PATH="/usr/local/bin"

echo "🌿 GitSync Installation"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+ first."
    echo "   Visit: https://golang.org/doc/install"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✓ Found Go version: $GO_VERSION"

# Download dependencies
echo ""
echo "📦 Downloading dependencies..."
go mod download
echo "✓ Dependencies ready"

# Build binary
echo ""
echo "🔨 Building $BINARY..."
go build -ldflags "-s -w" -o "$BINARY" ./src
echo "✓ Built successfully"

# Check if we need sudo
if [ -w "$INSTALL_PATH" ]; then
    SUDO=""
else
    SUDO="sudo"
    echo ""
    echo "🔐 Need sudo permission to install to $INSTALL_PATH"
fi

# Install
echo ""
echo "📥 Installing to $INSTALL_PATH/$BINARY..."
$SUDO mv "$BINARY" "$INSTALL_PATH/"
$SUDO chmod +x "$INSTALL_PATH/$BINARY"
echo "✓ Installed successfully"

# Verify installation
echo ""
if command -v gitsync &> /dev/null; then
    echo "✨ Installation complete!"
    echo ""
    echo "Run 'gitsync' from any git repository to get started."
    echo "Run 'gitsync -m' for manual mode with confirmations."
    echo ""
    echo "📚 Documentation: See README.md for full details."
else
    echo "⚠️  Installation completed but 'gitsync' not found in PATH"
    echo "   You may need to add $INSTALL_PATH to your PATH"
fi
