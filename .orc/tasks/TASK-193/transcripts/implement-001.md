# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Comprehensive auto-commit: ALL .orc/ file mutations must commit

**Description**: ## Problem
TASK-153 and TASK-191 added auto-commit for some task operations, but many mutations still don't auto-commit:

### Task operations missing auto-commit:
- Status changes during execution (running, completed, failed)
- State file updates (state.yaml)
- Transcript additions
- Phase transitions
- Retry context updates
- Token/cost tracking updates

### Initiative operations missing auto-commit:
- Status changes (draft → active → completed)
- Task linking/unlinking
- Decision additions
- Progress updates

### Other .orc/ changes:
- Config changes via API
- Project registration
- Any file watcher-triggered changes

## Solution
Audit ALL code paths that write to .orc/ and ensure they call CommitAndSync:

1. **Task lifecycle:**
   - internal/executor/*.go - all state updates
   - internal/task/store.go - all Save() calls
   
2. **Initiative lifecycle:**
   - internal/initiative/*.go - all mutations
   
3. **API handlers:**
   - internal/api/handlers*.go - all POST/PUT/DELETE

4. **Consider:** Centralized hook that commits after any .orc/ file write

## Success Criteria
1. `git status` shows clean after ANY orc operation
2. No manual commits needed for .orc/ files ever
3. Commit messages are descriptive of what changed



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

All tasks are complete. Let me provide a summary of what was implemented:

## Summary: Comprehensive Auto-Commit Implementation (TASK-193)

### Files Modified

| File | Changes |
|------|---------|
| `internal/executor/executor.go` | Added `commitTaskState()` and `commitTaskStatus()` helpers |
| `internal/executor/task_execution.go` | Auto-commit at: task start, phase completion, failure, setup failure, task completion, finalize phase, merge |
| `internal/executor/pr.go` | Auto-commit after PR creation and successful merge |
| `internal/state/orphan.go` | Added `commitOrphanRecovery()` for orphan task recovery |
| `internal/api/handlers_projects.go` | Added `autoCommitProjectTask()` and `autoCommitProjectTaskDeletion()` for all project task operations |
| `internal/api/handlers_initiatives.go` | Added `autoCommitInitiative()` and `autoCommitInitiativeDeletion()` for initiative CRUD |
| `internal/api/handlers_tasks_control.go` | Auto-commit for pause/resume operations |
| `internal/api/handlers_github.go` | Auto-commit for PR merge and status updates |
| `internal/api/handlers_config.go` | Added `autoCommitConfig()` for automation config changes |
| `internal/api/handlers_prompts.go` | Added `autoCommitPrompt()` for prompt save/delete |

### Pattern Used

All auto-commit implementations follow this pattern:
1. Check if `tasks.disable_auto_commit` config is set - skip if true
2. Find project root using `config.FindProjectRoot()`
3. Build commit config with prefix and logger
4. Call appropriate commit function (`task.CommitAndSync`, `task.CommitStatusChange`, `initiative.CommitAndSync`, or direct git commands)
5. Log warnings on failure (non-blocking)

### Test Results

- Build: ✓ Passes
- API tests: ✓ Pass
- Executor tests: ✓ Pass  
- State tests: ✓ Pass
- Git tests: Pre-existing failures (unrelated to changes)

The implementation ensures that `git status` will show clean after ANY orc operation that modifies `.orc/` files, as long as `tasks.disable_auto_commit` is not set.

---
Tokens: 12023999 input, 34117 output, 290212 cache_creation, 11733637 cache_read
Complete: false
Blocked: false
