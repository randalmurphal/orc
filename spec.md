# Specification: Fix: Execution info not persisted to database causing false orphan detection

## Problem Statement

The `ExecutionInfo` (PID, hostname, heartbeat) is set in memory by the executor when a task starts running, but is never persisted to the database. When state is loaded (via `LoadState`), the `Execution` field is always `nil`, causing `CheckOrphaned()` to return `true` for any running task after an orc restart or when viewing state from another process.

## Success Criteria

- [ ] `SaveState` persists `ExecutionInfo` fields (PID, Hostname, StartedAt, LastHeartbeat) to the `tasks` table
- [ ] `LoadState` restores `ExecutionInfo` from the database when loading state
- [ ] After orc restart, running tasks with valid execution info are NOT flagged as orphaned
- [ ] Tasks with stale heartbeats (>5 min) are still correctly detected as orphaned
- [ ] Tasks with dead PIDs are still correctly detected as orphaned
- [ ] `UpdateHeartbeat()` calls are persisted to database (heartbeat updates work)
- [ ] `ClearExecution()` clears execution columns in database

## Testing Requirements

- [ ] Unit test: `SaveState` with `ExecutionInfo` persists all fields to database
- [ ] Unit test: `LoadState` restores `ExecutionInfo` from database correctly
- [ ] Unit test: `LoadState` returns `nil` Execution when columns are NULL
- [ ] Unit test: Round-trip test - save state with execution info, load it back, verify fields match
- [ ] Integration test: Start task, save state, create new DatabaseBackend instance, load state, verify execution info present
- [ ] Existing orphan tests continue to pass

## Scope

### In Scope
- Modify `DatabaseBackend.SaveState()` to persist `ExecutionInfo` to database
- Modify `DatabaseBackend.loadStateUnlocked()` to restore `ExecutionInfo` from database
- Add execution info fields to the SQL queries in `SaveState`
- Add execution info field scanning in `LoadState`
- Add/update unit tests for execution info persistence

### Out of Scope
- Modifying the database schema (columns already exist in `project_012.sql`)
- Changing the orphan detection logic itself
- Adding new CLI commands or API endpoints
- Modifying the Task struct or TaskFull struct

## Technical Approach

The database schema already has the required columns (`executor_pid`, `executor_hostname`, `executor_started_at`, `last_heartbeat`) added in migration `project_012.sql`. The issue is that `SaveState` and `LoadState` don't use these columns.

### Files to Modify

1. **`internal/storage/database_backend.go`**:
   - `SaveState()`: Add execution info fields to the UPDATE query for the tasks table
   - `loadStateUnlocked()`: Add execution info fields to the SELECT query and populate `s.Execution`

2. **`internal/storage/database_backend_test.go`**:
   - Add tests for execution info persistence round-trip
   - Add test for heartbeat update persistence
   - Add test for execution clear persistence

## Bug Analysis

### Reproduction Steps
1. Start `orc run TASK-XXX`
2. While running, restart orc (Ctrl+C, then re-run)
3. Running task immediately shows as orphaned with reason "no execution info (legacy state or incomplete)"

### Current Behavior
- `StartExecution()` is called in executor and sets `s.Execution` in memory
- `SaveState()` is called but does NOT persist `s.Execution` to database
- On next `LoadState()`, execution info columns are not queried
- `s.Execution` is always `nil` after loading
- `CheckOrphaned()` returns `true, "no execution info (legacy state or incomplete)"`

### Expected Behavior
- `SaveState()` persists all `ExecutionInfo` fields to database columns
- `LoadState()` restores `ExecutionInfo` from database when columns have values
- Running tasks with valid execution info (alive PID, fresh heartbeat) are NOT orphaned
- Only tasks with actually stale/dead execution info are flagged as orphaned

### Root Cause
Gap between the in-memory state model (`state.State.Execution`) and the database persistence layer (`DatabaseBackend.SaveState/LoadState`) - execution info fields were added to schema but never wired into the save/load logic.

### Verification
After fix:
1. Start `orc run TASK-XXX`
2. In another terminal, run `orc status` - should show task as running (not orphaned)
3. Kill orc process, restart, run `orc status` - should show task as orphaned with "executor process not running" (correct detection)
