# ADR-001: Language and Technology Stack

**Status**: Accepted  
**Date**: 2026-01-10

---

## Context

Orc is an intelligent orchestrator for Claude Code that manages task execution, checkpointing, and provides a web UI for monitoring.

**CLI Requirements**:
- Fast startup time for responsive CLI interactions
- Single binary distribution for easy installation
- Strong concurrency primitives for managing multiple Claude processes
- Cross-platform support (Linux, macOS, Windows)

**UI Requirements**:
- Live transcript streaming with minimal latency
- Lightweight bundle size
- Modern reactive patterns for real-time updates

## Decision

**Backend**: Go with Cobra for CLI framework  
**Frontend**: Svelte 5 with SvelteKit  
**Communication**: REST API + Server-Sent Events (SSE) for live streaming

## Rationale

### Go for Backend

| Benefit | Description |
|---------|-------------|
| Single Binary | `go build` produces one executable with no runtime dependencies |
| Concurrency | Goroutines and channels map perfectly to managing multiple Claude processes |
| Fast Startup | Sub-millisecond startup vs seconds for JVM/interpreted languages |
| Cobra Ecosystem | Battle-tested CLI framework with completions, help generation |
| Cross-Compilation | Simple `GOOS=linux GOARCH=amd64 go build` |

### Svelte 5 for Frontend

| Benefit | Description |
|---------|-------------|
| Bundle Size | ~5KB runtime vs React's ~40KB+ |
| Reactivity | Native reactivity with runes ($state, $derived) perfect for live updates |
| SvelteKit | File-based routing, SSR/SSG options, built-in API routes |
| Simplicity | Less boilerplate for our simple UI needs (~10-15 components) |

## Consequences

**Positive**:
- Fast CLI, efficient UI updates for live transcripts
- Smaller codebase, fewer dependencies
- Single binary distribution

**Negative**:
- Smaller Svelte ecosystem than React
- Go verbosity in error handling

**Mitigation**: REST API means UI framework is swappable if needed.
