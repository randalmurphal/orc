# Specification: Bug: Task prints 'completed!' message when sync fails with conflicts

## Problem Statement

When a task completes all phases but the post-completion sync with the target branch fails due to merge conflicts, orc incorrectly prints the "Task TASK-XXX completed!" message even though the task status is set to `blocked`. This confuses users because the CLI message suggests success while the task actually requires manual intervention.

## Success Criteria

- [ ] When sync fails with conflicts during completion, CLI displays a blocked/warning message instead of "completed!"
- [ ] The message clearly indicates the task is blocked due to sync conflicts
- [ ] The message provides guidance on how to resolve the conflict (e.g., "run orc resume TASK-XXX after resolving conflicts")
- [ ] Existing behavior preserved: task execution success is still separate from sync/PR success
- [ ] Unit test covers the blocked-on-sync-conflict scenario
- [ ] Integration test verifies correct CLI output on sync conflict

## Testing Requirements

- [ ] Unit test: `TestTaskComplete_DoesNotPrintWhenBlocked` - verifies `TaskComplete()` is not called when task status is blocked
- [ ] Unit test: `TestExecuteTask_ReturnsBlockedOnSyncConflict` - verifies `ExecuteTask` returns a distinguishable result when blocked by sync conflict
- [ ] Integration test: E2E test that triggers a sync conflict and verifies the CLI output shows blocked message

## Scope

### In Scope
- Modify `completeTask()` to communicate the blocked-on-conflict status to callers
- Update CLI layer (`cmd_run.go`, `cmd_resume.go`, `cmd_finalize.go`, `cmd_go.go`) to handle blocked completion
- Add a display method for blocked-on-sync-conflict scenario in progress display
- Add appropriate tests

### Out of Scope
- Automatic conflict resolution improvements (already exists, this bug is about the message when resolution fails)
- Changes to how conflicts are detected or resolved
- Web UI changes (already shows correct status via WebSocket events)

## Technical Approach

The root cause is in `internal/executor/task_execution.go:completeTask()`. When sync fails with `ErrSyncConflict`:
1. The function correctly sets `t.Status = task.StatusBlocked` (line 492)
2. The function returns `nil` to indicate "task execution was successful" (line 515)
3. CLI interprets `nil` error as full success and calls `disp.TaskComplete()`

**Solution**: Return a sentinel error or result type that distinguishes "completed" from "blocked due to sync conflict" so CLI can display the appropriate message.

### Files to Modify

1. `internal/executor/task_execution.go`:
   - Create a new sentinel error `ErrTaskBlocked` that wraps the underlying sync conflict
   - Return `ErrTaskBlocked` when sync conflict blocks completion instead of `nil`
   - Add an `errors.Is(err, ErrTaskBlocked)` check for callers

2. `internal/cli/cmd_run.go`:
   - Check for `ErrTaskBlocked` after `ExecuteTask`
   - Call `disp.TaskBlocked()` instead of `disp.TaskComplete()` when blocked
   - Return `nil` (not an error) since the task itself executed correctly

3. `internal/cli/cmd_resume.go`:
   - Same changes as `cmd_run.go`

4. `internal/cli/cmd_finalize.go`:
   - Same changes as `cmd_run.go`

5. `internal/cli/cmd_go.go`:
   - Same changes as `cmd_run.go`

6. `internal/progress/display.go`:
   - Add `TaskBlocked(reason string)` method to display:
     ```
     ⚠️  Task TASK-XXX blocked: sync conflict
        To resolve: orc resume TASK-XXX after manually resolving conflicts
     ```

7. `internal/executor/task_execution_test.go`:
   - Add test for `ErrTaskBlocked` being returned on sync conflict

## Bug Analysis

### Reproduction Steps
1. Create two tasks that modify the same file
2. Run both tasks in parallel (or sequentially before merging)
3. First task completes and merges successfully
4. Second task completes all phases
5. Second task's completion sync detects conflicts
6. Observe: CLI prints "Task TASK-XXX completed!" even though task is blocked

### Current Behavior
```
🎉 Task TASK-002 completed!
   Total tokens: 5000
   Total time: 2m30s
   Files changed: 3 (+150, -20)
```

### Expected Behavior
```
⚠️  Task TASK-002 blocked: sync conflict
   All phases completed, but sync with main failed due to conflicts.
   To resolve: manually resolve conflicts then run 'orc resume TASK-002'
   Total tokens: 5000
   Total time: 2m30s
```

### Root Cause
In `internal/executor/task_execution.go:515`:
```go
// Don't return error - task execution itself was successful,
// just the post-execution sync/PR failed
return nil
```

This design intentionally returns `nil` to separate "task execution success" from "sync/PR success", but it prevents the CLI from knowing the task is actually blocked. The comment even acknowledges the decision, but the CLI layer wasn't updated to handle this case.

### Verification
After fix:
1. Blocked tasks show warning message with resolution guidance
2. Completed tasks still show celebration message
3. `orc status` shows the task as blocked (this already works correctly)
4. Web UI shows blocked status (this already works via events)
