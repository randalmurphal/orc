# Spec Package

Interactive specification sessions with Claude for feature planning.

## Overview

This package handles `orc spec` - spawning an interactive Claude session to collaboratively create specifications with the user. Unlike automated task execution, this is meant for user-in-the-loop spec creation.

## Key Types

| Type | Purpose |
|------|---------|
| `Options` | Spec session configuration |
| `Result` | Session outcome (SpecPath, TaskIDs) |
| `Spawner` | Claude process spawner |
| `PromptData` | Data for prompt generation |

## Workflow

1. Load detection from SQLite (if available)
2. Load initiative context (if specified)
3. Generate spec prompt from template
4. Spawn Claude interactively
5. Parse output for created spec/tasks

## Usage

```go
result, err := spec.Run(ctx, "User Authentication", spec.Options{
    WorkDir:      "/path/to/project",
    Model:        "claude-sonnet",
    InitiativeID: "INIT-001",  // Optional
    CreateTasks:  true,
    DryRun:       false,
})
```

## Prompt Template

The spec session prompt (`builtin/spec_session.md`) guides Claude through:

1. Understanding requirements
2. Researching the codebase
3. Clarifying details
4. Proposing an approach
5. Creating structured spec document

## Spawner

```go
type Spawner struct {
    opts SpawnerOptions
}

func (s *Spawner) RunInteractive(ctx context.Context, prompt string) error
// Spawns: claude --print -p <prompt> --dangerously-skip-permissions
// Inherits stdin/stdout/stderr for interactive use
```

## Integration with Initiatives

When linked to an initiative:
- Initiative vision included in prompt
- Prior decisions provided as context
- Spec saved in initiative directory
- Tasks can be linked automatically

## CLI Commands

| Command | Description |
|---------|-------------|
| `orc spec "title"` | Start spec session |
| `orc spec "title" --initiative INIT-001` | Link to initiative |
| `orc spec "title" --dry-run` | Show prompt only |
| `orc feature "title"` | Initiative + spec + tasks |

## Testing

```bash
go test ./internal/spec/... -v
```

Tests use mocked Claude spawner for isolation.
