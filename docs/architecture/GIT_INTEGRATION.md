# Git Integration

**Purpose**: Git is the checkpoint system - branches for isolation, commits for checkpoints, worktrees for parallelism.

---

## Branch Strategy

```
main
├── orc/TASK-001              # Task branch
│   ├── [orc] classify: complete
│   ├── [orc] spec: complete
│   └── [orc] implement: iteration 3
├── orc/TASK-002              # Another task
└── orc/TASK-001/fork-1       # Fork for alternative approach
```

### Branch Naming

| Pattern | Example | Purpose |
|---------|---------|---------|
| `orc/TASK-XXX` | `orc/TASK-001` | Primary task branch |
| `orc/TASK-XXX/fork-N` | `orc/TASK-001/fork-1` | Alternative approach |

---

## Checkpoint Commits

### Commit Message Format

```
[orc] TASK-ID: phase - status

Phase: phase-name
Status: completed|failed|paused
Iteration: N
Duration: Xm Ys

Files changed:
- path/to/file.go
```

### Example

```
[orc] TASK-001: implement - iteration 3

Phase: implement
Status: running
Iteration: 3
Duration: 5m 32s

Files changed:
- src/auth/login.go
- src/auth/login_test.go
```

---

## Worktree Strategy

Parallel task execution via git worktrees:

```
project/                      # Main working directory
├── .orc/
│   └── worktrees/
│       ├── TASK-001/        # Worktree for task 1
│       └── TASK-002/        # Worktree for task 2
└── ...
```

### Creating Worktrees

```go
func CreateWorktree(taskID string) (string, error) {
    branch := fmt.Sprintf("orc/%s", taskID)
    path := fmt.Sprintf(".orc/worktrees/%s", taskID)
    
    // Create worktree
    cmd := exec.Command("git", "worktree", "add", path, branch)
    return path, cmd.Run()
}
```

### Benefits

- Each task has isolated working directory
- No `git stash` when switching tasks
- Claude processes can't interfere
- Easy cleanup: delete directory

### Stale Worktree Handling

Git tracks worktrees in its internal state (`.git/worktrees/`). If a worktree directory is deleted without using `git worktree remove` (e.g., `rm -rf`), git retains a "stale" registration that blocks creating a new worktree at the same path.

**Orc handles this automatically**:
1. First attempts to create worktree normally
2. If that fails, tries to add worktree for existing branch
3. If both fail, prunes stale entries (`git worktree prune`)
4. Retries worktree creation after pruning

This means users can safely delete worktree directories manually without breaking future task execution.

```go
// PruneWorktrees can also be called manually if needed
func (g *Git) PruneWorktrees() error
```

### Worktree State Cleanup on Resume

When a task fails during execution, the worktree may be left in a problematic git state:
- Rebase in progress (interrupted sync operation)
- Merge in progress (interrupted conflict detection)
- Uncommitted changes or conflict markers

If the user then tries to resume the task, these states would block execution with errors like "rebase already in progress" or "you have unstaged changes".

**Orc handles this automatically** via `cleanWorktreeState()`:

1. **Check for rebase**: If `.git/rebase-merge/` or `.git/rebase-apply/` exists, abort it
2. **Check for merge**: If `.git/MERGE_HEAD` exists, abort it
3. **Check for dirty state**: If working directory is not clean, discard all changes

```go
// SetupWorktreeForTask automatically cleans up when reusing an existing worktree
func SetupWorktreeForTask(t *orcv1.Task, cfg *config.Config, gitOps *git.Git, backend storage.Backend) (*WorktreeSetup, error) {
    worktreePath := gitOps.WorktreePath(t.ID)
    if _, err := os.Stat(worktreePath); err == nil {
        // Worktree exists - clean up any problematic state
        if err := cleanWorktreeState(worktreePath, gitOps, expectedBranch); err != nil {
            return nil, err
        }
        return &WorktreeSetup{Path: worktreePath, Reused: true, TargetBranch: targetBranch}, nil
    }
    // ... create new worktree
}
```

