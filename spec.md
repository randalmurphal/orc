# Specification: Clean up WorkerPool.workers map on completion

## Problem Statement

Workers in `WorkerPool.workers` map are not cleaned up when they complete or fail. They remain in the map until the orchestrator's next `tick()` cycle calls `checkWorkers()`, which leads to memory leaks, incorrect capacity counting, and potential race conditions when re-running tasks.

## Success Criteria

- [ ] Worker removes itself from `WorkerPool.workers` map upon completion
- [ ] Worker removes itself from `WorkerPool.workers` map upon failure
- [ ] Capacity check (`len(p.workers) >= p.maxWorkers`) only counts actively running workers
- [ ] No race condition when same task ID is re-queued after completion
- [ ] `ActiveCount()` returns correct count immediately after worker completion (not delayed until next tick)
- [ ] Events are still published before worker cleanup (completion events must fire first)

## Testing Requirements

- [ ] Unit test: `TestWorkerPoolCleansUpOnCompletion` - spawn worker, let it complete, verify map is empty
- [ ] Unit test: `TestWorkerPoolCleansUpOnFailure` - spawn worker that fails, verify map is empty
- [ ] Unit test: `TestWorkerPoolCapacityAfterCompletion` - fill pool to capacity, complete one worker, verify new worker can spawn immediately
- [ ] Unit test: `TestConcurrentWorkerCleanup` - multiple workers completing simultaneously don't cause race conditions

## Scope

### In Scope
- Worker self-cleanup from pool map on completion/failure in `Worker.run()`
- Proper locking to prevent race conditions during cleanup
- Maintaining event publishing order (events before cleanup)

### Out of Scope
- Worktree cleanup (already handled by orchestrator's `handleWorkerComplete/Failed`)
- Scheduler state updates (already handled by orchestrator)
- Changes to the orchestrator's main loop or tick mechanism

## Technical Approach

The worker needs a reference to the pool to remove itself. Options:

1. **Pass pool reference to Worker** - Worker gets `pool` in `run()` already, use it for self-removal
2. **Callback function** - Pass a cleanup callback to the worker

Option 1 is simpler since `run(pool, ...)` already receives the pool.

### Implementation Plan

1. In `Worker.run()` defer block, call `pool.RemoveWorker(w.TaskID)` after setting final status
2. Ensure events are published before removal (already happening inside `run()`)
3. The orchestrator's `handleWorkerComplete/Failed` will then be a no-op for removal (already removed), but still handles worktree cleanup

**Key insight**: The orchestrator's `checkWorkers()` iterates over `GetWorkers()` which returns a copy. Workers that complete between ticks will already be removed from the map by self-cleanup, but the copy will still have stale entries. We need to:
- Skip workers that no longer exist in the pool (already removed)
- Or rely on status being already set to complete/failed for idempotent handling

### Files to Modify

- `internal/orchestrator/worker.go`:
  - `Worker.run()`: Add `pool.RemoveWorker(w.TaskID)` in defer block after status update
  - Ensure proper ordering: set status -> publish events -> remove from map

- `internal/orchestrator/orchestrator.go`:
  - `handleWorkerComplete()`: Make idempotent - check if worker still exists before operations
  - `handleWorkerFailed()`: Make idempotent - check if worker still exists before operations

- `internal/orchestrator/worker_test.go` (new file):
  - Add unit tests for cleanup behavior

## Bug Analysis

### Reproduction Steps
1. Create WorkerPool with `maxWorkers=2`
2. Spawn 2 workers for TASK-001 and TASK-002
3. Both workers complete (change status to `complete`)
4. Try to spawn worker for TASK-003 before next orchestrator tick
5. **Bug**: Spawn fails with "worker pool at capacity (2)"

### Current Behavior
- Workers complete and set `Status = WorkerStatusComplete`
- Workers remain in `p.workers` map until orchestrator's next `tick()` calls `checkWorkers()`
- `checkWorkers()` calls `RemoveWorker()` to clean up
- Between completion and tick: `len(p.workers)` includes completed workers

### Expected Behavior
- Workers should remove themselves from map immediately upon completion/failure
- `len(p.workers)` should only count actively running workers
- New workers can spawn immediately after another completes

### Root Cause
`Worker.run()` sets status but doesn't call `pool.RemoveWorker()`. The cleanup is delegated to `checkWorkers()` which runs on poll interval.

**Location**: `internal/orchestrator/worker.go:122-128` (defer block in `run()`)

### Verification
1. Run unit tests for cleanup behavior
2. Verify `ActiveCount()` returns correct value immediately after worker completion
3. Verify new worker can spawn right after another completes (no tick delay)
