# Internal Package Guide

## Package Overview

| Package | Purpose | Key Types |
|---------|---------|-----------|
| `api/` | gRPC server + REST gateway | `Server`, `*Server` services |
| `budgets/` | Budget enforcement (pre-execution spending checks) | `Enforcer`, `CostStore`, `BudgetStore` |
| `cli/` | Cobra command tree | `rootCmd`, subcommands |
| `config/` | Merged YAML + env config | `Config`, `Loader` |
| `db/` | Database layer (SQLite + PostgreSQL) | `GlobalDB`, `ProjectDB` |
| `events/` | Real-time event publishing | `Publisher`, `Event` |
| `executor/` | Phase execution engine | `Executor`, `PhaseRunner` |
| `gate/` | Quality gate evaluation | `Evaluator`, `GateResult` |
| `git/` | Git + worktree operations | `Service` |
| `jira/` | Jira Cloud import | `Client`, `Importer` |
| `llmutil/` | LLM interaction utilities | `ExecuteWithSchema[T]()` |
| `phase/` | Phase model + registry | `Phase`, `Registry` |
| `project/` | Multi-project management | `Registry`, `Project` |
| `prompt/` | Template rendering | `Service`, `TemplateData` |
| `storage/` | Task persistence | `Backend` interface |
| `task/` | Task domain model | `Task`, `Status` |
| `trigger/` | Lifecycle event triggers | `Engine`, `TriggerDef` |
| `variable/` | Template variable resolution | `Resolver` |
| `workflow/` | Workflow definitions | `Workflow`, `Registry` |

## Key Interfaces

### Storage Backend (`storage/backend.go`)
```go
type Backend interface {
    SaveTask(ctx, task) error
    GetTask(ctx, id) (*task.Task, error)
    ListTasks(ctx, filter) ([]*task.Task, error)
    // ... ~20 methods for tasks, initiatives, phases, events
}
```
All API services use this interface. SQLite and PostgreSQL implement it via `db.ProjectDB`.

### Phase Runner (`executor/runner.go`)
```go
type PhaseRunner interface {
    Run(ctx, task, phase, prompt) (*PhaseResult, error)
}
```
Wraps Claude Code CLI execution. `ClaudeRunner` is the production implementation.

## Common Patterns

### Adding a New API Endpoint

1. Define in `api/proto/orc/v1/*.proto`
2. Generate: `make proto`
3. Implement in `api/*_server.go`
4. Add route in `api/gateway.go` if REST needed

### Adding a New Phase

1. Register in `phase/registry.go`
2. Create template in `templates/<phase>.md`
3. Add to workflow definitions in `workflow/`

### Database Schema Changes

1. Add migration in `db/migrations/` (separate global/project)
2. Update `GlobalDB` or `ProjectDB` methods
3. Run `make test` to verify migration chain

## Testing Conventions

- Table-driven tests with `t.Run()`
- `testutil` package for common helpers
- Integration tests use `testutil.SetupTestDB()`
- Mock interfaces, not implementations

## Error Wrapping

Always wrap errors with context:
```go
return fmt.Errorf("executor run phase %s: %w", phase.Name, err)
```

Never: `return err` without context in exported functions.
