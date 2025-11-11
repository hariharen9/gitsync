.PHONY: build install clean run

# Binary name
BINARY=gitsync

# Build the binary
build:
	@echo "Building $(BINARY)..."
	@go build -o $(BINARY) .
	@echo "✓ Built $(BINARY)"

# Install to /usr/local/bin
install: build
	@echo "Installing $(BINARY) to /usr/local/bin..."
	@sudo mv $(BINARY) /usr/local/bin/
	@echo "✓ Installed! Run 'gitsync' from anywhere."

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY)
	@echo "✓ Cleaned"

# Run the tool
run: build
	@./$(BINARY)

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@echo "✓ Dependencies ready"
