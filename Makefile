# Makefile for Key-Value API Server

# Variables
APP_NAME = kvapi
CLIENT_NAME = kvclient
GO = go
GOBUILD = $(GO) build
GOCLEAN = $(GO) clean
GOTEST = $(GO) test
GOGET = $(GO) get
BINARY_NAME = $(APP_NAME)
CLIENT_BINARY_NAME = $(CLIENT_NAME)
BINARY_UNIX = $(BINARY_NAME)_unix
CLIENT_BINARY_UNIX = $(CLIENT_NAME)_unix
# Use UNIX epoch time in hexadecimal format for consistent versioning
TIMESTAMP = $(shell printf '%x' $$(date +%s))
VERSION ?= 0.1.$(TIMESTAMP)
BUILD_TIME = $(shell date +%FT%T%z)
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Add -s -w flags to strip debugging symbols and reduce binary size
LD_FLAGS = -s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'

# Check if UPX is available
UPX_COMMAND = $(shell command -v upx 2> /dev/null)
ifdef UPX_COMMAND
	UPX_ENABLED = true
else
	UPX_ENABLED = false
endif

# Default listen address and port
LISTEN_ADDR ?= :8080
ALLOWED_CIDR ?=
SIMULATE_FIREWALL ?= false

# Default target
.PHONY: all
all: test build

