# Specification: Fix worktree cleanup after task completion - 27+ worktrees were left orphaned

## Problem Statement

Worktrees are not being properly cleaned up after task completion, resulting in orphaned worktrees accumulating in `.orc/worktrees/`. Investigation revealed multiple failure paths where cleanup is skipped or fails silently, particularly for initiative-prefixed worktrees.

## Project Context

### Patterns to Follow

| Pattern | Example Location | How to Apply |
|---------|------------------|--------------|
| Error wrapping | `internal/executor/worktree.go:96` | Wrap errors with context: `fmt.Errorf("cleanup worktree: %w", err)` |
| Git ops abstraction | `internal/git/git.go:262-271` | Use git.Git methods, don't shell out directly |
| Backend interface | `internal/storage/backend.go` | Use Backend for task lookups in cleanup |
| Config-driven behavior | `internal/executor/worktree.go:248-261` | Check `cfg.Worktree.CleanupOnComplete` |

### Affected Code

| File | Current Behavior | After This Change |
|------|------------------|-------------------|
| `internal/executor/task_execution.go:297-314` | Uses `CleanupWorktree(t.ID)` ignoring initiative prefix | Uses stored worktree path directly |
| `internal/git/git.go:262-271` | `CleanupWorktree` only takes taskID | Adds path-based cleanup method |
| `internal/cli/cmd_cleanup.go:166-254` | Regex-based orphan detection misses initiative prefixes | Parses all worktree names regardless of prefix |
| `internal/api/server.go` (or init) | No startup cleanup | Prunes stale worktrees on startup |

### Breaking Changes

- [x] Backward compatible - all changes are additive or fix existing bugs

## Success Criteria

| ID | Criterion | Verification Method | Expected Result | Error Path |
|----|-----------|---------------------|-----------------|------------|
| SC-1 | Worktrees for completed tasks with initiative prefix are cleaned up | `orc cleanup --dry-run` after completing initiative task | Reports 0 orphaned worktrees | If initiative lookup fails, fall back to worktreePath scan |
| SC-2 | Executor uses stored worktree path for cleanup | Unit test: mock git ops, verify path passed to cleanup | Cleanup called with exact path from setup | Log warning if path missing, attempt ID-based fallback |
| SC-3 | `orc cleanup` detects initiative-prefixed worktrees | `orc cleanup --dry-run` with initiative worktrees present | Lists all orphaned worktrees regardless of prefix | Gracefully handle parse errors |
| SC-4 | Server startup prunes stale worktree entries | Start server, verify `git worktree prune` ran | Log message about pruning | Prune failure is non-fatal, logged as warning |
| SC-5 | Blocked-to-completed transition triggers cleanup | Resolve blocked task, check worktree removed | Worktree directory deleted | Log warning on cleanup failure, don't fail transition |

## Testing Requirements

| Test Type | Description | Command |
|-----------|-------------|---------|
| Unit | `CleanupWorktreeAtPath` correctly removes worktree by path | `go test ./internal/git -run TestCleanupWorktreeAtPath` |
| Unit | `findOrphanedWorktrees` detects initiative-prefixed worktrees | `go test ./internal/cli -run TestFindOrphanedWorktrees_InitiativePrefix` |
| Unit | `cleanupWorktreeForTask` uses stored path when available | `go test ./internal/executor -run TestCleanupWorktreeUsesStoredPath` |
| Integration | Task completion with initiative cleans worktree | `go test ./tests/integration -run TestInitiativeTaskCleanup` |
| Unit | Startup pruning runs without errors | `go test ./internal/api -run TestServerStartupPrunesWorktrees` |

## Scope

### In Scope

- Store actual worktree path in executor and use it for cleanup
- Add `CleanupWorktreeAtPath` method to git.Git for path-based cleanup
- Update `findOrphanedWorktrees` to detect worktrees regardless of naming convention
- Add startup pruning to `orc serve` initialization
- Add cleanup trigger on task state transitions to terminal states
- Unit tests for all new functionality

### Out of Scope

- Scheduled cleanup daemon (future enhancement)
- Cleanup of worktrees from deleted tasks
- Remote worktree cleanup
- Changing default `cleanup_on_complete` configuration
- GUI for worktree management

## Technical Approach

The fix addresses three root causes:

### 1. Use Stored Worktree Path for Cleanup

Currently `cleanupWorktreeForTask` reconstructs the path using `gitOps.CleanupWorktree(t.ID)`, which doesn't account for initiative prefixes. The fix stores the actual path during `setupWorktreeForTask` and uses it directly.

```go
// In cleanupWorktreeForTask
if e.worktreePath != "" {
    // Use stored path directly - handles all naming conventions
    if err := e.gitOps.CleanupWorktreeAtPath(e.worktreePath); err != nil {
        e.logger.Warn("failed to cleanup worktree", "error", err)
    }
}
```

### 2. Add Path-Based Cleanup to git.Git

Add a new method that takes an explicit path instead of computing it from taskID:

```go
func (g *Git) CleanupWorktreeAtPath(worktreePath string) error {
    return g.ctx.CleanupWorktree(worktreePath)
}
```

### 3. Fix Orphan Detection in `orc cleanup`

