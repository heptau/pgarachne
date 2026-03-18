# --- START OF FILE Makefile ---

# --- Configuration ---
# 1. Ensure make can find Go (Homebrew / standard paths)
export PATH := /opt/homebrew/bin:/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin:$(PATH)

BINARY_NAME=pgarachne
APP_NAME=PgArachne
CMD_PATH=./cmd/pgarachne
BIN_DIR=./bin
DIST_DIR=./dist

# Path to app icon (must exist: ./assets/pgarachne.icns)
ICON_PATH=./assets/pgarachne.icns

# Name of the actual binary inside .app (hidden, launched by the script)
EXEC_BINARY=$(BINARY_NAME)-exec

# --- Versioning ---
# Read version from the VERSION file. If it does not exist, use "0.0.0-dev"
VERSION_FILE=VERSION
APP_VERSION := $(shell cat $(VERSION_FILE) 2>/dev/null || echo "0.0.0-dev")

# --- External Config ---
-include config.mk

# --- Commands ---
GO := $(shell command -v go 2> /dev/null || echo go)
GORELEASER := $(shell command -v goreleaser 2> /dev/null || echo goreleaser)
GO_BUILD=$(GO) build
GO_TIDY=$(GO) mod tidy
GO_RUN=$(GO) run

# LDFLAGS:
# -s -w : reduce binary size (strip debug symbols)
# -X ... : inject version from Makefile into Go variable "main.Version"
LDFLAGS=-ldflags "-s -w -X 'main.Version=$(APP_VERSION)'"

# OS detection for local builds
GOOS := $(shell $(GO) env GOOS)
GOARCH := $(shell $(GO) env GOARCH)

.PHONY: help deps build run clean prepare-dist docs tests \
        release macos-apps \
        macos-app-amd64 macos-app-arm64 macos-app-universal

# ------------------------------------------------------------------------------
default: help

help:
	@echo ""
	@echo "PgArachne Build System (Version: $(APP_VERSION))"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Dev targets:"
	@echo "  build                 Build binary for current OS."
	@echo "  tests                 Run all tests (unit + integration) via Docker."
	@echo "  clean                 Remove artifacts."
	@echo "  docs                  Build documentation with Hugo."
	@echo "  release               Build local release artifacts + Homebrew files from VERSION."
	@echo ""

deps:
	@echo "==> Checking dependencies..."
	@$(GO_TIDY)

release:
	@echo "==> Building local release artifacts"
	@APP_VERSION=$(APP_VERSION) $(GORELEASER) release --snapshot --clean --skip=publish
	@$(MAKE) macos-apps
	@./scripts/generate_homebrew_formula.sh
	@./scripts/generate_homebrew_cask.sh

macos-apps:
	@$(MAKE) macos-app-amd64
	@$(MAKE) macos-app-arm64
	@$(MAKE) macos-app-universal

