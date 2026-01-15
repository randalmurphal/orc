# Specification: Fix detectConflictsViaMerge cleanup on failure

## Problem Statement

The `detectConflictsViaMerge` function in `internal/git/git.go` performs merge and reset operations to detect conflicts, but cleanup (merge abort + reset) only runs at the end of the function. If an error or panic occurs after the merge attempt, the worktree is left in a broken state with a merge in progress.

## Success Criteria

- [ ] Cleanup (`merge --abort` and `reset --hard`) runs via `defer` to guarantee execution even on error/panic
- [ ] Function still returns correct `SyncResult` with conflict information when conflicts are detected
- [ ] Function still returns correct `SyncResult` when no conflicts exist
- [ ] Worktree is never left in merge-in-progress state after function returns (success or failure)
- [ ] Existing tests continue to pass

## Testing Requirements

- [ ] Unit test: `TestDetectConflictsViaMerge_CleanupOnError` - verify cleanup runs when an error occurs mid-function
- [ ] Unit test: Existing `TestDetectConflicts_NoConflicts` passes
- [ ] Unit test: Existing `TestDetectConflicts_WithConflicts` passes
- [ ] Integration: Run `make test` with all git tests passing

## Scope

### In Scope
- Refactor `detectConflictsViaMerge` to use `defer` for cleanup
- Ensure cleanup is idempotent (safe to call even if merge wasn't started)

### Out of Scope
- Changes to `DetectConflicts` (the public method)
- Changes to `merge-tree` based conflict detection (the preferred path)
- Changes to other git operations

## Technical Approach

### Current Code (problematic)
```go
func (g *Git) detectConflictsViaMerge(target string) (*SyncResult, error) {
    // ... safety checks ...

    head, err := g.ctx.HeadCommit()  // Get HEAD for reset
    if err != nil {
        return nil, err  // Early return - no cleanup needed (merge not started)
    }

    _, mergeErr := g.ctx.RunGit("merge", "--no-commit", "--no-ff", target)  // MERGE STARTS

    // ... conflict detection logic ...

    // CLEANUP - only runs if we reach this point!
    _, _ = g.ctx.RunGit("merge", "--abort")
    _, _ = g.ctx.RunGit("reset", "--hard", head)

    return result, nil
}
```

### Fixed Code (using defer)
```go
func (g *Git) detectConflictsViaMerge(target string) (*SyncResult, error) {
    // ... safety checks ...

    head, err := g.ctx.HeadCommit()
    if err != nil {
        return nil, err
    }

    // Defer cleanup BEFORE merge attempt - guaranteed to run
    defer func() {
        _, _ = g.ctx.RunGit("merge", "--abort")
        _, _ = g.ctx.RunGit("reset", "--hard", head)
    }()

    _, mergeErr := g.ctx.RunGit("merge", "--no-commit", "--no-ff", target)

    // ... conflict detection logic ...

    return result, nil
}
```

### Files to Modify
- `internal/git/git.go:545-582`: Refactor `detectConflictsViaMerge` to use `defer` for cleanup
- `internal/git/git_test.go`: Add test to verify cleanup runs on error

## Bug Analysis

### Reproduction Steps
1. Call `DetectConflicts()` on a worktree with git < 2.38 (or force fallback)
2. Have the merge attempt succeed but a subsequent operation (e.g., `diff --name-only`) fail
3. Observe worktree left in merge-in-progress state

### Current Behavior
Cleanup only runs if the function reaches line 577-579. Any panic, runtime error, or early return after the merge starts would skip cleanup, leaving:
- `MERGE_HEAD` file present
- Working tree in conflict state
- Subsequent operations fail with "merge in progress" error

### Expected Behavior
Cleanup always runs after merge is attempted, regardless of function exit path.

### Root Cause
Cleanup code is placed at the end of the function instead of using Go's `defer` mechanism, which guarantees execution even during panics or early returns.

### Verification
1. Run existing tests - they should still pass
2. Add new test that simulates error during conflict detection
3. Verify `IsMergeInProgress()` returns false after function exits (success or failure)
