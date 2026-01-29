# =============================================================================
# Orc Makefile
# =============================================================================
#
# Quick Start (new users):
#   curl -fsSL https://raw.githubusercontent.com/randalmurphal/orc/main/install.sh | sh
#
# Development:
#   make setup    # First-time setup (creates go.work for local deps)
#   make build    # Build binary locally
#   make test     # Run tests
#   make dev-full # Start API + frontend dev servers
#
# Container Commands:
#   make docker-build   # Build all Docker images
#   make docker-test    # Run tests in container
#   make docker-shell   # Interactive shell in dev container
#
# =============================================================================

.PHONY: all setup build test lint doc-lint clean dev docker-build docker-test docker-shell help

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

## setup: First-time project setup (for contributors with local deps)
setup:
	@echo "==> Creating go.work for local development..."
	@if [ ! -f go.work ]; then \
		echo "go 1.24.0\n\nuse .\nuse ../llmkit\nuse ../flowgraph" > go.work; \
		echo "Created go.work"; \
	else \
		echo "go.work already exists"; \
	fi
	@echo "==> Installing frontend dependencies..."
	cd web && bun install
	@echo "==> Setup complete!"
	@echo ""
	@echo "For development, run: make dev-full"

## deps: Download dependencies
deps:
	go mod download
	go mod tidy

# =============================================================================
# Development (Native)
# =============================================================================

## build: Build the binary locally (with embedded frontend)
build: web-build embed-frontend
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/orc

## build-cli: Build CLI only (no frontend)
build-cli: $(BUILD_DIR)/$(BINARY)

$(BUILD_DIR)/$(BINARY): $(GO_FILES)
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o $@ ./cmd/orc

## embed-frontend: Copy frontend build to embed directory
embed-frontend:
	@echo "==> Embedding frontend..."
	@rm -rf internal/api/static
	@cp -r web/build internal/api/static
	@touch internal/api/static/.gitkeep
	@echo "    Frontend embedded to internal/api/static/"

## test: Run tests locally
test:
	@mkdir -p internal/api/static
	@test -f internal/api/static/.gitkeep || touch internal/api/static/.gitkeep
	GOWORK=off go test -v -race -cover ./...

## test-short: Run tests without race detector (faster)
test-short:
	@mkdir -p internal/api/static
	@test -f internal/api/static/.gitkeep || touch internal/api/static/.gitkeep
	GOWORK=off go test -v -cover ./...

## lint: Run linters locally
lint:
	golangci-lint run ./...

## doc-lint: Check CLAUDE.md files against line thresholds
doc-lint:
	@./scripts/doc-lint.sh

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
# Frontend (React)
# =============================================================================

## web-install: Install frontend dependencies
web-install:
	cd web && bun install

## web-dev: Start frontend dev server (proxies to :8080)
web-dev:
	cd web && bun run dev

## web-build: Build frontend for production
web-build:
	cd web && bun run build

## web-test: Run frontend tests (vitest unit tests)
web-test:
	cd web && bun run test

## serve: Start API server (for frontend development)
serve: build
	./$(BUILD_DIR)/$(BINARY) serve

## dev-full: Start both API server and frontend dev server
dev-full:
	@echo "Starting API server on :8080 and frontend on :5173..."
	@echo "API: http://localhost:8080"
	@echo "UI:  http://localhost:5173"
	@$(MAKE) serve & cd web && bun run dev

# =============================================================================
# Coverage
# =============================================================================

## coverage: Generate test coverage report
coverage:
	@mkdir -p internal/api/static
	@test -f internal/api/static/.gitkeep || touch internal/api/static/.gitkeep
	GOWORK=off go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## coverage-func: Show coverage by function
coverage-func:
	@mkdir -p internal/api/static
	@test -f internal/api/static/.gitkeep || touch internal/api/static/.gitkeep
	GOWORK=off go test -coverprofile=coverage.out ./...
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
