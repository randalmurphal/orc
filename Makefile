# =============================================================================
# Orc Makefile
# =============================================================================
#
# Quick Start:
#   make setup    # First-time setup (go mod, build deps)
#   make dev      # Start development shell in container
#   make build    # Build binary locally
#   make test     # Run tests
#
# Container Commands:
#   make docker-build   # Build all Docker images
#   make docker-test    # Run tests in container
#   make docker-shell   # Interactive shell in dev container
#
# =============================================================================

.PHONY: all setup build test lint clean dev docker-build docker-test docker-shell help

# Configuration
BINARY := orc
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DIR := bin
GO_FILES := $(shell find . -name '*.go' -not -path './vendor/*')

# Build flags
LDFLAGS := -ldflags="-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT)"

# Container runtime (supports docker, nerdctl, podman)
CONTAINER_RT ?= $(shell command -v nerdctl 2>/dev/null || command -v docker 2>/dev/null || echo "docker")
COMPOSE := $(CONTAINER_RT) compose

# Default target
all: build

# =============================================================================
# Setup
# =============================================================================

## setup: First-time project setup
setup: go.mod
	@echo "==> Setting up go.mod with local dependencies..."
	@if ! grep -q "replace.*llmkit" go.mod; then \
		echo "replace github.com/randymurphal/llmkit => ../llmkit" >> go.mod; \
	fi
	@if ! grep -q "replace.*flowgraph" go.mod; then \
		echo "replace github.com/randymurphal/flowgraph => ../flowgraph" >> go.mod; \
	fi
	@echo "==> Downloading dependencies..."
	go mod tidy
	@echo "==> Setup complete!"

## deps: Download dependencies
deps:
	go mod download
	go mod tidy

# =============================================================================
# Development (Native)
# =============================================================================

## build: Build the binary locally
build: $(BUILD_DIR)/$(BINARY)

$(BUILD_DIR)/$(BINARY): $(GO_FILES)
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $@ ./cmd/orc

## test: Run tests locally
test:
	go test -v -race -cover ./...

## test-short: Run tests without race detector (faster)
test-short:
	go test -v -cover ./...

## lint: Run linters locally
lint:
	golangci-lint run ./...

## vet: Run go vet
vet:
	go vet ./...

## fmt: Format code
fmt:
	go fmt ./...
	goimports -w .

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

## run: Build and run with arguments
run: build
	./$(BUILD_DIR)/$(BINARY) $(ARGS)

## install: Install binary to GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/orc

# =============================================================================
# Development (Container)
# =============================================================================

## docker-build: Build all Docker images
docker-build:
	$(COMPOSE) build

## docker-shell: Interactive development shell
docker-shell:
	$(COMPOSE) run --rm dev

## dev: Alias for docker-shell
dev: docker-shell

## docker-test: Run tests in container
docker-test:
	$(COMPOSE) run --rm test

## docker-lint: Run linter in container
docker-lint:
	$(COMPOSE) run --rm lint

## docker-clean: Remove containers and volumes
docker-clean:
	$(COMPOSE) down -v --remove-orphans
	$(CONTAINER_RT) image rm -f orc-dev orc-builder orc 2>/dev/null || true

# =============================================================================
# Release
# =============================================================================

## release-build: Build release binary in container
release-build:
	@mkdir -p $(BUILD_DIR)
	VERSION=$(VERSION) COMMIT=$(COMMIT) $(COMPOSE) run --rm build

## release-linux: Cross-compile for Linux
release-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-amd64 ./cmd/orc
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-linux-arm64 ./cmd/orc

## release-darwin: Cross-compile for macOS
release-darwin:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-amd64 ./cmd/orc
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY)-darwin-arm64 ./cmd/orc

# =============================================================================
# Frontend
# =============================================================================

## web-install: Install frontend dependencies
web-install:
	cd web && npm install

## web-dev: Start frontend dev server (proxies to :8080)
web-dev:
	cd web && npm run dev

## web-build: Build frontend for production
web-build:
	cd web && npm run build

## web-check: Type-check frontend
web-check:
	cd web && npm run check

## serve: Start API server (for frontend development)
serve: build
	./$(BUILD_DIR)/$(BINARY) serve

## dev-full: Start both API server and frontend dev server
dev-full:
	@echo "Starting API server on :8080 and frontend on :5173..."
	@echo "API: http://localhost:8080"
	@echo "UI:  http://localhost:5173"
	@$(MAKE) serve & cd web && npm run dev

# =============================================================================
# Coverage
# =============================================================================

## coverage: Generate test coverage report
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## coverage-func: Show coverage by function
coverage-func:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

# =============================================================================
# Help
# =============================================================================

## help: Show this help
help:
	@echo "Orc - Claude Code Task Orchestrator"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'
