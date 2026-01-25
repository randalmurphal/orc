# Plan Session Package

Interactive planning sessions with Claude Code for tasks and features.

## Overview

This package handles `orc plan` - spawning an interactive Claude Code session to collaboratively create specifications with the user. Unlike automated task execution, this is meant for user-in-the-loop spec creation.

## Modes

| Mode | Trigger | Purpose |
|------|---------|---------|
| `ModeTask` | `orc plan TASK-001` | Refine existing task with spec |
| `ModeFeature` | `orc plan "title"` | Create feature spec, optionally generate tasks |
| `ModeInteractive` | `orc plan` (no args) | Prompt user for target |

## Key Types

| Type | Purpose |
|------|---------|
| `Mode` | Planning mode (task/feature/interactive) |
| `Options` | Session configuration |
| `Result` | Session outcome (SpecPath, TaskIDs, ValidationResult) |
| `PromptData` | Data for prompt generation |
| `Spawner` | Claude process spawner |

Note: `Result.ValidationResult` uses `task.SpecValidation` from the task package.

## Workflow

### Task Mode

```
1. Load existing task
2. Load initiative context (if linked)
3. Generate prompt with task context
4. Spawn Claude Code interactively
5. User collaborates to create spec
6. Claude saves spec to .orc/tasks/TASK-ID/spec.md
7. Validate spec against requirements
```

### Feature Mode

```
1. Generate prompt for feature spec
2. Spawn Claude Code interactively
3. User collaborates to create spec
4. Save spec to .orc/specs/<name>.md
5. If --create-tasks: parse spec and generate tasks
```

## Usage

```go
result, err := plan_session.Run(ctx, "TASK-001", plan_session.Options{
    WorkDir:      "/path/to/project",
    Model:        "claude-sonnet",
    InitiativeID: "INIT-001",  // Optional
})

// Or for features
result, err := plan_session.Run(ctx, "User Authentication", plan_session.Options{
    WorkDir:     "/path/to/project",
    CreateTasks: true,
})
```

## Spec Validation

Specs are validated against minimum requirements:

| Section | Required For |
|---------|-------------|
| Intent | All weights except trivial |
| Success Criteria | All weights except trivial |
| Testing | All weights except trivial |

```go
result := ValidateSpec(content, task.WeightMedium)
if !result.Valid {
    for _, issue := range result.Issues {
        fmt.Println("Issue:", issue)
    }
}
```

## Prompt Template

The planning prompt (`builtin/plan_session.md`) guides Claude through:

1. Understanding requirements
2. Researching the codebase
3. Clarifying details
4. Proposing an approach
5. Creating structured spec document

Can be overridden at `.orc/prompts/plan.md`.

## Spawner

```go
spawner := NewSpawner(SpawnerOptions{
    WorkDir: "/path/to/project",
    Model:   "claude-sonnet",
})

err := spawner.RunInteractive(ctx, prompt)
// Spawns: claude --print -p <prompt> --dangerously-skip-permissions
// Inherits stdin/stdout/stderr for interactive use
```

## Integration with Initiatives

When linked to an initiative:
- Initiative vision included in prompt
- Prior decisions provided as context
- Tasks can be linked automatically

## Testing

```bash
go test ./internal/plan_session/... -v
```
