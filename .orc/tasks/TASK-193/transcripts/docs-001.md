# docs - Iteration 1

## Prompt

Update documentation for:

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

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

The documentation has been updated to reflect the comprehensive auto-commit feature from TASK-193. Here's a summary of what was documented:

## Documentation Updates

### CLAUDE.md (Root)
- Updated `tasks.disable_auto_commit` config description to reflect that it covers ALL .orc/ file mutations (not just task create/edit)
- Added new knowledge pattern entry for TASK-193 documenting the comprehensive auto-commit behavior

### docs/architecture/GIT_INTEGRATION.md
Added a new section "Auto-Commit for .orc/ Files" covering:
- **Covered Operations** table: Task lifecycle, Task CRUD, Initiative operations, Config changes, Prompt overrides, PR status, Finalize
- **Implementation** details: Executor helpers, API handlers, State package functions
- **Configuration**: The `tasks.disable_auto_commit` option
- **Behavior**: Non-blocking, idempotent, project-root aware, prefix configurable

### docs/specs/CONFIG_HIERARCHY.md
- Added `tasks.disable_auto_commit` to the config schema
- Added `tasks` to the config categories table

### internal/CLAUDE.md
- Updated state package description to include `CommitTaskState` and `CommitPhaseTransition` types

### internal/api/CLAUDE.md
- Added new "Auto-Commit Helpers" section documenting all `autoCommit*()` functions and when they're used

### internal/executor/CLAUDE.md
- Added executor.go to support modules table for auto-commit helpers
- Added new "Auto-Commit Helpers" section documenting `commitTaskState()` and `commitTaskStatus()`

<phase_complete>true</phase_complete>

---
Tokens: 2511763 input, 7921 output, 77397 cache_creation, 2432427 cache_read
Complete: true
Blocked: false