Update the regex pattern to extract task IDs from any worktree naming convention:

```go
// Match TASK-XXX anywhere in the directory name
taskIDPattern := regexp.MustCompile(`(TASK-\d+)`)
```

### Files to Modify

- `internal/executor/task_execution.go`:
  - Modify `cleanupWorktreeForTask` to use `e.worktreePath` instead of reconstructing path
  - Add fallback to ID-based cleanup if path is empty (for resume scenarios)

- `internal/git/git.go`:
  - Add `CleanupWorktreeAtPath(path string) error` method
  - Keep existing `CleanupWorktree(taskID string)` for backward compatibility

- `internal/cli/cmd_cleanup.go`:
  - Update `findOrphanedWorktrees` regex to match TASK-XXX anywhere in path
  - Add logic to detect initiative-prefixed directories

- `internal/api/server.go` (or appropriate startup location):
  - Call `gitOps.PruneWorktrees()` during server initialization
  - Log the result (info level)

### New Files

None - all changes are to existing files.

## Bug Analysis

### Reproduction Steps

1. Create a task belonging to an initiative with `branch_prefix: "feature/auth-"`
2. Run `orc run TASK-XXX` until completion
3. Check `.orc/worktrees/` - worktree `feature-auth-TASK-XXX` still exists
4. Run `orc cleanup --dry-run` - may not detect it as orphaned
5. Repeat with multiple tasks - worktrees accumulate

### Current Behavior

- `cleanupWorktreeForTask` calls `gitOps.CleanupWorktree(t.ID)`
- `CleanupWorktree` computes path as `orc-TASK-XXX` (without initiative prefix)
- Actual worktree is at `feature-auth-TASK-XXX`
- Cleanup silently "succeeds" because `orc-TASK-XXX` doesn't exist
- Worktree `feature-auth-TASK-XXX` remains orphaned

### Expected Behavior

- `cleanupWorktreeForTask` uses the stored `e.worktreePath`
- Cleanup removes the actual worktree at `feature-auth-TASK-XXX`
- `orc cleanup` detects and lists worktrees regardless of naming prefix

### Root Cause

The worktree path is computed during setup with initiative prefix but reconstructed without it during cleanup. The `e.worktreePath` field is set correctly in `setupWorktreeForTask` but ignored in `cleanupWorktreeForTask`.

Location: `internal/executor/task_execution.go:297-314`

```go
// BUG: This reconstructs path without initiative prefix
if err := e.gitOps.CleanupWorktree(t.ID); err != nil {
```

Should be:

```go
// FIX: Use stored path that was set during setup
if err := e.gitOps.CleanupWorktreeAtPath(e.worktreePath); err != nil {
```

### Verification

After the fix:

1. Create task with initiative prefix
2. Complete task execution
3. Verify worktree is removed: `ls .orc/worktrees/` should not show the task
4. Run `orc cleanup --dry-run` - should report 0 orphaned worktrees
5. Run `git worktree list` - should only show main repo and active tasks

## Failure Modes

| Failure Scenario | Expected Behavior | User Feedback | Test |
|------------------|-------------------|---------------|------|
| Stored worktree path is empty | Fall back to ID-based cleanup | Warning in logs | `TestCleanupFallbackToIDPath` |
| Git worktree remove fails | Log error, continue | Warning in logs | `TestCleanupWorktreeError` |
| Initiative lookup fails | Use default path calculation | Warning in logs | `TestCleanupWithoutInitiative` |
| Startup prune fails | Log error, continue server startup | Warning in logs | `TestStartupPruneError` |
| Worktree doesn't exist | No-op, return success | No message | `TestCleanupNonexistentWorktree` |

## Edge Cases

| Input/State | Expected Behavior | Test |
|-------------|-------------------|------|
| Task has no worktree (e.g., trivial task) | Skip cleanup, no error | `TestCleanupNoWorktree` |
| Worktree already deleted manually | `CleanupWorktree` succeeds (idempotent) | `TestCleanupAlreadyDeleted` |
| Task in worktree being cleaned (concurrent) | Cleanup waits for or skips active | `TestConcurrentCleanup` |
| e.gitOps is nil | Skip cleanup, no panic | `TestCleanupNilGitOps` |
| Task completed but e.worktreePath empty (resume) | Fall back to ID-based lookup | `TestCleanupResumedTask` |
| Multiple worktrees for same task ID | Clean all matching (shouldn't happen) | `TestCleanupDuplicateWorktrees` |

## Review Checklist

### Code Quality

- [ ] Linting passes (`golangci-lint run ./...` returns 0 errors)
- [ ] Type checking passes (`go vet ./...` returns 0 errors)
- [ ] No TODOs or debug statements in new code
- [ ] Error messages include context for debugging

### Test Coverage

- [ ] Coverage >= 85% on new code
- [ ] All success criteria have tests
- [ ] All edge cases tested
- [ ] All failure modes tested
- [ ] Existing tests still pass

### Integration

- [ ] No merge conflicts with main
- [ ] Build succeeds (`make build`)
- [ ] `make test` passes
- [ ] Manual verification: complete a task with initiative, verify cleanup

## Open Questions

None - the root cause and fix are well understood. Implementation is straightforward.
