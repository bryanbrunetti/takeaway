# Google Photos Takeout Cleanup Tool - Makefile

BINARY_NAME=takeaway-cleanup
MODULE_NAME=takeaway
VERSION?=1.0.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Build flags
LDFLAGS=-ldflags "-X main.version=$(VERSION)"
BUILD_DIR=bin

# Default target
.PHONY: all
all: clean build

# Build the application
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for all platforms
.PHONY: build-all
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)

	# Windows
	@echo "Building for Windows (amd64)..."
	@GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe .

	# macOS
	@echo "Building for macOS (amd64)..."
	@GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 .
	@echo "Building for macOS (arm64)..."
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 .

	# Linux
	@echo "Building for Linux (amd64)..."
	@GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 .
	@echo "Building for Linux (arm64)..."
	@GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 .

	@echo "Cross-compilation complete!"
	@ls -la $(BUILD_DIR)/

# Test the application
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Test with coverage
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GOFMT) ./...

# Vet code
.PHONY: vet
vet:
	@echo "Vetting code..."
	$(GOCMD) vet ./...

# Update dependencies
.PHONY: deps
deps:
	@echo "Updating dependencies..."
	$(GOMOD) tidy
	$(GOMOD) download

# Run the application with test data
.PHONY: run-test
run-test: build
	@echo "Running with test data..."
	./$(BUILD_DIR)/$(BINARY_NAME) -source ./test/src -output ./test/output -dry-run -move

# Run the application with test data (actual run)
.PHONY: run-test-real
run-test-real: build
	@echo "Running with test data (making real changes)..."
	@rm -rf ./test/output
	./$(BUILD_DIR)/$(BINARY_NAME) -source ./test/src -output ./test/output -move

# Install the binary to GOPATH/bin
.PHONY: install
install: build
	@echo "Installing $(BINARY_NAME) to GOPATH/bin..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(shell go env GOPATH)/bin/
	@echo "Installation complete!"

# Check prerequisites
.PHONY: check-prereqs
check-prereqs:
	@echo "Checking prerequisites..."
	@which exiftool > /dev/null || (echo "ERROR: exiftool not found in PATH" && exit 1)
	@echo "✓ exiftool found: $$(exiftool -ver)"
	@which go > /dev/null || (echo "ERROR: go not found in PATH" && exit 1)
	@echo "✓ go found: $$(go version)"

# Create release archives
.PHONY: release
release: build-all
	@echo "Creating release archives..."
	@mkdir -p releases

	# Windows
	@cd $(BUILD_DIR) && zip ../releases/$(BINARY_NAME)-$(VERSION)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe

	# macOS
	@cd $(BUILD_DIR) && tar -czf ../releases/$(BINARY_NAME)-$(VERSION)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	@cd $(BUILD_DIR) && tar -czf ../releases/$(BINARY_NAME)-$(VERSION)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64

	# Linux
	@cd $(BUILD_DIR) && tar -czf ../releases/$(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	@cd $(BUILD_DIR) && tar -czf ../releases/$(BINARY_NAME)-$(VERSION)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64

	@echo "Release archives created in releases/ directory"
	@ls -la releases/

# Development workflow
.PHONY: dev
dev: fmt vet test build

# CI workflow
.PHONY: ci
ci: check-prereqs deps fmt vet test build

# Help
.PHONY: help
help:
	@echo "Google Photos Takeout Cleanup Tool - Available targets:"
	@echo ""
	@echo "  build          Build the application for current platform"
	@echo "  build-all      Build for all supported platforms"
	@echo "  test           Run tests"
	@echo "  test-coverage  Run tests with coverage report"
	@echo "  clean          Remove build artifacts"
	@echo "  fmt            Format source code"
	@echo "  vet            Run go vet"
	@echo "  deps           Update dependencies"
	@echo "  run-test       Run with test data (dry run)"
	@echo "  run-test-real  Run with test data (actual changes)"
	@echo "  install        Install binary to GOPATH/bin"
	@echo "  check-prereqs  Check if prerequisites are installed"
	@echo "  release        Create release archives"
	@echo "  dev            Development workflow (fmt, vet, test, build)"
	@echo "  ci             CI workflow (includes prerequisite checks)"
	@echo "  help           Show this help message"
