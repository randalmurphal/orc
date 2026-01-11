# Internal Packages

Core Go packages for the orc orchestrator. Each package has a single responsibility.

## Package Overview

| Package | Responsibility | Key Types |
|---------|----------------|-----------|
| `api/` | HTTP server, REST endpoints, WebSocket | `Server`, handlers |
| `cli/` | Command-line interface (Cobra) | Commands |
| `claude/` | Re-exports llmkit/claudeconfig types | `Settings`, `Skill` |
| `config/` | Configuration loading and management | `Config` |
| `detect/` | Project type detection | `Detector` |
| `enhance/` | Task enhancement via AI | `Enhancer` |
| `errors/` | Custom error types | `OrcError` |
| `events/` | Event publishing for real-time updates | `Publisher`, `Event` |
| `executor/` | Phase execution engine | `Executor`, `Result` |
| `gate/` | Quality gates and approval workflow | `Gate`, `Evaluator` |
| `git/` | Git operations, worktrees | `Git`, `Checkpoint` |
| `plan/` | Plan generation from templates | `Plan`, `Phase` |
| `progress/` | Progress tracking and display | `Tracker` |
| `project/` | Multi-project registry | `Registry`, `Project` |
| `prompt/` | Prompt template management | `Service` |
| `state/` | Execution state persistence | `State` |
| `task/` | Task model and YAML persistence | `Task`, `Store` |
| `template/` | Go template rendering | `Engine` |
| `tokenpool/` | OAuth token pool for rate limit failover | `Pool`, `Account` |
| `wizard/` | Interactive CLI wizard | `Wizard` |

## Dependency Graph

```
cmd/orc
    └── cli/
        ├── api/
        │   ├── events/
        │   ├── executor/
        │   │   ├── events/
        │   │   ├── git/
        │   │   ├── plan/
        │   │   ├── prompt/
        │   │   ├── state/
        │   │   ├── task/
        │   │   └── tokenpool/
        │   ├── project/
        │   ├── prompt/
        │   └── task/
        ├── executor/
        ├── git/
        ├── plan/
        ├── state/
        └── task/
```

## Key Patterns

### Error Handling
All errors wrap context for traceability:
```go
return fmt.Errorf("load task %s: %w", id, err)
```

### Functional Options
Constructors use functional options pattern:
```go
executor := NewExecutor(
    WithGitSvc(gitSvc),
    WithPublisher(publisher),
    WithLogger(logger),
)
```

### Interface-Based Design
Core components use interfaces for testability:
```go
type Publisher interface {
    Publish(event Event)
}
```

## Testing

Each package has comprehensive tests. Run all:
```bash
go test ./internal/... -v
```

Run specific package:
```bash
go test ./internal/executor/... -v
```

## Package Details

See package-specific CLAUDE.md files:
- `api/CLAUDE.md` - API server and handlers
- `cli/CLAUDE.md` - CLI commands
- `executor/CLAUDE.md` - Execution engine modules
