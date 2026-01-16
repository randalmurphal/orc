# Specification: CLI: orc resolve should support --force for stuck running tasks

## Problem Statement

When a task is stuck in 'running' status but has a merged PR (e.g., executor crashed after merge), there's no CLI way to mark it complete. Currently `orc resolve` only works on failed tasks, requiring direct database updates for stuck running tasks.

## Success Criteria

- [ ] `orc resolve TASK-XXX --force` works on tasks with any status (running, paused, blocked, created, planned, etc.)
- [ ] Without `--force`, the command continues to require `status=failed` (preserves current behavior)
- [ ] If task has a merged PR (`PR.Status == PRStatusMerged` or `PR.Merged == true`), command auto-detects and reports this
- [ ] If task has no PR or PR is not merged, command warns user about potential incomplete work
- [ ] Resolution metadata includes `force_resolved=true` when `--force` is used on non-failed tasks
- [ ] Resolution metadata includes `pr_was_merged=true` if PR was merged at resolution time
- [ ] Command output clearly indicates the action taken (resolved, PR merge status)

## Testing Requirements

- [ ] Unit test: `TestResolveCommand_ForceOnRunningTask` - verifies --force works on running tasks
- [ ] Unit test: `TestResolveCommand_ForceOnPausedTask` - verifies --force works on paused tasks
- [ ] Unit test: `TestResolveCommand_ForceOnBlockedTask` - verifies --force works on blocked tasks (overrides the helpful error)
- [ ] Unit test: `TestResolveCommand_ForceOnCreatedTask` - verifies --force works on created tasks
- [ ] Unit test: `TestResolveCommand_ForceWithMergedPR` - verifies merged PR detection and metadata
- [ ] Unit test: `TestResolveCommand_ForceWithoutPR` - verifies warning when no PR exists
- [ ] Unit test: `TestResolveCommand_ForceWithOpenPR` - verifies warning when PR exists but not merged
- [ ] Unit test: `TestResolveCommand_WithoutForceStillRequiresFailed` - confirms non-force behavior unchanged

## Scope

### In Scope
- Adding `--force` flag behavior to bypass status check in `orc resolve`
- Detecting PR merge status from task.PR field
- Adding appropriate warnings for potential incomplete work
- Adding resolution metadata to track force-resolved tasks
- Updating help text to document new behavior

### Out of Scope
- Fetching live PR status from GitHub (use cached task.PR data)
- Automatic worktree cleanup (existing `--cleanup` flag handles this)
- Adding new task statuses (use existing `StatusCompleted` with metadata)
- Changing the confirmation prompt behavior (still skippable with `-f`)

## Technical Approach

The implementation extends the existing `orc resolve` command to accept `--force` for non-failed tasks.

### Files to Modify

- `internal/cli/cmd_resolve.go`:
  - Modify status check (lines 154-166) to allow any status when `--force` is set
  - Add PR merge detection before status update
  - Add warnings for missing/unmerged PR when force-resolving
  - Add metadata tracking (`force_resolved`, `pr_was_merged`, `original_status`)
  - Update help text to document `--force` on non-failed tasks

- `internal/cli/cmd_resolve_test.go`:
  - Add test cases for force-resolving running/paused/blocked/created tasks
  - Add test cases for PR merge detection
  - Add test case to verify non-force behavior is unchanged

## Bug Analysis

### Reproduction Steps
1. Run `orc run TASK-XXX`
2. Task creates PR and PR gets merged
3. Executor crashes or loses connection after merge but before marking task complete
4. Task is stuck in `status=running` with merged PR
5. Run `orc resolve TASK-XXX` - fails with "task is running, not failed"
6. Only workaround is direct SQLite update

### Current Behavior
```
$ orc resolve TASK-201
Error: task TASK-201 is running, not failed; resolve is only for failed tasks
```

### Expected Behavior
```
$ orc resolve TASK-201 --force
PR merged (PR #123)
Task TASK-201 marked as resolved (was: running)

$ orc resolve TASK-202 --force  # task with no PR
Warning: No PR found for this task. Work may be incomplete.
Task TASK-202 marked as resolved (was: running)

$ orc resolve TASK-203 --force  # task with open PR
Warning: PR #45 is not merged (status: pending_review). Work may be incomplete.
Task TASK-203 marked as resolved (was: running)
```

### Root Cause
The status check at `cmd_resolve.go:154` explicitly rejects all non-failed tasks:
```go
if t.Status != task.StatusFailed {
    // ... only allows failed tasks
    return fmt.Errorf("task %s is %s, not failed; resolve is only for failed tasks", id, t.Status)
}
```

### Verification
After the fix:
1. Create a task with `orc new "test"`
2. Set it to running manually or via `orc run`
3. Run `orc resolve TASK-XXX --force`
4. Verify task status is `completed` with metadata `force_resolved=true`
5. Verify original behavior still works: `orc resolve TASK-YYY` on non-failed task without `--force` still fails
