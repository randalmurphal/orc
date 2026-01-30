# Storage Package

Per-project storage backend abstraction layer. Each `Backend` wraps a single `ProjectDB` -- all operations are project-scoped.

## Overview

| File | Purpose |
|------|---------|
| `backend.go` | `Backend` interface, `DatabaseBackend` struct |
| `storage.go` | Backend factory (`NewBackend`), setup |
| `testing.go` | Test helpers: `NewTestBackend()`, `NewTestGlobalDB()` |
| `task.go` | Task CRUD operations including execution state |
| `initiative.go` | Initiative CRUD operations |
| `workflow.go` | Workflow and phase template operations |
| `queries.go` | Query/search operations |
| `config.go` | Config storage operations |
| `export.go` | Export functionality for task artifacts |
| `import.go` | Import functionality |
| `cleanup.go` | Cleanup and maintenance operations |

## Multi-Project Architecture

Each project gets its own `Backend` instance wrapping a `ProjectDB`. The API layer routes requests to the correct backend via `ProjectCache` (`internal/api/project_cache.go`).

```
Request (project_id) → ProjectCache (LRU) → Backend → ProjectDB → SQLite
                                                          ↑
                                              One per project (.orc/orc.db)
```

`ProjectCache` is an LRU cache of open `ProjectDB` + `Backend` pairs, keyed by project ID. Evicts least-recently-used entries when at capacity. Lives in the API layer, not in storage.

## Task-Centric Approach

All execution state is embedded in `orcv1.Task.Execution`. The proto type `orcv1.Task` (from `gen/proto/orc/v1/task.pb.go`) is the domain model used throughout the codebase.

**Note:** `db.Task` is an internal DTO used only within the storage layer for database mapping. It is not exposed outside storage - all public APIs use `*orcv1.Task`.

**Proto conversion** (`proto_convert.go`) maps all fields between `orcv1.Task` and `db.Task`, including branch control fields:

| Proto Field | DB Field | Conversion Notes |
|-------------|----------|------------------|
| `BranchName *string` | `BranchName *string` | Direct pointer copy |
| `TargetBranch *string` | `TargetBranch string` | `ptrToString` / `stringToPtr` |
| `PrDraft *bool` | `PrDraft *bool` | Direct pointer copy |
| `PrLabels []string` | `PrLabels string` | JSON marshal/unmarshal |
| `PrReviewers []string` | `PrReviewers string` | JSON marshal/unmarshal |
| `PrLabelsSet bool` | `PrLabelsSet bool` | Tracks explicit empty vs unset |
| `PrReviewersSet bool` | `PrReviewersSet bool` | Tracks explicit empty vs unset |

| Operation | Method |
|-----------|--------|
| Save task with execution state | `backend.SaveTask(t)` where `t.Execution` contains phases, gates, cost |
| Load task with execution state | `backend.LoadTask(taskID)` then access `t.Execution` |
| Load all tasks with execution state | `backend.LoadAllTasks()` then access each `t.Execution` |

## Backend Interface

All storage operations are defined by the `Backend` interface:

| Category | Operations |
|----------|------------|
| Task | `SaveTask`, `LoadTask`, `LoadAllTasks`, `DeleteTask`, `TaskExists`, `GetNextTaskID` |
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

### LoadAllTasks Optimization

`LoadAllTasks()` uses batch queries to avoid N+1 problem:

```go
// 4 queries total instead of 4N queries:
dbTasks, _ := d.db.ListTasks(db.ListOpts{})       // 1 query
allDeps, _ := d.db.GetAllTaskDependencies()       // 1 query
allPhases, _ := d.db.GetAllPhasesGrouped()        // 1 query
allGates, _ := d.db.GetAllGateDecisionsGrouped()  // 1 query

// Build tasks with execution state from pre-fetched maps
for _, dbTask := range dbTasks {
    t := dbTaskToTask(&dbTask)
    t.BlockedBy = allDeps[t.ID]
    t.Execution.Phases = allPhases[t.ID]
    t.Execution.Gates = allGates[t.ID]
}
```

| Operation | Before | After |
|-----------|--------|-------|
| 100 tasks | 401 queries | 4 queries |

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
| `SaveTask` / `SaveTaskCtx` | `tasks`, `task_dependencies`, `phases`, `gate_decisions` |
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
| `SaveInitiative` | `SaveInitiativeCtx` | Save initiative with context |

The `*Ctx` methods propagate context through `RunInTx` to `TxOps`, which stores and uses the context for all database operations within the transaction. This enables:

- **Request cancellation**: HTTP handlers can pass `r.Context()` to abort DB operations when client disconnects
- **Timeouts**: Wrap context with `context.WithTimeout()` to limit operation duration
- **Tracing**: Future integration with OpenTelemetry spans

The non-context methods (`SaveTask`, `SaveInitiative`) use `context.Background()` internally and remain for backward compatibility.

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
CLI   → NewBackend(projectPath) → DatabaseBackend → db.ProjectDB → SQLite
API   → ProjectCache.GetBackend(projectID) → DatabaseBackend → db.ProjectDB → SQLite
```

All reads and writes go through the backend. No YAML files are created.

## Task Status vs Phase Status

The database stores task-level and phase-level status:

| Field | Type | Values | Purpose |
|-------|------|--------|---------|
| Task status | `orcv1.TaskStatus` | created, classifying, planned, running, paused, blocked, finalizing, completed, finished, failed | UI display, workflow |
| Phase status | `orcv1.PhaseStatus` | pending, running, completed, failed, paused, interrupted, skipped, blocked | Per-phase execution state |

Phase status is stored in `orcv1.Task.Execution.Phases[phaseID].Status`.

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

## Testing

`testing.go` provides in-memory test helpers (10-100x faster than file-based):

```go
// Per-project backend (in-memory ProjectDB)
backend := storage.NewTestBackend(t)

// Global database (in-memory GlobalDB for workflows, agents, costs)
globalDB := storage.NewTestGlobalDB(t)
```

Both auto-cleanup via `t.Cleanup()`. Always use `t.Parallel()` with these helpers.
