# Specification: Add cleanup for finalizeTracker memory leak

## Problem Statement
The `finalizeTracker` global variable in `internal/api/handlers_finalize.go` accumulates `FinalizeState` entries indefinitely. Entries are added when finalize operations start but are never removed after completion or failure, causing unbounded memory growth.

## Success Criteria
- [ ] Completed/failed finalize states are removed from memory after a configurable retention period (default: 5 minutes)
- [ ] Cleanup runs automatically via background goroutine started with server
- [ ] Cleanup goroutine stops cleanly on server shutdown (no goroutine leak)
- [ ] GET `/api/tasks/{id}/finalize` still returns completed state from persistent storage after cleanup
- [ ] `finTracker.delete(taskID)` is called at appropriate time (not immediately, to allow status polling)
- [ ] No data races introduced (proper mutex usage)
- [ ] `make test` passes

## Testing Requirements
- [ ] Unit test: `TestFinalizeTrackerCleanup` verifies old completed/failed entries are removed
- [ ] Unit test: `TestFinalizeTrackerCleanupPreservesRunning` verifies running entries are preserved
- [ ] Unit test: `TestFinalizeTrackerCleanupShutdown` verifies cleanup goroutine stops on context cancel
- [ ] Integration test: Verify GET finalize status works after entry cleaned up (falls back to state.yaml)

## Scope
### In Scope
- Add cleanup method to `finalizeTracker` that removes stale completed/failed entries
- Add background goroutine to run cleanup periodically
- Wire cleanup into server lifecycle (start/stop with server)
- Configure retention period

### Out of Scope
- Changing the finalize status API response format
- Persisting in-progress finalize state to disk
- Adding config options (use sensible defaults for now)

## Technical Approach
The fix adds a background cleanup routine that periodically removes completed/failed finalize states older than a retention period. The cleanup respects server shutdown via context cancellation.

### Files to Modify
- `internal/api/handlers_finalize.go`:
  - Add `cleanupStale(retention time.Duration)` method to `finalizeTracker`
  - Add `startCleanup(ctx context.Context, interval, retention time.Duration)` method
  - Export cleanup start function for server integration

- `internal/api/server.go`:
  - Start finalize tracker cleanup in `StartContext()`
  - No explicit stop needed (context cancellation handles it)

- `internal/api/handlers_finalize_test.go`:
  - Add unit tests for cleanup behavior

## Bug Analysis

**Root Cause:** The `runFinalizeAsync` function (line 279) updates `finState.Status` to `FinalizeStatusCompleted` or `FinalizeStatusFailed` but never calls `finTracker.delete(taskID)`.

**Current Behavior:** Each finalize operation adds an entry to `finTracker.states` that remains in memory forever:
```go
// Line 175: Entry added
finTracker.set(taskID, finState)

// Lines 448-466: Status updated but entry NOT removed
finState.Status = FinalizeStatusCompleted
// Missing: finTracker.delete(taskID)
```

**Expected Behavior:** Completed/failed entries should be cleaned up after a reasonable retention period to allow clients to poll for status.

**Verification:** After the fix:
1. Memory usage should stabilize over time (not grow with each finalize)
2. `GET /api/tasks/{id}/finalize` should continue working (falls back to persistent state)
3. Duplicate finalize requests should still be detected during the operation
