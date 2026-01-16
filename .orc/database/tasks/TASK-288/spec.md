# Specification: CLI: orc resolve should support blocked tasks or guide users to correct command

## Problem Statement

The `orc resolve` command currently only accepts failed tasks (`status=failed`) and rejects blocked tasks (`status=blocked`) with an unhelpful error message. Users trying to "resolve" a blocked task (e.g., one waiting for manual intervention or gate approval) get no guidance on what command to use instead. This creates confusion between two unrelated "blocked" concepts: (1) execution-blocked (task waiting for human gate approval), and (2) dependency-blocked (task waiting on other tasks to complete).

## Success Criteria

- [ ] `orc resolve TASK-XXX` on a blocked task provides clear guidance to the correct command
- [ ] Error message distinguishes between execution-blocked tasks (use `orc approve/resume`) and other blocked states
- [ ] Error message includes actionable command suggestions with the task ID
- [ ] Unit tests verify correct error message content for blocked tasks
- [ ] No regression: `orc resolve` continues to work correctly for failed tasks

## Testing Requirements

- [ ] Unit test: `TestResolveCommand_BlockedTask_GuidesToCorrectCommand` - verifies blocked task error includes approve/resume guidance
- [ ] Unit test: `TestResolveCommand_OnlyFailedTasks` already covers status rejection; verify message quality
- [ ] Integration test: Manual verification that error messages are helpful and actionable

## Scope

### In Scope

- Improve error message when `orc resolve` is run on a blocked task
- Include specific command suggestions (`orc approve`, `orc resume`) in the error
- Include the task ID in suggested commands for copy-paste convenience

### Out of Scope

- Adding blocked task support to `orc resolve` (these are different workflows)
- Changing the semantics of `orc approve` or `orc resume`
- Handling dependency-blocked tasks (those with `blocked_by` references to incomplete tasks)
- Interactive prompts to auto-select the correct command

## Technical Approach

The fix is a simple error message improvement. Currently the error says:
```
task TASK-XXX is blocked, not failed; resolve is only for failed tasks
```

The improved error should say:
```
task TASK-XXX is blocked (status: blocked), not failed

For blocked tasks, use one of these commands instead:
  orc approve TASK-XXX   Approve a gate and mark task ready to run
  orc resume TASK-XXX    Resume execution (for paused/blocked/failed tasks)

The 'resolve' command is for marking failed tasks as complete without re-running.
```

### Files to Modify

- `internal/cli/cmd_resolve.go:152-153`: Improve error message for blocked tasks with actionable guidance
- `internal/cli/cmd_resolve_test.go`: Add test verifying error message content for blocked tasks

## Feature Details

### User Story

As a user who runs `orc resolve TASK-XXX` on a blocked task, I want to see clear guidance on what command to use instead, so that I can quickly take the correct action without having to look up documentation.

### Acceptance Criteria

1. Running `orc resolve` on a task with `status=blocked` produces an error message that:
   - States the task is blocked (not failed)
   - Suggests `orc approve TASK-XXX` for gate approval
   - Suggests `orc resume TASK-XXX` for resuming execution
   - Briefly explains what `orc resolve` is actually for

2. The error message includes the actual task ID for easy copy-paste

3. The error message format is consistent with other CLI error messages (no emoji unless user has enabled them)
