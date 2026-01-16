# Specification: UX: Better guidance for manual conflict resolution in worktrees

## Problem Statement

When task execution fails with sync conflicts requiring manual resolution, users receive minimal guidance. The current `TaskBlocked` message ("manually resolve conflicts then run 'orc resume'") doesn't explain how to navigate to the worktree, what git commands to run, or how to verify resolution before resuming.

## Success Criteria

- [ ] `TaskBlocked` output includes worktree path for quick navigation
- [ ] Output shows specific conflicted files list when available
- [ ] Output provides step-by-step resolution commands (cd, git fetch, git rebase/merge, conflict resolution, git add, continue)
- [ ] Output shows verification command to check conflicts are resolved before resuming
- [ ] `orc status` shows worktree conflict state for blocked tasks
- [ ] Help text is contextual - shows rebase vs merge commands based on sync strategy
- [ ] Output is formatted for easy copy-paste of commands

## Testing Requirements

- [ ] Unit test: `TaskBlocked` displays worktree path when provided
- [ ] Unit test: `TaskBlocked` displays conflicted files list when available
- [ ] Unit test: Resolution instructions include correct worktree path
- [ ] Unit test: Commands are formatted for copy-paste (no extraneous characters in command strings)
- [ ] Integration test: `orc status` shows conflict state for blocked tasks with worktrees

## Scope

### In Scope
- Enhanced `TaskBlocked` display with worktree path and file list
- Step-by-step resolution instructions with copy-pasteable commands
- `orc status` enhancement to show worktree conflict state
- Contextual help based on sync strategy (merge vs rebase)

### Out of Scope
- Automated conflict resolution improvements (separate task)
- Interactive conflict resolution TUI
- Web UI changes for conflict display
- Changes to the conflict resolver itself (ConflictResolver)
- `orc resolve` command changes (already handles worktree state)

## Technical Approach

The core change is enhancing the `progress.Display.TaskBlocked` method to accept additional context (worktree path, conflict files, sync strategy) and produce actionable output. The `orc status` command should also detect and display worktree conflict state.

### Files to Modify

1. `internal/progress/display.go`:
   - Add `BlockedContext` struct with `WorktreePath`, `ConflictFiles`, `SyncStrategy` fields
   - Update `TaskBlocked` signature to accept `BlockedContext`
   - Add helper to format step-by-step resolution commands

2. `internal/cli/cmd_run.go`:
   - Pass worktree context to `TaskBlocked` when sync fails
   - Include conflict files from executor result

3. `internal/cli/cmd_go.go`:
   - Same changes as cmd_run.go for `orc go` command

4. `internal/cli/cmd_resume.go`:
   - Same changes for consistency

5. `internal/cli/cmd_status.go`:
   - Add worktree conflict detection for blocked/running tasks
   - Display conflict files if present

6. `internal/executor/errors.go`:
   - Add `ConflictFiles` field to `ErrTaskBlocked` if not present
   - Ensure worktree path is propagated with the error

### Example Output (After)

```
Task TASK-042 blocked: sync conflict

   Worktree: .orc/worktrees/orc-TASK-042
   Conflicted files:
     - internal/api/handler.go
     - CLAUDE.md

   To resolve manually:
   ────────────────────────────────────────
   cd .orc/worktrees/orc-TASK-042
   git fetch origin
   git rebase origin/main

   # For each conflicted file:
   #   1. Edit the file to resolve conflict markers
   #   2. git add <file>

   git rebase --continue
   ────────────────────────────────────────

   Verify resolution:
     git diff --name-only --diff-filter=U  # Should show no files

   Then resume:
     orc resume TASK-042

   Total tokens: 45,231
   Total time: 12m34s
```

## Feature Analysis

### User Story

As an orc user whose task failed with sync conflicts, I want clear instructions on how to manually resolve the conflicts so that I can complete the task without searching documentation.

### Acceptance Criteria

1. When a task is blocked due to sync conflicts:
   - User sees the exact worktree path they need to navigate to
   - User sees which files have conflicts
   - User sees step-by-step git commands to resolve
   - User sees how to verify conflicts are resolved
   - User sees the exact resume command

2. When running `orc status`:
   - Blocked tasks show worktree path
   - Blocked tasks with conflicts show conflict file count
   - Running tasks with worktree issues show warnings

3. Command formatting:
   - Commands are copy-pasteable (no emoji/decorators in command strings)
   - Commands use relative paths when possible for portability
   - Commands are contextual (rebase for rebase strategy, merge for merge)