# Build the server application
.PHONY: build
build:
	@echo "Building $(APP_NAME) version $(VERSION) ($(GIT_COMMIT))..."
	$(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(BINARY_NAME) -v
ifeq ($(UPX_ENABLED), true)
	@echo "Compressing $(BINARY_NAME) with UPX..."
	$(UPX_COMMAND) --fast $(BINARY_NAME)
else
	@echo "UPX not found. Skipping compression for $(BINARY_NAME)."
endif

# Build for Linux
.PHONY: build-linux
build-linux:
	@echo "Building $(APP_NAME) for Linux version $(VERSION) ($(GIT_COMMIT))..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(BINARY_UNIX) -v
ifeq ($(UPX_ENABLED), true)
	@echo "Compressing $(BINARY_UNIX) with UPX..."
	$(UPX_COMMAND) --fast $(BINARY_UNIX)
else
	@echo "UPX not found. Skipping compression for $(BINARY_UNIX)."
endif

# Build the client application
.PHONY: build-client
build-client:
	@echo "Building $(CLIENT_NAME) version $(VERSION) ($(GIT_COMMIT))..."
	$(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(CLIENT_BINARY_NAME) -v ./cmd/kvclient
ifeq ($(UPX_ENABLED), true)
	@echo "Compressing $(CLIENT_BINARY_NAME) with UPX..."
	$(UPX_COMMAND) --fast $(CLIENT_BINARY_NAME)
else
	@echo "UPX not found. Skipping compression for $(CLIENT_BINARY_NAME)."
endif

# Build the client for Linux
.PHONY: build-client-linux
build-client-linux:
	@echo "Building $(CLIENT_NAME) for Linux version $(VERSION) ($(GIT_COMMIT))..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags="$(LD_FLAGS)" -o $(CLIENT_BINARY_UNIX) -v ./cmd/kvclient
ifeq ($(UPX_ENABLED), true)
	@echo "Compressing $(CLIENT_BINARY_UNIX) with UPX..."
	$(UPX_COMMAND) --fast $(CLIENT_BINARY_UNIX)
else
	@echo "UPX not found. Skipping compression for $(CLIENT_BINARY_UNIX)."
endif

# Build both server and client
.PHONY: build-all
build-all: build build-client

# Build both server and client for Linux
.PHONY: build-all-linux
build-all-linux: build-linux build-client-linux

# Clean build files
.PHONY: clean
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f $(CLIENT_BINARY_NAME)
	rm -f $(CLIENT_BINARY_UNIX)

# Run the application
.PHONY: run
run:
	@echo "Running $(APP_NAME) version $(VERSION) on $(LISTEN_ADDR)..."
	@if [ -n "$(ALLOWED_CIDR)" ]; then \
		echo "Restricting access to CIDR: $(ALLOWED_CIDR)"; \
		if [ "$(SIMULATE_FIREWALL)" = "true" ]; then \
			echo "Firewall simulation enabled: Silently dropping requests from non-allowed IPs"; \
			./$(BINARY_NAME) --listen $(LISTEN_ADDR) --allowed-cidr $(ALLOWED_CIDR) --simulate-firewall $(ARGS); \
		else \
			./$(BINARY_NAME) --listen $(LISTEN_ADDR) --allowed-cidr $(ALLOWED_CIDR) $(ARGS); \
		fi; \
	else \
		./$(BINARY_NAME) --listen $(LISTEN_ADDR) $(ARGS); \
	fi

# Run with firewall simulation (local network only)
.PHONY: run-secure
run-secure:
	@echo "Running $(APP_NAME) with local network restriction..."
	$(MAKE) ALLOWED_CIDR=192.168.0.0/16 run

# Run with firewall simulation (localhost only)
.PHONY: run-local
run-local:
	@echo "Running $(APP_NAME) with localhost restriction..."
	$(MAKE) ALLOWED_CIDR=127.0.0.1/32 run

# Run with drop firewall simulation (localhost only)
.PHONY: run-fw-drop
run-fw-drop:
	@echo "Running $(APP_NAME) with DROP firewall simulation (localhost only)..."
	$(MAKE) ALLOWED_CIDR=127.0.0.1/32 ARGS="--fw-drop" run

# Run with reject firewall simulation (localhost only)
.PHONY: run-fw-reject
run-fw-reject:
	@echo "Running $(APP_NAME) with REJECT firewall simulation (localhost only)..."
	$(MAKE) ALLOWED_CIDR=127.0.0.1/32 ARGS="--fw-reject" run

# For backward compatibility (deprecated, will be removed in future release)
.PHONY: run-firewall
run-firewall:
	@echo "Running $(APP_NAME) with firewall simulation (localhost only) - DEPRECATED, use run-fw-drop instead..."
	$(MAKE) ALLOWED_CIDR=127.0.0.1/32 ARGS="--simulate-firewall" run

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GOGET) -v ./...

# Build and install
.PHONY: install
install: build
	@echo "Installing $(APP_NAME) version $(VERSION)..."
	mv $(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

# Build and install client
.PHONY: install-client
install-client: build-client
	@echo "Installing $(CLIENT_NAME) version $(VERSION)..."
	mv $(CLIENT_BINARY_NAME) $(GOPATH)/bin/$(CLIENT_BINARY_NAME)

# Build and install both server and client
.PHONY: install-all
install-all: install install-client

# Change version
.PHONY: version
version:
	@echo "Current version: $(VERSION)"
	@echo "Note: The version number is now automatically generated as 0.1.HEX"
	@echo "      where HEX is the UNIX epoch time in hexadecimal format."
	@echo "To use a specific version number, set VERSION environment variable:"
	@echo "  make VERSION=custom.version build-all"

# Display help information
.PHONY: help
help:
	@echo "Key-Value API Server - Make Targets"
	@echo ""
	@echo "Current version: $(VERSION) (auto-generated as 0.1.HEX)"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  all             Build and run tests (default)"
	@echo "  build           Build the server application"
	@echo "  build-linux     Build the server for Linux"
	@echo "  build-client    Build the client application"
	@echo "  build-client-linux Build the client for Linux"
	@echo "  build-all       Build both server and client"
	@echo "  build-all-linux Build both server and client for Linux"
	@echo "  clean           Remove build artifacts"
	@echo "  run             Run the server (LISTEN_ADDR=:8080 by default)"
	@echo "  run-secure      Run with local network restriction (192.168.0.0/16)"
	@echo "  run-local       Run with localhost restriction (127.0.0.1/32)"
	@echo "  run-firewall    Run with firewall simulation (localhost only)"
	@echo "  test            Run tests"
	@echo "  deps            Install dependencies"
	@echo "  install         Build and install the server"
	@echo "  install-client  Build and install the client"
	@echo "  install-all     Install both server and client"
	@echo "  version         Display version information"
	@echo "  help            Display this help message"
	@echo ""
	@echo "Examples:"
	@echo "  make build-all              # Build both server and client"
	@echo "  make run                    # Run server with default settings"
	@echo "  make LISTEN_ADDR=:3000 run  # Run server on port 3000"
	@echo "  make ALLOWED_CIDR=10.0.0.0/8 run # Restrict to private IP range"
	@echo "  make build-client          # Build just the client application"
	@echo "  make VERSION=0.2.custom version # Use a custom version number" 