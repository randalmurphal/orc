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
| Phase Output | `SavePhaseOutput`, `GetPhaseOutput`, `GetPhaseOutputByVarName`, `GetAllPhaseOutputs`, `LoadPhaseOutputsAsMap`, `GetPhaseOutputsForTask`, `DeletePhaseOutput`, `PhaseOutputExists` |
| Spec (via Phase Output) | `GetSpecForTask`, `GetFullSpecForTask`, `SpecExistsForTask`, `SaveSpecForTask` |
| Initiative | `SaveInitiative`, `LoadInitiative`, `LoadAllInitiatives`, `DeleteInitiative`, `InitiativeExists`, `GetNextInitiativeID` |
| Phase Template | `SavePhaseTemplate`, `GetPhaseTemplate`, `ListPhaseTemplates`, `DeletePhaseTemplate` |
| Workflow | `SaveWorkflow`, `GetWorkflow`, `ListWorkflows`, `DeleteWorkflow`, workflow phases/variables operations |
| Workflow Run | `SaveWorkflowRun`, `GetWorkflowRun`, `ListWorkflowRuns`, `DeleteWorkflowRun`, `GetNextWorkflowRunID` |
| Transcript | `AddTranscript`, `AddTranscriptBatch`, `GetTranscripts`, `SearchTranscripts` |
| Attachment | `SaveAttachment`, `GetAttachment`, `ListAttachments`, `DeleteAttachment` |
| Comments | `ListTaskComments`, `SaveTaskComment`, `ListReviewComments`, `SaveReviewComment` |
| Gates | `ListGateDecisions`, `SaveGateDecision` |
| Events | `SaveEvent`, `SaveEvents`, `QueryEvents` |
| Branch | `SaveBranch`, `LoadBranch`, `ListBranches`, `UpdateBranchStatus`, `UpdateBranchActivity`, `DeleteBranch`, `GetStaleBranches` |
| Constitution | `SaveConstitution`, `LoadConstitution`, `ConstitutionExists`, `DeleteConstitution` |
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

### LoadAllStates Optimization

`LoadAllStates()` uses batch queries to avoid N+1 problem:

```go
// 3 queries total instead of 3N queries:
dbTasks, _ := d.db.ListTasks(db.ListOpts{})       // 1 query
allPhases, _ := d.db.GetAllPhasesGrouped()        // 1 query
allGates, _ := d.db.GetAllGateDecisionsGrouped()  // 1 query

// Build states from pre-fetched maps
for _, dbTask := range dbTasks {
    s := d.buildStateFromData(dbTask, allPhases[dbTask.ID], allGates[dbTask.ID])
}
```

| Operation | Before | After |
|-----------|--------|-------|
| 100 tasks | 301 queries | 3 queries |

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

## Transcript Batch Persistence

`AddTranscriptBatch(ctx, []Transcript)` writes multiple transcript entries in a single database transaction. Used by `executor.TranscriptBuffer` for efficient streaming transcript persistence.

**Key characteristics:**
- All transcripts inserted atomically (single transaction)
- Empty slice is a no-op (no error)
- `Transcript` struct includes: `TaskID`, `Phase`, `Iteration`, `Role`, `Content`, `Timestamp`

**Role values:**
- `"prompt"` - User/system prompts
- `"response"` - Model responses
- `"chunk"` - Aggregated streaming chunks
- `"combined"` - Full transcript for phase

## Event Persistence

`PersistentPublisher` (in `events/` package) wraps `MemoryPublisher` to add database persistence while maintaining real-time WebSocket broadcasts.

### Event Operations

| Operation | Description |
|-----------|-------------|
| `SaveEvent(e)` | Persist single event to `event_log` table |
| `SaveEvents(events)` | Batch insert (transactional) for efficiency |
| `QueryEvents(opts)` | Retrieve events with filtering and pagination |

### How It Works

```
API Server → PersistentPublisher → MemoryPublisher → WebSocket clients
                    ↓
              DatabaseBackend → event_log table
```

1. Events broadcast to WebSocket subscribers immediately (real-time)
2. Events buffered in memory (10 events or 5 seconds)
3. Buffer flushed to database in batch transaction
4. Phase completion events trigger immediate flush
5. DB failures logged but don't block WebSocket broadcast

### Phase Duration Tracking

`PersistentPublisher` tracks phase start times. When phase completes:
- Calculates `duration_ms` from start to completion
- Includes duration in the persisted event
- Cleans up start time entry to prevent memory leaks

### Usage

```go
// API server creates PersistentPublisher on startup
pub := events.NewPersistentPublisher(backend, "executor", logger)
defer pub.Close()  // Flushes remaining events

// All standard Publisher methods work
pub.Publish(event)
ch := pub.Subscribe("TASK-001")
pub.Unsubscribe("TASK-001", ch)
```
