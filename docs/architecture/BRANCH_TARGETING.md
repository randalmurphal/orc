# Branch Targeting Architecture

Configurable target branches for tasks with 5-level resolution hierarchy, initiative branch support, developer staging, and branch lifecycle management.

## Overview

By default, all task PRs target `main`. This feature adds configurable branch targeting at multiple levels:

1. **Task-level override** - Per-task `target_branch` for hotfixes/special cases
2. **Initiative branches** - Group related tasks under a feature branch
3. **Developer staging** - Personal branch for batching work
4. **Project-level default** - Global default in config
5. **Fallback to main** - Hardcoded default

## Resolution Hierarchy

The `ResolveTargetBranch()` function resolves the target branch:

```go
func ResolveTargetBranch(t *task.Task, init *initiative.Initiative, cfg *config.Config) string {
    // 1. Task explicit override (highest priority)
    if t != nil && t.TargetBranch != "" {
        return t.TargetBranch
    }
    // 2. Initiative branch
    if init != nil && init.BranchBase != "" {
        return init.BranchBase
    }
    // 3. Developer staging (personal config)
    if cfg != nil && cfg.Developer.StagingEnabled && cfg.Developer.StagingBranch != "" {
        return cfg.Developer.StagingBranch
    }
    // 4. Project config
    if cfg != nil && cfg.Completion.TargetBranch != "" {
        return cfg.Completion.TargetBranch
    }
    // 5. Default
    return "main"
}
```

**Location:** `internal/executor/branch.go`

## Configuration

### Task-Level Override

```yaml
# In task definition
target_branch: hotfix/v2.1
```

CLI:
```bash
orc new "Hotfix" --target-branch hotfix/v2.1
orc edit TASK-001 --target-branch release/v3.0
```

### Initiative Branches

```yaml
# In initiative definition
branch_base: feature/user-auth
branch_prefix: feature/auth-  # Optional prefix for task branches
```

CLI:
```bash
orc initiative new "User Auth" --branch-base feature/user-auth
orc initiative edit INIT-001 --branch-base feature/auth
```

When a task belongs to an initiative with `branch_base`:
- Task PRs target the initiative branch
- Task branches can use initiative's `branch_prefix`
- When initiative completes, initiative branch can auto-merge to main

### Developer Staging

```yaml
# In personal config (~/.orc/config.yaml or .orc/local/config.yaml)
developer:
  staging_branch: dev/randy
  staging_enabled: true
  auto_sync_after: 5  # Optional: auto-create PR after N tasks
```

CLI:
```bash
orc staging status   # Show staging branch health
orc staging sync     # Create PR staging → main
orc staging enable   # Enable staging
orc staging disable  # Disable staging
```

### Project-Level Default

```yaml
# .orc/config.yaml
completion:
  target_branch: develop  # Default for all tasks
```

## Branch Creation

Branches are created automatically on first task run:

1. Task starts running
2. `EnsureTargetBranchExists()` checks if target exists
3. If not exists and not a default branch, create from target's base
4. Continue with task execution

**Default branches** (not auto-created): `main`, `master`, `develop`, `development`

**Location:** `internal/executor/setup.go:EnsureTargetBranchExists()`

## Initiative Completion Flow

When all tasks in an initiative complete, the initiative status is updated:

### With BranchBase (Feature Branch)

```
1. Check: All tasks completed?
2. Create PR: feature/user-auth → main
3. Profile-based behavior:
   - auto/fast: Wait for CI, auto-merge
   - safe/strict: Leave PR for human review
4. Update initiative: Status=completed, MergeStatus=merged, MergeCommit
```

**Function:** `CheckAndCompleteInitiative()` in `internal/executor/initiative_completion.go`

### Without BranchBase (Direct to Main)

```
1. Check: All tasks completed?
2. Update initiative: Status=completed
```

**Function:** `CheckAndCompleteInitiativeNoBranch()` in `internal/executor/initiative_completion.go`