docs:
	@echo "==> Building documentation (Hugo)"
	@cd docs-src && hugo --cleanDestinationDir --minify
	@cp docs-src/static/index.html docs/index.html
	@echo "==> Customizing root 404.html for GitHub Pages"
	@if [ -f docs/en/404.html ]; then \
		sed 's|href=../|href=|g; s|src=../|src=|g; s|/404\.html||g' docs/en/404.html > docs/404.html; \
		rm -f docs/*/404.html; \
	fi
	@echo "==> Fixing typography (TypoLima)"
	@command -v typolima >/dev/null 2>&1 || { echo "==> Skipping typography fixes (install 'typolima' via brew)"; }
	@if command -v typolima >/dev/null 2>&1; then \
		typolima docs/cs/ --lang cs --recursive --in-place; \
		typolima docs/en/ --lang en --recursive --in-place; \
		typolima docs/de/ --lang de --recursive --in-place; \
		typolima docs/fr/ --lang fr --recursive --in-place; \
		typolima docs/es/ --lang es --recursive --in-place; \
		typolima docs/it/ --lang it --recursive --in-place; \
		typolima docs/pt/ --lang pt --recursive --in-place; \
	fi
	@command -v minify >/dev/null 2>&1 || { echo "==> Skipping static asset minification (install 'minify' via brew)"; exit 0; }
	@echo "==> Minifying static assets"
	@find docs/assets -type f -name '*.css' -print0 | xargs -0 -I{} minify -o {} {}
	@find docs/assets -type f -name '*.js' -print0 | xargs -0 -I{} minify -o {} {}
	@minify -o docs/index.html docs/index.html
	@minify -o docs/404.html docs/404.html
	@minify --type application/xml -o docs/sitemap.xml docs/sitemap.xml
	@minify --type application/json -o docs/manifest.json docs/manifest.json

prepare-dist:
	@mkdir -p $(BIN_DIR)
	@mkdir -p $(DIST_DIR)

# ------------------------------------------------------------------------------
# Helpers (Functions)
# ------------------------------------------------------------------------------

# 1. Generate Info.plist with VERSION
define generate_plist
	echo '<?xml version="1.0" encoding="UTF-8"?>' > $(1)/Info.plist; \
	echo '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' >> $(1)/Info.plist; \
	echo '<plist version="1.0">' >> $(1)/Info.plist; \
	echo '<dict>' >> $(1)/Info.plist; \
	echo '    <key>CFBundleExecutable</key>' >> $(1)/Info.plist; \
	echo '    <string>$(BINARY_NAME)</string>' >> $(1)/Info.plist; \
	echo '    <key>CFBundleIconFile</key>' >> $(1)/Info.plist; \
	echo '    <string>AppIcon</string>' >> $(1)/Info.plist; \
	echo '    <key>CFBundleIdentifier</key>' >> $(1)/Info.plist; \
	echo '    <string>com.example.$(BINARY_NAME)</string>' >> $(1)/Info.plist; \
	echo '    <key>CFBundleName</key>' >> $(1)/Info.plist; \
	echo '    <string>$(APP_NAME)</string>' >> $(1)/Info.plist; \
	echo '    <key>CFBundlePackageType</key>' >> $(1)/Info.plist; \
	echo '    <string>APPL</string>' >> $(1)/Info.plist; \
	echo '    <key>CFBundleShortVersionString</key>' >> $(1)/Info.plist; \
	echo '    <string>$(APP_VERSION)</string>' >> $(1)/Info.plist; \
	echo '    <key>CFBundleVersion</key>' >> $(1)/Info.plist; \
	echo '    <string>$(APP_VERSION)</string>' >> $(1)/Info.plist; \
	echo '    <key>LSUIElement</key>' >> $(1)/Info.plist; \
	echo '    <false/>' >> $(1)/Info.plist; \
	echo '</dict>' >> $(1)/Info.plist; \
	echo '</plist>' >> $(1)/Info.plist
endef

# 2. Create Launcher Script (Bash)
define create_launcher
	echo '#!/bin/bash' > $(1)/$(BINARY_NAME)
	echo 'DIR="$$(cd "$$(dirname "$$0")" && pwd)"' >> $(1)/$(BINARY_NAME)
	echo 'open -a Terminal "$$DIR/$(EXEC_BINARY)"' >> $(1)/$(BINARY_NAME)
	chmod +x $(1)/$(BINARY_NAME)
endef

# 3. Copy Icon
define copy_icon
	mkdir -p $(1)/Contents/Resources
	@if [ -f "$(ICON_PATH)" ]; then \
		echo "    ... copying icon from $(ICON_PATH)"; \
		cp "$(ICON_PATH)" "$(1)/Contents/Resources/AppIcon.icns"; \
	else \
		echo "    !!! WARNING: Icon not found at $(ICON_PATH). App will use default icon."; \
	fi
endef

# 4. Sign App
define sign_app
	if [ -z "$(SIGNING_IDENTITY)" ]; then \
		echo "==> Applying AD-HOC signature (-)..."; \
		codesign --force --deep --sign "-" $(1); \
	else \
		echo "==> Signing with identity: $(SIGNING_IDENTITY)"; \
		codesign --force --deep --sign "$(SIGNING_IDENTITY)" $(1); \
	fi; \
	echo "==> Verifying signature..."; \
	codesign -dv $(1)
endef

# ------------------------------------------------------------------------------
# macOS App Bundles (.app) with Icon & Launcher
# ------------------------------------------------------------------------------

# 1. macOS AMD64 (Intel) .app
macos-app-amd64: prepare-dist
	@echo "==> Building macOS .app Intel (v$(APP_VERSION))..."
	$(eval TMP_DIR=$(DIST_DIR)/tmp-amd64)
	$(eval APP_DIR=$(TMP_DIR)/$(APP_NAME).app)
	$(eval MACOS=$(APP_DIR)/Contents/MacOS)

	@# Clean temp & Prepare
	@rm -rf $(TMP_DIR)
	@mkdir -p $(MACOS)

	@# Build
	@GOOS=darwin GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(MACOS)/$(EXEC_BINARY) $(CMD_PATH)
	@$(call create_launcher,$(MACOS))
	@$(call copy_icon,$(APP_DIR))
	@$(call generate_plist,$(APP_DIR)/Contents)
	@$(call sign_app,$(APP_DIR))

	@# Zip from inside TMP to maintain "PgArachne.app" root folder
	@cd $(TMP_DIR) && zip -rq ../$(BINARY_NAME)-macos-amd64-app.zip $(APP_NAME).app

	@# Cleanup
	@rm -rf $(TMP_DIR)
	@echo "Ready: $(DIST_DIR)/$(BINARY_NAME)-macos-amd64-app.zip"

# 2. macOS ARM64 (Apple Silicon) .app
macos-app-arm64: prepare-dist
	@echo "==> Building macOS .app Silicon (v$(APP_VERSION))..."
	$(eval TMP_DIR=$(DIST_DIR)/tmp-arm64)
	$(eval APP_DIR=$(TMP_DIR)/$(APP_NAME).app)
	$(eval MACOS=$(APP_DIR)/Contents/MacOS)

	@rm -rf $(TMP_DIR)
	@mkdir -p $(MACOS)

	@GOOS=darwin GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(MACOS)/$(EXEC_BINARY) $(CMD_PATH)
	@$(call create_launcher,$(MACOS))
	@$(call copy_icon,$(APP_DIR))
	@$(call generate_plist,$(APP_DIR)/Contents)
	@$(call sign_app,$(APP_DIR))

	@cd $(TMP_DIR) && zip -rq ../$(BINARY_NAME)-macos-arm64-app.zip $(APP_NAME).app

	@rm -rf $(TMP_DIR)
	@echo "Ready: $(DIST_DIR)/$(BINARY_NAME)-macos-arm64-app.zip"

# 3. macOS Universal .app
macos-app-universal: prepare-dist
	@echo "==> Building macOS .app Universal (v$(APP_VERSION))..."
	$(eval TMP_DIR=$(DIST_DIR)/tmp-universal)
	$(eval APP_DIR=$(TMP_DIR)/$(APP_NAME).app)
	$(eval MACOS=$(APP_DIR)/Contents/MacOS)

	@rm -rf $(TMP_DIR)
	@mkdir -p $(MACOS)

	@echo "    ... building partial binaries"
	@GOOS=darwin GOARCH=amd64 $(GO_BUILD) $(LDFLAGS) -o $(MACOS)/$(EXEC_BINARY)-amd64 $(CMD_PATH)
	@GOOS=darwin GOARCH=arm64 $(GO_BUILD) $(LDFLAGS) -o $(MACOS)/$(EXEC_BINARY)-arm64 $(CMD_PATH)
	@echo "    ... merging with lipo"
	@lipo -create -output $(MACOS)/$(EXEC_BINARY) $(MACOS)/$(EXEC_BINARY)-amd64 $(MACOS)/$(EXEC_BINARY)-arm64
	@rm $(MACOS)/$(EXEC_BINARY)-amd64 $(MACOS)/$(EXEC_BINARY)-arm64

	@$(call create_launcher,$(MACOS))
	@$(call copy_icon,$(APP_DIR))
	@$(call generate_plist,$(APP_DIR)/Contents)
	@$(call sign_app,$(APP_DIR))

	@cd $(TMP_DIR) && zip -rq ../$(BINARY_NAME)-macos-universal-app.zip $(APP_NAME).app

	@rm -rf $(TMP_DIR)
	@echo "Ready: $(DIST_DIR)/$(BINARY_NAME)-macos-universal-app.zip"

# Spins up a Postgres container, applies schema + mcp_functions, runs
# go test ./..., then tears the container down.
# Requires: Docker Desktop (or Docker Engine) and Docker Compose v2.
tests:
	@echo "==> Running tests (Docker + Go)"
	@bash scripts/run_tests.sh

build: deps
	@echo "==> Building locally (v$(APP_VERSION))..."
	@mkdir -p $(BIN_DIR)
	@$(GO_BUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) $(CMD_PATH)

run: deps
	@$(GO_RUN) $(CMD_PATH)/main.go

clean:
	@echo "==> Cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -rf $(DIST_DIR)
