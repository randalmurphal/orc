# Specification: Bug: Sync fails with '0 files in conflict' error

## Problem Statement

The `syncOnTaskStart` function fails with error "sync conflict detected: task branch has 0 files in conflict with target" when rebase fails but no actual conflict files are detected. If there are 0 conflicts, sync should succeed (or provide a more accurate error message), not fail with a misleading "0 files in conflict" error.

## Bug Analysis

### Reproduction Steps

1. Run a task that is behind the target branch (e.g., TASK-180)
2. `sync_on_start: true` is configured (default)
3. The underlying `git rebase` command fails for some reason other than merge conflicts (e.g., dirty working tree, uncommitted changes, or edge case rebase issue)
4. `git diff --name-only --diff-filter=U` returns empty string (no unmerged files)
5. Error message shows "0 files in conflict" which is confusing

### Current Behavior

In `internal/git/git.go:RebaseWithConflictCheck()` (lines 634-647):

```go
// Attempt rebase
_, rebaseErr := g.ctx.RunGit("rebase", target)
if rebaseErr != nil {
    // Check for conflicts
    output, _ := g.ctx.RunGit("diff", "--name-only", "--diff-filter=U")
    if output != "" {
        result.ConflictsDetected = true
        result.ConflictFiles = strings.Split(strings.TrimSpace(output), "\n")
    }

    // Abort the rebase
    _, _ = g.ctx.RunGit("rebase", "--abort")

    return result, fmt.Errorf("%w: %d files have conflicts", ErrMergeConflict, len(result.ConflictFiles))
}
```

**Problem**: When rebase fails but `git diff --name-only --diff-filter=U` returns empty:
- `ConflictsDetected` stays `false`
- `ConflictFiles` stays `nil` (length 0)
- Error returned is `ErrMergeConflict` with "0 files have conflicts"

This gets propagated to `syncOnTaskStart` which shows:
```
sync conflict detected: task branch has 0 files in conflict with target
Conflicting files: []
```

### Expected Behavior

1. If `git diff --name-only --diff-filter=U` returns empty (no unmerged files), the rebase failure is NOT a merge conflict - it's a different error
2. The function should:
   - If no unmerged files detected: return the raw rebase error (NOT `ErrMergeConflict`)
   - If unmerged files detected: return `ErrMergeConflict` with the conflict count

### Root Cause

The code unconditionally returns `ErrMergeConflict` when rebase fails, even when no actual merge conflicts are detected. The rebase could have failed for other reasons:
- Dirty working tree
- Uncommitted changes that would be lost
- Rebase already in progress
- Other git internal errors

## Success Criteria

- [ ] `RebaseWithConflictCheck` returns `ErrMergeConflict` ONLY when actual conflicts are detected (`len(ConflictFiles) > 0`)
- [ ] When rebase fails without conflicts, the raw rebase error is returned (not wrapped in `ErrMergeConflict`)
- [ ] `syncOnTaskStart` displays accurate error messages (no "0 files in conflict")
- [ ] Existing tests in `internal/git/git_test.go` continue to pass
- [ ] New test case covers the "rebase fails without conflicts" scenario

## Testing Requirements

- [ ] Unit test: `TestRebaseWithConflictCheck_FailWithoutConflicts` - verify correct error type when rebase fails but no conflicts detected
- [ ] Unit test: Existing `TestRebaseWithConflictCheck_Conflict` continues to pass (regression)
- [ ] Unit test: Existing `TestRebaseWithConflictCheck_Success` continues to pass (regression)
- [ ] Integration test: Verify `syncOnTaskStart` error message is clear when rebase fails

## Scope

### In Scope
- Fix `RebaseWithConflictCheck()` to return appropriate error types
- Update `syncOnTaskStart()` to handle non-conflict rebase failures gracefully
- Add unit test for the new behavior

### Out of Scope
- Changes to conflict resolution logic
- Changes to the finalize phase sync logic
- UI changes for error display

## Technical Approach

The fix is straightforward: only return `ErrMergeConflict` when we actually detect conflicts.

### Files to Modify

1. **`internal/git/git.go`** (lines 634-647):
   - Modify `RebaseWithConflictCheck()` to only return `ErrMergeConflict` when `len(ConflictFiles) > 0`
   - When `len(ConflictFiles) == 0`, return the raw rebase error with context

2. **`internal/git/git_test.go`**:
   - Add test `TestRebaseWithConflictCheck_FailWithoutConflicts`
   - Simulate rebase failure without conflicts (e.g., dirty working tree)

### Implementation Details

```go
// In RebaseWithConflictCheck - after checking for conflicts:
if rebaseErr != nil {
    // Check for conflicts
    output, _ := g.ctx.RunGit("diff", "--name-only", "--diff-filter=U")
    if output != "" {
        result.ConflictsDetected = true
        result.ConflictFiles = strings.Split(strings.TrimSpace(output), "\n")
    }

    // Abort the rebase
    _, _ = g.ctx.RunGit("rebase", "--abort")

    // FIXED: Only return ErrMergeConflict if we actually detected conflicts
    if result.ConflictsDetected {
        return result, fmt.Errorf("%w: %d files have conflicts", ErrMergeConflict, len(result.ConflictFiles))
    }
    // Rebase failed for another reason (dirty tree, etc.)
    return result, fmt.Errorf("rebase failed: %w", rebaseErr)
}
```

No changes needed in `syncOnTaskStart()` - it already checks for `errors.Is(err, git.ErrMergeConflict)` and will handle other errors appropriately.
