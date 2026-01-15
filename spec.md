# Specification: Add context propagation to DatabaseBackend methods

## Problem Statement
The `DatabaseBackend` methods currently don't accept or propagate `context.Context`, making it impossible to implement request timeouts, cancellation, or tracing for database operations. This limits observability and control in long-running operations or when integrating with HTTP request handlers.

## Success Criteria
- [ ] All public `Backend` interface methods accept `context.Context` as their first parameter
- [ ] All `DatabaseBackend` method implementations propagate context to underlying db operations
- [ ] Context cancellation properly terminates in-flight database operations
- [ ] All callers of `Backend` methods are updated to pass appropriate context
- [ ] Existing tests pass with updated signatures
- [ ] No regression in functionality (operations work identically with `context.Background()`)

## Testing Requirements
- [ ] Unit test: Verify context cancellation aborts long-running operations (e.g., `LoadAllTasks` with cancelled context returns context error)
- [ ] Unit test: Verify `context.Background()` works identically to current behavior (backward compatibility)
- [ ] Integration test: Verify API handlers properly propagate request context to storage operations
- [ ] Test: Verify transaction operations (`SaveTask`, `SaveState`, `SaveInitiative`) respect context

## Scope
### In Scope
- Update `Backend` interface with context parameters
- Update `DatabaseBackend` implementation to accept and propagate context
- Update all callers (CLI commands, API handlers, orchestrator, template) to pass context
- Update `TxOps` methods to use the provided context instead of `context.Background()`
- Update existing tests to pass context

### Out of Scope
- Adding request tracing/spans (future enhancement)
- Adding per-operation timeouts (callers can wrap context with timeout)
- Changes to the underlying `db` package methods (they already accept context via driver)
- PostgreSQL dialect changes (already has context-aware methods)

## Technical Approach

### Phase 1: Update Backend Interface
Add `context.Context` as the first parameter to all interface methods:

```go
type Backend interface {
    // Task operations
    SaveTask(ctx context.Context, t *task.Task) error
    LoadTask(ctx context.Context, id string) (*task.Task, error)
    LoadAllTasks(ctx context.Context) ([]*task.Task, error)
    DeleteTask(ctx context.Context, id string) error
    TaskExists(ctx context.Context, id string) (bool, error)
    GetNextTaskID(ctx context.Context) (string, error)
    // ... all other methods
}
```

### Phase 2: Update DatabaseBackend Implementation
Propagate context through to all database operations. For transactions, pass context to `RunInTx`:

```go
func (d *DatabaseBackend) SaveTask(ctx context.Context, t *task.Task) error {
    d.mu.Lock()
    defer d.mu.Unlock()
    return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
        // ...
    })
}
```

### Phase 3: Update TxOps to Use Context
Currently `TxOps.Exec/Query/QueryRow` use `context.Background()`. Update to accept context parameter or store it:

```go
type TxOps struct {
    tx      driver.Tx
    dialect driver.Dialect
    ctx     context.Context  // Store context from RunInTx
}
```

### Phase 4: Update All Callers
Update callers to pass context:
- CLI commands: Use `cmd.Context()` from cobra
- API handlers: Use `r.Context()` from HTTP request
- Orchestrator: Propagate context from worker
- Tests: Use `context.Background()` or `t.Context()`

### Files to Modify
- `internal/storage/backend.go`: Add context to interface
- `internal/storage/database_backend.go`: Update all methods to accept and propagate context
- `internal/storage/database_backend_test.go`: Update tests
- `internal/db/project.go`: Update `TxOps` to store and use context
- `internal/cli/cmd_*.go`: Update all CLI commands using backend (~20 files)
- `internal/api/handlers.go`: Update API handlers
- `internal/orchestrator/orchestrator.go`: Update orchestrator
- `internal/orchestrator/worker.go`: Update worker
- `internal/template/template.go`: Update template loader

## Refactor Analysis

### Before Pattern
```go
// Backend interface without context
type Backend interface {
    SaveTask(t *task.Task) error
    LoadTask(id string) (*task.Task, error)
}

// Implementation ignores cancellation
func (d *DatabaseBackend) LoadTask(id string) (*task.Task, error) {
    d.mu.RLock()
    defer d.mu.RUnlock()
    dbTask, err := d.db.GetTask(id)  // No way to cancel
    // ...
}
```

### After Pattern
```go
// Backend interface with context
type Backend interface {
    SaveTask(ctx context.Context, t *task.Task) error
    LoadTask(ctx context.Context, id string) (*task.Task, error)
}

// Implementation respects cancellation
func (d *DatabaseBackend) LoadTask(ctx context.Context, id string) (*task.Task, error) {
    d.mu.RLock()
    defer d.mu.RUnlock()
    dbTask, err := d.db.GetTaskContext(ctx, id)  // Respects context
    // ...
}
```

### Risk Assessment
| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Breaking existing callers | Certain | Low | Compile-time errors guide updates |
| Incorrect context propagation | Low | Medium | Test with cancellation scenarios |
| Performance regression | Very Low | Low | Context adds minimal overhead |
| Deadlock from context in mutex | Low | High | Release lock before blocking on context |

### Migration Strategy
1. Update interface first (causes compile errors)
2. Fix implementation methods one by one
3. Fix callers starting with tests (catch issues early)
4. Fix CLI and API callers last

## Method Count by Category

| Category | Methods | Context Needed |
|----------|---------|----------------|
| Task ops | 6 | All |
| State ops | 3 | All |
| Plan ops | 2 | All |
| Spec ops | 3 | All |
| Initiative ops | 6 | All |
| Transcript ops | 3 | All |
| Attachment ops | 4 | All |
| Context/Lifecycle | 5 | Sync/Cleanup only |
| **Total** | **32** | **30** |