Both functions are called automatically when tasks complete (in `WorkflowExecutor.completeTask()`).

### Merge Status

| Status | Description |
|--------|-------------|
| `none` | No merge needed (no branch base) |
| `pending` | Ready for merge, awaiting review |
| `merged` | Successfully merged |
| `failed` | Merge failed |

## Branch Registry

All orc-managed branches are tracked in the database:

### Schema

```sql
CREATE TABLE branches (
    name TEXT PRIMARY KEY,
    type TEXT NOT NULL,          -- 'initiative' | 'staging' | 'task'
    owner_id TEXT,               -- INIT-001, task ID, or developer name
    created_at TIMESTAMP,
    last_activity TIMESTAMP,
    status TEXT DEFAULT 'active' -- 'active' | 'merged' | 'stale' | 'orphaned'
);
```

### Branch Types

| Type | Owner | Purpose |
|------|-------|---------|
| `initiative` | Initiative ID | Feature branch for initiative |
| `staging` | Developer name | Personal staging area |
| `task` | Task ID | Individual task branch |

### Branch Status

| Status | Description |
|--------|-------------|
| `active` | Currently in use |
| `merged` | Merged to target |
| `stale` | No activity for configured period |
| `orphaned` | Owner no longer exists |

### CLI Commands

```bash
orc branches list              # List tracked branches
orc branches list --status stale  # Filter by status
orc branches cleanup           # Delete merged/orphaned
orc branches prune             # Remove stale tracking entries
```

### API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/branches` | List branches (filter: type, status) |
| GET | `/api/branches/{name}` | Get branch details |
| PATCH | `/api/branches/{name}/status` | Update branch status |
| DELETE | `/api/branches/{name}` | Remove from registry |

**Location:** `internal/api/handlers_branches.go`

## Web UI

### Branches Page (`/branches`)

- Status summary cards (active, merged, stale, orphaned)
- Type and status filters
- Branch table with actions (mark stale, delete, etc.)
- Bulk cleanup of merged/orphaned branches

### Initiative Detail

- Branch configuration section (branch_base, branch_prefix)
- Branch status in initiative metadata

### Task Detail

- Target branch display
- Override editor in task edit modal

## Branch Naming

### Task Branches

Without initiative:
```
orc/TASK-001
```

With initiative prefix:
```
feature/auth-TASK-001
```

With worktree isolation:
```
executor-1/feature/auth-TASK-001
```

### Initiative Branches

Created from project's target branch:
```
main → feature/user-auth
develop → feature/user-auth
```

### Developer Staging

Named by convention:
```
dev/{username}
staging/{username}
```

## Stale Detection

Branches are marked stale based on last activity:

```go
func (d *DatabaseBackend) GetStaleBranches(since time.Time) ([]*Branch, error)
```

Default threshold: 30 days (configurable)

## Error Handling

### Branch Creation Failures

- Network errors: Retry with backoff
- Permission errors: Log and fail task
- Already exists: Continue (idempotent)

### Merge Conflicts

Initiative → main merge conflicts:
1. Attempt automatic resolution
2. If failed, mark as `pending` for human review
3. Notify via PR comment

### Orphaned Branch Detection

Run periodically:
1. Load all tracked branches
2. Check if owner (task/initiative) still exists
3. If not, mark as `orphaned`

## Testing

### Unit Tests

**Branch resolution:** `internal/executor/branch_test.go`
- All 5 levels of hierarchy
- Source identification
- Default branch detection

**Storage:** `internal/storage/database_backend_test.go`
- Save/Load/Update/Delete operations
- List with filters
- Stale detection

### Integration Tests

**Initiative completion:** Verify auto-merge flow
**Branch creation:** Verify lazy creation on first run

## Related Documentation

- [Git Integration](GIT_INTEGRATION.md) - Worktree and sync strategies
- [Executor](EXECUTOR.md) - Task execution flow
- [Task Model](TASK_MODEL.md) - Task fields including target_branch
