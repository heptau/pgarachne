# --- Configuration ---
BINARY_NAME=pgarachne
CMD_PATH=./cmd/pgarachne
BIN_DIR=./bin

# --- Commands ---
GO=go
GO_BUILD=$(GO) build
GO_TIDY=$(GO) mod tidy
GO_RUN=$(GO) run
LDFLAGS=-ldflags="-s -w"

# Dynamic OS and Arch detection for build
GOOS := $(shell $(GO) env GOOS)
GOARCH := $(shell $(GO) env GOARCH)

# Targets that are not file names
.PHONY: help deps build build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-darwin-amd64 build-darwin-arm64 build-all run clean

# ------------------------------------------------------------------------------
# Default target remains 'help'
default: help

help:
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Available targets:"
	@echo "  help                  Shows this help message."
	@echo "  build                 Builds the binary for the current operating system ($(GOOS)/$(GOARCH))."
	@echo "                        Output: $(BIN_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)"
	@echo ""
	@echo "  --- Linux ---"
	@echo "  build-linux-amd64     Builds for Linux (x64/Intel/AMD)."
	@echo "  build-linux-arm64     Builds for Linux (ARM64/Aarch64 - e.g. Raspberry Pi, Graviton)."
	@echo ""
	@echo "  --- Windows ---"
	@echo "  build-windows-amd64   Builds for Windows (x64/Intel/AMD)."
	@echo "  build-windows-arm64   Builds for Windows (ARM64 - e.g. Surface Pro X, Snapdragon)."
	@echo ""
	@echo "  --- macOS ---"
	@echo "  build-darwin-amd64    Builds for macOS (x64/Intel)."
	@echo "  build-darwin-arm64    Builds for macOS (ARM64/Apple Silicon)."
	@echo ""
	@echo "  --- General ---"
	@echo "  build-all             Builds for ALL target systems listed above."
	@echo "  run                   Runs the application using 'go run' (no binary created)."
	@echo "  clean                 Removes all build artifacts from the '$(BIN_DIR)' directory."
	@echo "  deps                  Manually runs the dependency check and download."
	@echo ""

# ------------------------------------------------------------------------------
# Target to ensure dependencies
deps:
	@echo "==> Ensuring dependencies are up to date..."
	@$(GO_TIDY)

# ------------------------------------------------------------------------------
# Build targets

# Build for current OS (for local development or specific build)
build: deps
	@echo "==> Building for current system ($(GOOS)/$(GOARCH))..."
	@mkdir -p $(BIN_DIR)
	@$(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH) $(CMD_PATH)
	@echo "==> Build complete: $(BIN_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)"

# --- Linux Builds ---
build-linux-amd64: deps
	@echo "==> Building for Linux (amd64)..."
	@mkdir -p $(BIN_DIR)
	@GOOS=linux GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_PATH)

build-linux-arm64: deps
	@echo "==> Building for Linux (arm64)..."
	@mkdir -p $(BIN_DIR)
	@GOOS=linux GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_PATH)

# --- Windows Builds ---
build-windows-amd64: deps
	@echo "==> Building for Windows (amd64)..."
	@mkdir -p $(BIN_DIR)
	@GOOS=windows GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_PATH)

build-windows-arm64: deps
	@echo "==> Building for Windows (arm64)..."
	@mkdir -p $(BIN_DIR)
	@GOOS=windows GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-windows-arm64.exe $(CMD_PATH)

# --- macOS Builds ---
build-darwin-amd64: deps
	@echo "==> Building for macOS (amd64/Intel)..."
	@mkdir -p $(BIN_DIR)
	@GOOS=darwin GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_PATH)

build-darwin-arm64: deps
	@echo "==> Building for macOS (arm64/Apple Silicon)..."
	@mkdir -p $(BIN_DIR)
	@GOOS=darwin GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_PATH)

# --- Batch Build ---
# Builds everything defined above
build-all: build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-arm64 build-darwin-amd64 build-darwin-arm64
	@echo "==> All cross-compilation builds finished."

# Build and run the application
run: deps
	@echo "==> Running application (go run)..."
	@$(GO_RUN) $(CMD_PATH)/main.go

# Cleanup
clean:
	@echo "==> Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)
