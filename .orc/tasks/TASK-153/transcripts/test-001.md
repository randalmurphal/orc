# test - Iteration 1

## Prompt

Final test verification for: Auto-commit task files on creation and modification

**Original task**: ## Problem
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

1. Run the full test suite
2. Fix any failures found
3. Verify edge cases are covered

Keep iterating until all tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

No failures. All tests pass.

## Summary

The auto-commit functionality for task files has been successfully implemented and verified:

### What was implemented:
1. **`internal/task/commit.go`** - Core commit functions:
   - `CommitAndSync()` - Commits task creation/modification
   - `CommitDeletion()` - Commits task deletion
   - `CommitStatusChange()` - Commits status changes

2. **`internal/cli/cmd_new.go`** - Auto-commit on task creation (line 336-348)

3. **`internal/cli/cmd_edit.go`** - Auto-commit on task edit (line 297-307)

4. **`internal/config/config.go`** - `DisableAutoCommit` config option to disable the feature

### Tests:
- `TestCommitAndSync` - Tests task creation commit
- `TestCommitDeletion` - Tests task deletion commit
- `TestCommitStatusChange` - Tests status change commit
- `TestDefaultCommitConfig` - Tests default configuration

### Commit message format:
```
[orc] task TASK-001: created - Task Title
[orc] task TASK-001: updated - Title
[orc] task TASK-001: status running - Title
[orc] task TASK-001: deleted
```

### Configuration:
- Enabled by default
- Can be disabled via `tasks.disable_auto_commit: true` in config

<phase_complete>true</phase_complete>

---
Tokens: 2711282 input, 8157 output, 43456 cache_creation, 2667773 cache_read
Complete: true
Blocked: false
