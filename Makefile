.PHONY: all build clean test run install deps build-all help

# Binary name
BINARY_NAME=tekmetric-mcp

# Version information
VERSION?=dev
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(BUILD_DATE)'"

# Directories
DIST_DIR=dist
BIN_DIR=bin

all: deps test build ## Run deps, test, and build

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

build: ## Build the binary for current platform
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/tekmetric-mcp
	@echo "Build complete: ./$(BINARY_NAME)"

build-all: clean ## Build binaries for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)

	@echo "Building for Linux AMD64..."
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/tekmetric-mcp

	@echo "Building for Linux ARM64..."
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/tekmetric-mcp

	@echo "Building for macOS AMD64..."
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/tekmetric-mcp

	@echo "Building for macOS ARM64 (Apple Silicon)..."
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/tekmetric-mcp

	@echo "Creating macOS universal binary..."
	@if command -v lipo >/dev/null 2>&1; then \
		lipo -create -output $(DIST_DIR)/$(BINARY_NAME)-darwin-universal \
			$(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 \
			$(DIST_DIR)/$(BINARY_NAME)-darwin-arm64; \
		echo "Universal binary created: $(DIST_DIR)/$(BINARY_NAME)-darwin-universal"; \
	else \
		echo "lipo not available, skipping universal binary"; \
	fi

	@echo "Building for Windows AMD64..."
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/tekmetric-mcp

	@echo "All builds complete! Binaries in $(DIST_DIR)/"
	@ls -lh $(DIST_DIR)

install: build ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME) to $(GOPATH)/bin..."
	go install $(LDFLAGS) ./cmd/tekmetric-mcp
	@echo "Installation complete!"

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests and show coverage
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out

run: build ## Build and run the server
	@echo "Starting server..."
	./$(BINARY_NAME) serve

run-debug: build ## Build and run with debug logging
	@echo "Starting server in debug mode..."
	./$(BINARY_NAME) -d serve

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -rf $(DIST_DIR)
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out
	@echo "Clean complete!"

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...

lint: ## Run linter
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with: brew install golangci-lint"; \
	fi

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)

version: ## Show version information
	@go run $(LDFLAGS) ./cmd/tekmetric-mcp version

# Development helpers
dev-setup: deps ## Set up development environment
	@echo "Setting up development environment..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		brew install golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@echo "Development environment ready!"

watch: ## Watch for changes and rebuild (requires entr)
	@echo "Watching for changes..."
	@if command -v entr >/dev/null 2>&1; then \
		find . -name '*.go' | entr -r make run; \
	else \
		echo "entr not installed. Install with: brew install entr"; \
	fi

# Release targets
release-dry: ## Dry run of release process
	@echo "Dry run release for version $(VERSION)..."
	@echo "Would create the following binaries:"
	@echo "  - $(DIST_DIR)/$(BINARY_NAME)-linux-amd64"
	@echo "  - $(DIST_DIR)/$(BINARY_NAME)-linux-arm64"
	@echo "  - $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64"
	@echo "  - $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64"
	@echo "  - $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe"

release: build-all ## Create a release (requires VERSION env var)
	@if [ "$(VERSION)" = "dev" ]; then \
		echo "ERROR: VERSION must be set for release. Example: make release VERSION=v1.0.0"; \
		exit 1; \
	fi
	@echo "Creating release $(VERSION)..."
	@mkdir -p $(DIST_DIR)
	cd $(DIST_DIR) && sha256sum * > checksums.txt
	@echo "Release $(VERSION) created in $(DIST_DIR)/"
	@echo "Files:"
	@ls -lh $(DIST_DIR)

# Desktop Extension targets
extension: ## Build Desktop Extension (.mcpb file)
	@command -v goreleaser >/dev/null 2>&1 || { \
		echo "ERROR: goreleaser is required to build the macOS universal binary."; \
		echo "Install it from https://goreleaser.com, then run make extension again."; \
		exit 1; \
	}
	goreleaser build --snapshot --clean
	@$(MAKE) package-extension

package-extension: ## Package existing binaries into .mcpb (for CI after GoReleaser)
	@echo "Packaging Desktop Extension..."
	@mkdir -p $(DIST_DIR)/extension
	@cp manifest.json $(DIST_DIR)/extension/
	@# The archive carries only the binaries manifest.json names. The macOS
	@# universal binary already holds both architectures, so shipping the two
	@# single architecture builds beside it would repeat 26MB.
	@if [ -d "$(DIST_DIR)/tekmetric-mcp-universal_darwin_all" ]; then \
		echo "Using GoReleaser output..."; \
		cp $(DIST_DIR)/tekmetric-mcp-universal_darwin_all/tekmetric-mcp $(DIST_DIR)/extension/tekmetric-mcp-darwin-universal; \
		cp $(DIST_DIR)/tekmetric-mcp_linux_amd64_v1/tekmetric-mcp $(DIST_DIR)/extension/tekmetric-mcp-linux-amd64; \
		cp $(DIST_DIR)/tekmetric-mcp_windows_amd64_v1/tekmetric-mcp.exe $(DIST_DIR)/extension/tekmetric-mcp-windows-amd64.exe; \
	else \
		echo "Using flat dist/ output..."; \
		cp $(DIST_DIR)/$(BINARY_NAME)-darwin-universal $(DIST_DIR)/extension/ 2>/dev/null || true; \
		cp $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(DIST_DIR)/extension/ 2>/dev/null || true; \
		cp $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(DIST_DIR)/extension/ 2>/dev/null || true; \
	fi
	@if [ -f icon.png ]; then cp icon.png $(DIST_DIR)/extension/; fi
	@echo "Checking the binaries manifest.json names..."
	@missing=""; \
	for binary in $$(grep -o '$${__dirname}/[A-Za-z0-9._-]*' manifest.json | sed 's|$${__dirname}/||' | sort -u); do \
		if [ ! -f "$(DIST_DIR)/extension/$$binary" ]; then \
			missing="$$missing $$binary"; \
		fi; \
	done; \
	if [ -n "$$missing" ]; then \
		echo ""; \
		echo "ERROR: manifest.json names binaries the build did not produce:"; \
		for binary in $$missing; do echo "  - $$binary"; done; \
		echo ""; \
		echo "Claude Desktop runs these by name. A missing one leaves an"; \
		echo "extension that cannot start on that platform."; \
		echo ""; \
		echo "The macOS universal binary comes from GoReleaser. Build it with:"; \
		echo "  goreleaser build --snapshot --clean && make package-extension"; \
		echo ""; \
		rm -rf $(DIST_DIR)/extension; \
		exit 1; \
	fi
	@cd $(DIST_DIR)/extension && zip -r -X ../tekmetric-mcp.mcpb .
	@rm -rf $(DIST_DIR)/extension
	@echo "Desktop Extension created: $(DIST_DIR)/tekmetric-mcp.mcpb"
	@ls -lh $(DIST_DIR)/tekmetric-mcp.mcpb

extension-test: ## Check a built .mcpb: binaries, settings, and that it runs
	@python3 scripts/test-extension.py $(DIST_DIR)/tekmetric-mcp.mcpb
