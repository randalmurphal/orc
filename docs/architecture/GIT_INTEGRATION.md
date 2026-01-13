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
    fail_on_conflict: true   # Abort on conflicts (default: true)
    max_conflict_files: 0    # Max conflict files before abort (0 = unlimited)
    skip_for_weights:        # Skip sync for trivial tasks
      - trivial
```

### Conflict Handling

When conflicts are detected:

1. **fail_on_conflict: true** (default) — Task fails with clear error message listing conflicting files and resolution options
2. **fail_on_conflict: false** — Warning logged, PR created (may have merge conflicts)

Resolution options on conflict:
- Manually resolve conflicts and retry task
- Rebase your changes onto latest target branch
- Set `fail_on_conflict: false` to allow PR with conflicts

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