**Available git methods for state detection**:

| Method | Purpose |
|--------|---------|
| `IsRebaseInProgress()` | Check for rebase-merge or rebase-apply directories |
| `IsMergeInProgress()` | Check for MERGE_HEAD file |
| `IsClean()` | Check for uncommitted changes |
| `AbortRebase()` | Run `git rebase --abort` |
| `AbortMerge()` | Run `git merge --abort` |
| `DiscardChanges()` | Reset staged, checkout tracked, clean untracked |

This ensures users can safely resume failed tasks without manual worktree cleanup.

---

## Thread Safety

The `Git` struct includes a mutex to protect compound operations that must be atomic. This enables safe concurrent use from multiple goroutines (e.g., parallel task execution, API handlers).

### Mutex-Protected Operations

| Method | Why Protected |
|--------|---------------|
| `tryCreateWorktree` | Prune + retry must be atomic |
| `CreateCheckpoint` | Stage + commit must be atomic |
| `detectConflictsViaMerge` | Merge + diff + abort + reset must be atomic |
| `RebaseWithConflictCheck` | Rebase + diff + abort must be atomic |
| `RestoreOrcDir` | Diff + checkout + add + commit must be atomic |
| `RestoreClaudeSettings` | Diff + checkout + add + commit must be atomic |

### Design Principles

1. **Individual git commands don't need locking** - they're atomic at the process level
2. **Compound operations need protection** - operations with cleanup/abort steps that must complete together
3. **Each Git instance has its own mutex** - worktree instances don't contend with parent

### Example: Checkpoint Race Condition (Fixed)

Without mutex, parallel checkpoint creation could interleave:

```
Goroutine A: git add -A         (stages files)
Goroutine B: git add -A         (stages different files)
Goroutine A: git commit         (commits B's staged files too!)
Goroutine B: git commit         (nothing to commit - error)
```

With mutex, each checkpoint operation completes atomically.

### Worktree Instance Independence

When using `InWorktree()`, the returned instance gets a new (unlocked) mutex:

```go
// Main repo Git instance
mainGit := git.New(...)

// Worktree instance - independent mutex
worktreeGit := mainGit.InWorktree(".orc/worktrees/TASK-001")

// These don't contend - different directories, different mutexes
go mainGit.CreateCheckpoint(...)     // Uses mainGit.mu
go worktreeGit.CreateCheckpoint(...) // Uses worktreeGit.mu
```

This is correct because worktrees operate on different directories and can safely run in parallel.

---

## Operations

### Create Task Branch

```go
func CreateTaskBranch(taskID string) error {
    branch := fmt.Sprintf("orc/%s", taskID)
    return exec.Command("git", "checkout", "-b", branch).Run()
}
```

### Create Checkpoint

```go
func Checkpoint(task *Task, phase string, message string) error {
    worktree := GetWorktreePath(task.ID)
    
    // Stage all changes
    exec.Command("git", "-C", worktree, "add", "-A").Run()
    
    // Create commit
    commitMsg := FormatCheckpointMessage(task, phase, message)
    return exec.Command("git", "-C", worktree, "commit", "-m", commitMsg).Run()
}
```

### Rewind to Checkpoint

```go
func Rewind(taskID, commitRef string) error {
    worktree := GetWorktreePath(taskID)
    
    // Hard reset to checkpoint
    err := exec.Command("git", "-C", worktree, "reset", "--hard", commitRef).Run()
    if err != nil {
        return err
    }
    
    // Reload task state
    return ReloadTaskState(taskID)
}
```

### Fork from Checkpoint

```go
func Fork(taskID, newTaskID, commitRef string) error {
    newBranch := fmt.Sprintf("orc/%s", newTaskID)
    
    // Create new branch from commit
    exec.Command("git", "branch", newBranch, commitRef).Run()
    
    // Create worktree
    CreateWorktree(newTaskID)
    
    // Copy and update task state
    return CopyTaskState(taskID, newTaskID)
}
```

---

## Branch Synchronization

