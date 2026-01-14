# implement - Iteration 1

## Prompt

Implement the following task:

**Task**: Auto-commit task files on creation and modification

**Description**: ## Problem
Task files can get lost if not tracked in git. When tasks are created or modified, changes should be committed automatically (like initiatives already do).

## Solution
Add auto-commit behavior to task operations:

1. On task creation (`orc new`): commit the new task.yaml and plan.yaml
2. On task modification (`orc edit`): commit the updated task.yaml
3. On status changes: commit state updates

Use same pattern as initiatives:
```
[orc] task TASK-001: created - Title
[orc] task TASK-001: updated - changed weight to medium
[orc] task TASK-001: status - running
```

## Success Criteria
1. `orc new` auto-commits the task files
2. `orc edit` auto-commits changes
3. Commit messages follow consistent format
4. Can be disabled via config if needed

## Files to Modify
- internal/task/store.go (add git commit after save)
- internal/cli/cmd_new.go (ensure commit happens)
- internal/cli/cmd_edit.go (ensure commit happens)



1. Implement the required changes
2. Write/update tests as needed
3. Run tests and fix any failures

Keep iterating until implementation is complete and all tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

Let me provide a summary of what was implemented:

## Summary

I have implemented auto-commit for task files following the same pattern used by initiatives. Here's what was done:

### Files Created
1. **`internal/task/commit.go`** - New module with git commit functions:
   - `CommitAndSync()` - Commits task files after creation/modification
   - `CommitDeletion()` - Commits task deletion
   - `CommitStatusChange()` - Commits status changes
   - `DefaultCommitConfig()` - Returns sensible defaults

2. **`internal/task/commit_test.go`** - Tests for all commit functions

### Files Modified
1. **`internal/config/config.go`**:
   - Added `TasksConfig` struct with `DisableAutoCommit` option
   - Added `Tasks` field to main `Config` struct
   - Added default value (`DisableAutoCommit: false`)

2. **`internal/cli/cmd_new.go`**:
   - Added auto-commit call after task creation with commit message format: `[orc] task TASK-001: created - Title`

3. **`internal/cli/cmd_edit.go`**:
   - Added auto-commit call after task edit with commit message format: `[orc] task TASK-001: updated weight, title - Title`

4. **`CLAUDE.md`** (both copies):
   - Added `tasks.disable_auto_commit` to Key Config Options table
   - Added pattern entry in Knowledge section

### Commit Message Format
The implementation uses consistent commit messages:
```
[orc] task TASK-001: created - Task Title
[orc] task TASK-001: updated weight, title - Task Title
[orc] task TASK-001: status running - Task Title
[orc] task TASK-001: deleted
```

### Configuration
Auto-commit can be disabled via config:
```yaml
tasks:
  disable_auto_commit: true
```

### Tests
All tests pass (existing and new):
- `TestCommitAndSync` - Tests task commit on creation
- `TestCommitDeletion` - Tests task deletion commit
- `TestCommitStatusChange` - Tests status change commit
- `TestDefaultCommitConfig` - Tests default config values

<phase_complete>true</phase_complete>

---
Tokens: 4262952 input, 15384 output, 82877 cache_creation, 4180022 cache_read
Complete: true
Blocked: false
