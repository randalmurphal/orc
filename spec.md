# Specification: Fix: SaveTask overwrites executor fields (false orphan detection)

## Problem Statement

When `SaveTask` is called to update a task (e.g., changing status, title, or other task fields), it overwrites the executor tracking fields (`executor_pid`, `executor_hostname`, `executor_started_at`, `last_heartbeat`) with zero values. This causes running tasks to be falsely flagged as orphaned because the orphan detection logic sees no executor info.

## Success Criteria

- [ ] `SaveTask` preserves executor fields (`ExecutorPID`, `ExecutorHostname`, `ExecutorStartedAt`, `LastHeartbeat`) from the existing database row when updating a task
- [ ] A running task's executor fields survive calls to `SaveTask` for unrelated updates (title, status, etc.)
- [ ] Orphan detection correctly identifies running tasks as NOT orphaned after `SaveTask` is called
- [ ] `SaveState` remains the authoritative way to set/clear executor fields (no change to SaveState behavior)
- [ ] Unit test verifies that `SaveTask` preserves executor fields set by prior `SaveState`
- [ ] Unit test verifies orphan detection works correctly after a task update via `SaveTask`

## Testing Requirements

- [ ] Unit test: `TestSaveTask_PreservesExecutorFields` - Save a task, set executor fields via SaveState, update task via SaveTask, verify executor fields are preserved
- [ ] Unit test: `TestSaveTask_PreservesExecutorFields_OrphanDetection` - After the SaveTask update, verify LoadState returns correct ExecutionInfo and CheckOrphaned returns false (not orphaned)

## Scope

### In Scope
- Fix `SaveTaskCtx` in `internal/storage/database_backend.go` to preserve executor fields
- Add unit tests validating the fix
- Update documentation in CLAUDE.md knowledge section

### Out of Scope
- Changing how `SaveState` handles executor fields (it already works correctly)
- Modifying the `task.Task` struct to include executor fields (they belong in state, not task)
- Refactoring the task/state separation (just fixing the preservation bug)

## Technical Approach

The fix is minimal: extend the preservation logic in `SaveTaskCtx` (which already preserves `StateStatus` and `RetryContext`) to also preserve the executor fields.

### Files to Modify

1. `internal/storage/database_backend.go`:
   - In `SaveTaskCtx`, after reading the existing task and preserving `StateStatus`/`RetryContext`, also preserve:
     - `dbTask.ExecutorPID = existingTask.ExecutorPID`
     - `dbTask.ExecutorHostname = existingTask.ExecutorHostname`
     - `dbTask.ExecutorStartedAt = existingTask.ExecutorStartedAt`
     - `dbTask.LastHeartbeat = existingTask.LastHeartbeat`

2. `internal/storage/database_backend_test.go`:
   - Add `TestSaveTask_PreservesExecutorFields` test
   - Add `TestSaveTask_PreservesExecutorFields_OrphanDetection` test

3. `CLAUDE.md`:
   - Add entry to Known Gotchas table documenting this fix

## Bug Analysis

### Reproduction Steps
1. Create a task and start execution (SaveState with ExecutionInfo)
2. Call SaveTask to update an unrelated field (e.g., title)
3. Load the state and check ExecutionInfo
4. Observe ExecutionInfo is nil/zeroed

### Current Behavior (Bug)
`SaveTaskCtx` (line 77-110) calls `taskToDBTask` which creates a fresh `db.Task` with zero values for executor fields. While it preserves `StateStatus` and `RetryContext` from the existing row, it doesn't preserve executor fields. The SQL upsert then overwrites the database with these zero values.

### Expected Behavior
`SaveTask` should only update task-related fields and preserve state-related fields (including executor tracking) that are managed by `SaveState`.

### Root Cause
Lines 84-90 of `database_backend.go`:
```go
existingTask, err := d.db.GetTask(t.ID)
if err == nil && existingTask != nil {
    dbTask.StateStatus = existingTask.StateStatus
    dbTask.RetryContext = existingTask.RetryContext
    // MISSING: executor field preservation
}
```

The preservation logic was incomplete - it handled `StateStatus` and `RetryContext` but not the executor fields added later for orphan detection (TASK-242).

### Verification
After the fix:
1. SaveState sets executor fields → fields persist in DB
2. SaveTask updates task → executor fields remain unchanged
3. LoadState returns correct ExecutionInfo → orphan detection works