Parallel tasks can diverge from the target branch, causing merge conflicts at completion. Orc automatically syncs task branches with the target branch to detect and resolve conflicts early.

### Sync Strategies

| Strategy | When Sync Happens | Use Case |
|----------|------------------|----------|
| `none` | Never | Manual sync only, full control |
| `phase` | Before each phase starts | Maximum conflict detection, slight overhead |
| `completion` | Before PR/merge (default) | Balance of safety and efficiency |
| `detect` | At completion, detection only | Fail-fast without auto-resolution |

### Configuration

```yaml
# .orc/config.yaml
completion:
  sync:
    strategy: completion     # none | phase | completion | detect
    sync_on_start: true      # Sync before execution starts (default: true)
    fail_on_conflict: true   # Abort on conflicts (default: true)
    max_conflict_files: 0    # Max conflict files before abort (0 = unlimited)
    skip_for_weights:        # Skip sync for trivial tasks
      - trivial
```

### Sync on Start (Parallel Task Fix)

When `sync_on_start: true` (default), orc syncs the task branch with the target branch **before execution begins**. This catches conflicts from parallel tasks early:

```
Timeline:
1. TASK-A and TASK-B both start from main@SHA1
2. TASK-A completes and merges → main@SHA2
3. TASK-B starts execution:
   - sync_on_start=true: rebases onto main@SHA2, incorporates TASK-A changes
   - sync_on_start=false: stays on stale SHA1, conflicts at completion
```

**Benefits:**
- Implement phase sees latest code including parallel task changes
- AI can incorporate those changes during implementation
- Fewer/no conflicts at completion sync

**Disable if:**
- You want to isolate your task from concurrent changes
- You're intentionally working on an older branch state

### Conflict Handling

When conflicts are detected:

1. **fail_on_conflict: true** (default) — Task is marked as blocked with detailed resolution guidance
2. **fail_on_conflict: false** — Warning logged, PR created (may have merge conflicts)

**Enhanced Blocked Task Output**: When a task is blocked by sync conflicts, orc provides:
- Worktree path for quick navigation
- List of conflicted files
- Step-by-step resolution commands (contextual for rebase vs merge strategy)
- Verification command to confirm resolution
- Exact resume command

