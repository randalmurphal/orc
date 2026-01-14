# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Bug: Task worktree created from stale base causes completion conflicts

**Description**: When parallel tasks run and the first one merges to main, subsequent tasks' worktrees are based on a stale commit. During completion sync, this causes merge conflicts that aren't automatically resolved.

**Reproduction:**
1. Run TASK-A and TASK-B in parallel (both modify CLAUDE.md)
2. TASK-A completes and merges first
3. TASK-B completes but conflicts on CLAUDE.md

**Root cause:** Worktree is created at task start from current main, but by completion time main has moved forward.

**Fix options:**
1. Rebase task branch onto main BEFORE the implement phase starts
2. Add conflict resolution phase that uses AI to resolve conflicts
3. Auto-rebase during completion sync before attempting merge
4. Create worktree just-in-time when task runs (not when task is created)



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

Let me provide a summary of the implementation:

## Summary

I've implemented a fix for the bug where task worktrees created from a stale base cause completion conflicts when parallel tasks run and merge in different orders.

### Changes Made:

**1. Config (`internal/config/config.go`):**
- Added `SyncOnStart` field to `SyncConfig` struct (default: `true`)
- Added `ShouldSyncOnStart()` helper method that respects the sync strategy

**2. Executor (`internal/executor/pr.go`):**
- Added `syncOnTaskStart()` method that rebases the task branch onto the latest target branch before execution begins
- Handles conflicts gracefully according to existing config options (`fail_on_conflict`, `max_conflict_files`)

**3. Task Execution (`internal/executor/task_execution.go`):**
- Added sync-on-start call after worktree setup but before phases begin
- Only runs when `ShouldSyncOnStart()` and `ShouldSyncForWeight()` return true

**4. Git (`internal/git/git.go`):**
- Exported `GetCommitCounts()` method for use by the executor

**5. Tests:**
- Added `TestShouldSyncOnStart` with 7 test cases covering all strategy combinations
- Added `TestSyncOnStart_DefaultEnabled` to verify default config
- Added `TestSyncOnTaskStart_SkipsWithoutGitOps` and `TestSyncOnTaskStart_UsesWorktreeGit`

**6. Documentation:**
- Updated `docs/architecture/GIT_INTEGRATION.md` with new "Sync on Start" section
- Added config option to `CLAUDE.md` key config table
- Added env var mapping to `docs/specs/CONFIG_HIERARCHY.md`

### How It Works:

1. When `orc run TASK-XXX` executes and worktree is set up
2. Before the first phase runs, if `sync_on_start: true` (default):
   - Fetch latest from origin
   - Check how many commits behind target branch
   - If behind, attempt rebase onto target
   - If conflicts: fail or warn based on `fail_on_conflict` setting
3. Implement phase now sees latest code including parallel task changes
4. Completion sync will have fewer/no conflicts

### Config Options:

```yaml
completion:
  sync:
    sync_on_start: true      # NEW - default: true
    strategy: completion     # existing
    fail_on_conflict: true   # existing
```

<phase_complete>true</phase_complete>

---
Tokens: 5789246 input, 15922 output, 99693 cache_creation, 5686475 cache_read
Complete: true
Blocked: false
