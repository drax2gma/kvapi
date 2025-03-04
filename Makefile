# Makefile for Key-Value API Server

# Application names
APP_NAME = kvapi
CLIENT_NAME = kvclient

# Go commands
GO = go
GOBUILD = $(GO) build
GOCLEAN = $(GO) clean
GOTEST = $(GO) test

# Build variables
BUILD_ID_FILE := .build_id
$(shell mkdir -p $(dir $(BUILD_ID_FILE)))
$(shell test -f $(BUILD_ID_FILE) || printf '%x' $$(date +%s) > $(BUILD_ID_FILE))
BUILD_ID := $(shell cat $(BUILD_ID_FILE))

# Version information
VERSION ?= 0.1.$(BUILD_ID)
BUILD_TIME = $(shell date +%FT%T%z)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LD_FLAGS = -s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'

# Check if UPX is available
UPX_COMMAND = $(shell command -v upx 2> /dev/null)
ifdef UPX_COMMAND
	UPX_ENABLED = true
else
	UPX_ENABLED = false
endif

# Default target
.PHONY: all
all: test build-all

# Clean
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(APP_NAME)* $(CLIENT_NAME)* $(BUILD_ID_FILE)

# Function to build both server and client for a platform
define build_platform
	@echo "Building $(APP_NAME) for $(1) $(2) $(3) version $(VERSION) ($(GIT_COMMIT))..."
	CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) $(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(APP_NAME)-$(1)-$(2)$(4) -v
	@echo "Building $(CLIENT_NAME) for $(1) $(2) $(3) version $(VERSION) ($(GIT_COMMIT))..."
	CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) $(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(CLIENT_NAME)-$(1)-$(2)$(4) -v ./cmd/kvclient
	@if [ "$(UPX_ENABLED)" = "true" ]; then \
		echo "Compressing binaries with UPX..."; \
		$(UPX_COMMAND) --fast $(APP_NAME)-$(1)-$(2)$(4) $(CLIENT_NAME)-$(1)-$(2)$(4); \
	else \
		echo "UPX not found. Skipping compression."; \
	fi
	@touch .last_build_success
endef

# Platform-specific build targets
.PHONY: build-linux
build-linux:
	$(call build_platform,linux,amd64,64-bit,)
	@rm -f $(BUILD_ID_FILE)

.PHONY: build-windows
build-windows:
	$(call build_platform,windows,amd64,64-bit,.exe)
	@rm -f $(BUILD_ID_FILE)

.PHONY: build-osx
build-osx:
	$(call build_platform,darwin,amd64,64-bit,)
	@rm -f $(BUILD_ID_FILE)

.PHONY: build-arm
build-arm:
	$(call build_platform,linux,arm64,ARM 64-bit,)
	@rm -f $(BUILD_ID_FILE)

# Build for current platform
.PHONY: build
build:
	@echo "Building $(APP_NAME) for current platform version $(VERSION) ($(GIT_COMMIT))..."
	$(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(APP_NAME) -v
	@echo "Building $(CLIENT_NAME) for current platform version $(VERSION) ($(GIT_COMMIT))..."
	$(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(CLIENT_NAME) -v ./cmd/kvclient
	@if [ "$(UPX_ENABLED)" = "true" ]; then \
		echo "Compressing binaries with UPX..."; \
		$(UPX_COMMAND) --fast $(APP_NAME) $(CLIENT_NAME); \
	else \
		echo "UPX not found. Skipping compression."; \
	fi
	@rm -f $(BUILD_ID_FILE)

# Build all platforms
.PHONY: build-all
build-all: build-linux build-windows build-osx build-arm
	@echo "All platforms built successfully"

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Display help information
.PHONY: help
help:
	@echo "Key-Value API Server - Make Targets"
	@echo ""
	@echo "Current version: $(VERSION) (build ID: $(BUILD_ID))"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all             Run tests and build all platforms (default)"
	@echo "  build           Build for current platform"
	@echo "  build-linux     Build for Linux (amd64)"
	@echo "  build-windows   Build for Windows (amd64)"
	@echo "  build-osx       Build for macOS (amd64)"
	@echo "  build-arm       Build for ARM (arm64)"
	@echo "  build-all       Build for all platforms"
	@echo "  clean           Remove all generated files"
	@echo "  test            Run tests"
	@echo "  help            Display this help message"
	@echo ""
	@echo "Note: All build commands create both server and client binaries" 