See [Troubleshooting: Task Blocked After Sync Conflict](../guides/TROUBLESHOOTING.md#task-blocked-after-sync-conflict) for example output and resolution workflow.

### Sync Process

```
1. Fetch latest from origin
2. Check commits behind target branch
3. If strategy is 'detect':
   - Use git merge-tree to detect conflicts without modifying working tree
   - Fail if conflicts found
4. If strategy is 'phase' or 'completion':
   - Attempt rebase onto target
   - On conflict: abort rebase, report conflicting files, fail task
   - On success: continue with synced branch
```

### Why Sync Matters

Without sync, parallel tasks can diverge significantly:

```
main:     A → B → C → D → E (other tasks merged)
task-001: A → X → Y        (started from A, unaware of B-E)
task-002: A → Z            (also started from A)
```

When task-001 completes and creates a PR, it may conflict with changes in B-E. With sync enabled:

```
task-001 (after sync): A → B → C → D → E → X' → Y'
```

The task rebases onto the latest target, catching conflicts before PR creation.

---

## CLAUDE.md Auto-Merge

When running multiple tasks in parallel, conflicts in `CLAUDE.md`'s knowledge section are common since each task may add entries to the same tables. Orc provides automatic conflict resolution for these predictable, append-only conflicts.

### How It Works

During git sync operations (finalize phase or completion sync), orc detects conflicts in `CLAUDE.md` and attempts automatic resolution:

1. **Detection**: Conflict must be within the `<!-- orc:knowledge:begin -->` / `<!-- orc:knowledge:end -->` markers
2. **Analysis**: Each conflict block is analyzed to determine if it's purely additive (both sides add new rows)
3. **Resolution**: If purely additive, rows from both sides are combined and sorted by source ID (TASK-XXX)
4. **Fallback**: Complex conflicts (overlapping edits, malformed tables) fall back to manual resolution

### Supported Tables

Auto-merge works for append-only tables in the knowledge section:

| Table | Marker Detected By |
|-------|-------------------|
| Patterns Learned | `### Patterns Learned` heading |
| Known Gotchas | `### Known Gotchas` heading |
| Decisions | `### Decisions` heading |

### Resolution Rules

| Scenario | Action |
|----------|--------|
| Both sides add new rows | ✓ Auto-merge, combine rows |
| Same row edited differently | ✗ Manual resolution required |
| Conflict outside knowledge section | ✗ Manual resolution required |
| Malformed table syntax | ✗ Manual resolution required |
| Same TASK-XXX on both sides | ✓ Auto-merge (different patterns allowed) |

### Logging

Auto-resolution is logged for auditability:

```
INFO CLAUDE.md auto-merge successful tables_merged=1
```

If auto-resolution fails, detailed logs explain why:

```
WARN CLAUDE.md conflict cannot be auto-resolved: Table 'Patterns Learned': conflict is not purely additive
```

### Configuration

CLAUDE.md auto-merge is always enabled during git sync operations. There is no configuration to disable it - if auto-resolution fails, manual resolution is required as normal.

---

## Finalize Phase Sync

The finalize phase provides advanced sync capabilities beyond basic completion sync, including AI-assisted conflict resolution.

### Finalize Sync Strategies

| Strategy | Behavior | Result |
|----------|----------|--------|
| `merge` (default) | Merge target into task branch | Preserves full history, creates merge commit |
| `rebase` | Rebase task onto target | Linear history, may require more conflict resolution |

### AI-Assisted Conflict Resolution

When conflicts are detected during finalize, Claude is invoked to resolve them:

```
1. Detect conflicts via git merge/rebase
2. List conflicted files
3. For each file:
   a. Claude reads both sides of conflict
   b. Applies merge rules (never remove features, merge intentions)
   c. Resolves conflict preserving both changes
   d. Stages resolved file
4. Complete merge/rebase
5. Re-run tests to verify resolution
```

**Conflict Resolution Rules:**
- Never take "ours" or "theirs" blindly
- Both upstream AND task changes must be preserved
- Merge intentions, not just text
- Prefer additive resolutions

### Finalize Configuration

```yaml
completion:
  finalize:
    enabled: true
    sync:
      strategy: merge      # merge | rebase
    conflict_resolution:
      enabled: true        # AI conflict resolution
      instructions: ""     # Additional resolution guidance
    risk_assessment:
      enabled: true
      re_review_threshold: high
```

### Escalation

When finalize can't resolve issues, it escalates back to the implement phase:
- >10 unresolved conflicts
- >5 test failures after fix attempts
- Complex conflicts requiring manual intervention

The implement phase receives full context about what failed.

---

## Merge Strategy

### Squash Merge (Default)

Task branch squashes to single commit on main:

```bash
git checkout main
git merge --squash orc/TASK-001
git commit -m "feat: Add user authentication (#TASK-001)"
```

### Preserve History (Optional)

```yaml
# orc.yaml
git:
  merge_strategy: preserve  # squash (default) | preserve | rebase
```

---

## Diverged Branch Handling (Re-runs)

When re-running a task that was previously completed and pushed, the remote branch has different history from the fresh local branch. Orc automatically handles this with safe force pushing.

### The Problem

```
Timeline:
1. TASK-001 runs, commits A→B→C, pushes to origin/orc/TASK-001
2. User re-runs TASK-001 (e.g., to incorporate feedback)
3. Worktree recreated fresh from main, new commits X→Y
4. Push fails: "non-fast-forward" - local (X→Y) diverges from remote (A→B→C)
```

### The Solution

Orc detects non-fast-forward push errors and automatically retries with `--force-with-lease`:

```go
if err := gitOps.Push("origin", taskBranch, true); err != nil {
    if isNonFastForwardError(err) {
        // Safe force push - fails if remote has unexpected commits
        return gitOps.PushForce("origin", taskBranch, true)
    }
    return err
}
```

### Why `--force-with-lease`?

| Option | Behavior | Safety |
|--------|----------|--------|
| `--force` | Overwrites remote unconditionally | Dangerous - may lose others' work |
| `--force-with-lease` | Overwrites only if remote matches expected state | Safe - fails if remote was updated |

If another developer pushed to the same branch after your last fetch, `--force-with-lease` will fail rather than overwrite their work. This is the right behavior - you should fetch and review their changes first.

### Protected Branches

Force push is **never allowed** on protected branches. The `PushForce()` method checks against the protected branches list:

```go
func (g *Git) PushForce(remote, branch string, setUpstream bool) error {
    if IsProtectedBranch(branch, g.protectedBranches) {
        return fmt.Errorf("%w: cannot force push to '%s'", ErrProtectedBranch, branch)
    }
    // ... proceed with --force-with-lease
}
```

Default protected branches: `main`, `master`, `develop`, `release/*`

### Logging

When a diverged branch triggers force push, orc logs a warning:

```
WARN remote branch has diverged, force pushing branch=orc/TASK-001 reason=re-run of completed task
```

This provides visibility without interrupting the automated workflow.

---

## CI Wait and Auto-Merge

After the finalize phase completes, orc can automatically wait for CI checks to pass and then merge the PR. This provides a complete automation flow without requiring hosting provider auto-merge features (which require branch protection).

### Flow After Finalize

```
finalize completes → push changes → poll CI → merge PR via API → cleanup
```

1. **Push finalize changes**: Any commits from conflict resolution or sync
2. **Poll CI checks**: Wait for all required checks to pass
3. **Merge PR via API**: Use hosting provider API directly (GitHub REST API or GitLab API, avoids worktree conflicts)
4. **Cleanup**: Delete branch via API if configured

### Why API Instead of CLI?

CLI merge commands (e.g., `gh pr merge`) often try to fast-forward the local target branch after a server-side merge. When running from a worktree while the target branch (e.g., `main`) is checked out in the main repo (the common workflow), git refuses with:

```
fatal: 'main' is already used by worktree at '/path/to/main/repo'
```

By using the hosting provider API directly via the Provider interface, we merge server-side only without any local git operations. This works regardless of which branch is checked out locally and returns the merge commit SHA directly from the response.

### API Endpoints Used

| Operation | GitHub | GitLab |
|-----------|--------|--------|
| Merge PR | PUT /repos/{owner}/{repo}/pulls/{number}/merge | PUT /projects/{id}/merge_requests/{iid}/merge |
| Delete branch | DELETE /repos/{owner}/{repo}/git/refs/heads/{branch} | DELETE /projects/{id}/repository/branches/{branch} |
| Enable auto-merge | Requires GraphQL (not supported) | Accept MR with merge_when_pipeline_succeeds |
| Update branch | POST /repos/{owner}/{repo}/pulls/{number}/update-branch | POST /projects/{id}/merge_requests/{iid}/rebase |

### CI Polling

The CI merger uses the Provider interface's `GetCheckRuns()` method to poll status.

| Bucket | Meaning |
|--------|---------|
| `pass` | Check succeeded |
| `fail` | Check failed |
| `pending` | Check still running |
| `skipping` | Check was skipped (treated as pass) |
| `cancel` | Check was cancelled (treated as fail) |

### Configuration

```yaml
completion:
  ci:
    wait_for_ci: false              # Enable CI polling (default: false)
    ci_timeout: 10m                 # Max wait time (default: 10m)
    poll_interval: 30s              # Polling frequency (default: 30s)
    merge_on_ci_pass: false         # Auto-merge when CI passes (default: false)
    merge_method: squash            # squash | merge | rebase (default: squash)
    verify_sha_on_merge: true       # Verify HEAD SHA before merge (default: true)
  delete_branch: true               # Delete branch after merge (default: true)
  merge_commit_template: ""         # Custom merge commit message template
  squash_commit_template: ""        # Custom squash commit message template
```

### Profile Restrictions

CI wait and auto-merge default to OFF for all profiles. Users must explicitly enable them via configuration (`wait_for_ci: true` and `merge_on_ci_pass: true`).

| Profile | CI Wait | Auto-Merge | Notes |
|---------|---------|------------|-------|
| `auto` | Off (opt-in) | Off (opt-in) | Enable via config for full automation |
| `fast` | Off (opt-in) | Off (opt-in) | Enable via config for full automation |
| `safe` | Off (opt-in) | Off (opt-in) | Human review still required before merge |
| `strict` | Off (opt-in) | Off (opt-in) | Human gates required throughout |

When enabled, `safe` and `strict` profiles still require human approval before the merge executes. The CI wait simply polls for check status, and auto-merge only triggers after all configured gates have been satisfied.

---

## Cleanup

After task completion:

```bash
# Remove worktree
git worktree remove .orc/worktrees/TASK-001

# Delete branch
git branch -d orc/TASK-001

# Prune worktree refs
git worktree prune
```

Automated via `orc cleanup`:

```bash
orc cleanup                    # Remove completed task branches
orc cleanup --all              # Remove all task branches
orc cleanup --older-than 7d    # Remove branches older than 7 days
```

---

## .gitignore

```gitignore
# Orc worktrees (ephemeral)
.orc/worktrees/

# Orc cache (regenerable)
.orc/cache/
```

**Tracked**: `.orc/tasks/`, `.orc/config.yaml`, `.orc/prompts/`

---

## Auto-Commit for .orc/ Files

All mutations to `.orc/` files are automatically committed to git, ensuring `git status` is always clean after any orc operation.

### Covered Operations

| Category | Operations | Commit Messages |
|----------|-----------|-----------------|
| **Task Lifecycle** | status changes (running, completed, failed), phase transitions, retry context, token tracking | `[orc] task TASK-001: running`, `[orc] task TASK-001: implement phase completed` |
| **Task CRUD** | create, update, delete | `[orc] task TASK-001: created - Title`, `[orc] task TASK-001: updated`, `[orc] task TASK-001: deleted` |
| **Initiative Operations** | create, update, delete, status changes, task linking/unlinking, decisions | `[orc] initiative INIT-001: created`, `[orc] initiative INIT-001: task TASK-002 added` |
| **Config Changes** | config updates via API/UI | `[orc] config: automation settings updated` |
| **Prompt Overrides** | prompt create, update, delete | `[orc] prompt: implement updated` |
| **PR Status** | PR polling updates | `[orc] task TASK-001: PR status updated` |
| **Finalize** | finalize phase completion | `[orc] task TASK-001: finalize completed` |

### Implementation

**Executor (internal/executor):**
- `commitTaskState(t, action)` - commits task state changes during execution
- `commitTaskStatus(t, status)` - convenience wrapper for status changes

**API Handlers (internal/api):**
- `autoCommitTask(t, action)` - commits task changes
- `autoCommitTaskDeletion(taskID)` - commits task deletions
- `autoCommitInitiative(init, action)` - commits initiative changes
- `autoCommitConfig(description)` - commits config changes
- `autoCommitPrompt(phase, action)` - commits prompt changes

**State Package (internal/state):**
- `CommitTaskState(taskID, action, cfg)` - stages and commits task directory
- `CommitPhaseTransition(taskID, phase, transition, cfg)` - convenience for phase events
- `CommitExecutionState(taskID, description, cfg)` - convenience for execution events

### Configuration

```yaml
# .orc/config.yaml
tasks:
  disable_auto_commit: false  # Set to true to disable all auto-commits
```

When disabled:
- File saves still occur normally
- No git commits are created
- Manual commits required to track changes
- Useful for development/debugging scenarios

### Behavior

- **Non-blocking**: Failed commits log a warning but don't fail the operation
- **Idempotent**: "Nothing to commit" states are handled gracefully
- **Project-root aware**: Always commits to main repo, even from worktrees
- **Prefix configurable**: Uses `commit_prefix` config (default: `[orc]`)
