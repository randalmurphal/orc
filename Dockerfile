# syntax=docker/dockerfile:1

# =============================================================================
# Orc - Claude Code Task Orchestrator
# =============================================================================
# Multi-stage build for development and production use.
#
# Usage:
#   Development:  docker compose up -d dev
#   Build only:   docker build --target builder -t orc-builder .
#   Production:   docker build --target runtime -t orc .
# =============================================================================

# -----------------------------------------------------------------------------
# Base: Common dependencies
# -----------------------------------------------------------------------------
FROM golang:1.23-alpine AS base

RUN apk add --no-cache \
    git \
    make \
    bash \
    curl \
    openssh-client

WORKDIR /app

# -----------------------------------------------------------------------------
# Dependencies: Download and cache Go modules
# -----------------------------------------------------------------------------
FROM base AS deps

# Copy go.mod/sum first for better layer caching
COPY go.mod go.sum ./

# Local module replacements are handled via volume mounts in development
# For CI/production, modules should be published or vendored
RUN go mod download

# -----------------------------------------------------------------------------
# Builder: Compile the binary
# -----------------------------------------------------------------------------
FROM deps AS builder

COPY . .

# Build with version info
ARG VERSION=dev
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" \
    -o /bin/orc \
    ./cmd/orc

# -----------------------------------------------------------------------------
# Development: Full development environment
# -----------------------------------------------------------------------------
FROM base AS dev

# Install additional dev tools
RUN apk add --no-cache \
    nodejs \
    npm \
    vim \
    jq \
    yq

# Install Go tools
RUN go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest && \
    go install golang.org/x/tools/gopls@latest && \
    go install github.com/go-delve/delve/cmd/dlv@latest

# Create non-root user for development
RUN adduser -D -u 1000 dev
USER dev

WORKDIR /app

# Default command for development
CMD ["bash"]

# -----------------------------------------------------------------------------
# Runtime: Minimal production image
# -----------------------------------------------------------------------------
FROM alpine:3.20 AS runtime

RUN apk add --no-cache \
    git \
    bash \
    ca-certificates

# Create non-root user
RUN adduser -D -u 1000 orc
USER orc

WORKDIR /home/orc

# Copy binary from builder
COPY --from=builder /bin/orc /usr/local/bin/orc

# Copy templates (needed at runtime)
COPY --chown=orc:orc templates/ /home/orc/.orc-templates/

ENTRYPOINT ["orc"]
CMD ["--help"]
