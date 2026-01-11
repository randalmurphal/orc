# Setup Package

Claude-powered interactive project setup.

## Overview

This package handles the `orc setup` command - spawning an interactive Claude session to configure the project. Unlike `orc init` (instant bootstrap), this is meant for thorough project understanding and configuration.

## Key Types

| Type | Purpose |
|------|---------|
| `Options` | Setup configuration (WorkDir, Model, DryRun, etc.) |
| `Result` | Setup outcome (Validated bool) |
| `Spawner` | Claude process spawner |
| `Validator` | Output validation |

## Workflow

1. Load detection from SQLite (from `orc init`)
2. Generate setup prompt from detection + template
3. Spawn Claude interactively (or show prompt in dry-run)
4. Validate output (CLAUDE.md changes, config files)

## Usage

```go
result, err := setup.Run(ctx, setup.Options{
    WorkDir:        "/path/to/project",
    Model:          "claude-sonnet",
    DryRun:         false,
    SkipValidation: false,
})
```

## Prompt Generation

The setup prompt adapts to project complexity:

| Project Size | Approach |
|--------------|----------|
| Small | Quick scan, minimal CLAUDE.md additions |
| Medium | Sample files, document patterns |
| Large/Monorepo | Ask focus areas, incremental setup |

Template location: `builtin/setup.yaml` (embedded)

## Spawner

```go
type Spawner struct {
    opts SpawnerOptions
}

func (s *Spawner) RunInteractive(ctx context.Context, prompt string) error
// Spawns: claude --print -p <prompt> --dangerously-skip-permissions
// Inherits stdin/stdout/stderr for interactive use
```

## Validator

```go
type Validator struct {
    workDir string
}

func (v *Validator) Validate() error
// Checks:
// - CLAUDE.md exists and has orc section
// - config.yaml is valid YAML
// - Optional: skills created correctly
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `--dry-run` | Show prompt without running Claude |
| `--model` | Claude model to use |
| `--skip-validation` | Skip output validation |

## Testing

```bash
go test ./internal/setup/... -v
```

Tests use mocked Claude spawner for isolation.
