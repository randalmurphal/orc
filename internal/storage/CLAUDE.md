# Storage Package

Storage backend abstraction layer. SQLite is the sole source of truth for all data.

## Overview

| File | Purpose |
|------|---------|
| `backend.go` | `Backend` interface definition, types |
| `database_backend.go` | `DatabaseBackend` implementation (SQLite) |
| `factory.go` | Backend factory (`NewBackend`) |
| `export.go` | Export functionality for task artifacts |

## Backend Interface

All storage operations are defined by the `Backend` interface:

| Category | Operations |
|----------|------------|
| Task | `SaveTask`, `LoadTask`, `LoadAllTasks`, `DeleteTask`, `TaskExists`, `GetNextTaskID` |
| State | `SaveState`, `LoadState`, `LoadAllStates` |
| Plan | `SavePlan`, `LoadPlan` |
| Spec | `SaveSpec`, `LoadSpec`, `SpecExists` |
| Initiative | `SaveInitiative`, `LoadInitiative`, `LoadAllInitiatives`, `DeleteInitiative`, `InitiativeExists`, `GetNextInitiativeID` |
| Transcript | `AddTranscript`, `GetTranscripts`, `SearchTranscripts` |
| Attachment | `SaveAttachment`, `GetAttachment`, `ListAttachments`, `DeleteAttachment` |
| Comments | `ListTaskComments`, `SaveTaskComment`, `ListReviewComments`, `SaveReviewComment` |
| Gates | `ListGateDecisions`, `SaveGateDecision` |
| Branch | `SaveBranch`, `LoadBranch`, `ListBranches`, `UpdateBranchStatus`, `UpdateBranchActivity`, `DeleteBranch`, `GetStaleBranches` |
| Context | `MaterializeContext`, `NeedsMaterialization` |
| Lifecycle | `Sync`, `Cleanup`, `Close` |

## DatabaseBackend

Primary implementation using SQLite via the `db` package.

### Features

- All data stored in `.orc/orc.db`
- Thread-safe: mutex protects all operations
- JSON serialization for complex fields (phases, metadata)
- Batch loading to avoid N+1 queries
- Foreign key cascades for deletion

### Concurrency

```go
// Read operations use RLock
d.mu.RLock()
defer d.mu.RUnlock()

// Write operations use Lock
d.mu.Lock()
defer d.mu.Unlock()
```

### Transaction Support

Multi-table write operations use transactions for atomicity. This prevents partial updates if any operation fails:

| Operation | Tables Modified |
|-----------|-----------------|
| `SaveTask` / `SaveTaskCtx` | `tasks`, `task_dependencies` |
| `SaveState` / `SaveStateCtx` | `tasks`, `phases`, `gate_decisions` |
| `SaveInitiative` / `SaveInitiativeCtx` | `initiatives`, `initiative_decisions`, `initiative_tasks`, `initiative_dependencies` |

Example: `SaveTaskCtx` wraps task + dependencies in a single transaction:

```go
return d.db.RunInTx(ctx, func(tx *db.TxOps) error {
    if err := db.SaveTaskTx(tx, dbTask); err != nil {
        return err  // Rollback
    }
    if err := db.ClearTaskDependenciesTx(tx, t.ID); err != nil {
        return err  // Rollback
    }
    for _, depID := range t.BlockedBy {
        if err := db.AddTaskDependencyTx(tx, t.ID, depID); err != nil {
            return err  // Rollback
        }
    }
    return nil  // Commit
})
```

If any step fails, all changes are rolled back, ensuring database consistency.

### Context-Aware Methods

For operations requiring cancellation or timeout support, use the `*Ctx` variants:

| Method | Context-Aware Variant | Purpose |
|--------|----------------------|---------|
| `SaveTask` | `SaveTaskCtx` | Save task with context propagation |
| `SaveState` | `SaveStateCtx` | Save execution state with context |
| `SaveInitiative` | `SaveInitiativeCtx` | Save initiative with context |

The `*Ctx` methods propagate context through `RunInTx` to `TxOps`, which stores and uses the context for all database operations within the transaction. This enables:

- **Request cancellation**: HTTP handlers can pass `r.Context()` to abort DB operations when client disconnects
- **Timeouts**: Wrap context with `context.WithTimeout()` to limit operation duration
- **Tracing**: Future integration with OpenTelemetry spans

The non-context methods (`SaveTask`, `SaveState`, `SaveInitiative`) use `context.Background()` internally and remain for backward compatibility.

### Direct DB Access

```go
backend := storage.NewDatabaseBackend(projectPath, cfg)
db := backend.DB() // WARNING: bypasses mutex
```

Use Backend interface methods for thread-safety.

## Usage

```go
import "github.com/randalmurphal/orc/internal/storage"

// Create backend
backend, err := storage.NewBackend(projectPath, &config.StorageConfig{})

// Save task
err = backend.SaveTask(task)

// Load task
task, err := backend.LoadTask("TASK-001")

// Load all tasks with dependencies
tasks, err := backend.LoadAllTasks()

// Close when done
defer backend.Close()
```

## Data Flow

```
CLI/API → Backend interface → DatabaseBackend → db.ProjectDB → SQLite
```

All reads and writes go through the backend. No YAML files are created.

## State vs Task Status

The database stores two status fields:

| Field | Values | Purpose |
|-------|--------|---------|
| `Status` (task) | created, classifying, planned, running, paused, blocked, finalizing, completed, finished, failed | UI display, workflow |
| `StateStatus` | pending, running, completed, failed, paused, interrupted, skipped | Execution engine state |

`SaveState` updates `StateStatus`, while `SaveTask` updates `Status`.
