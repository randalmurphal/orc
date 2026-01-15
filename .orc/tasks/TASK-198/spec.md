# Specification: Fix: Re-running completed tasks fails to push due to diverged remote branch

## Problem Statement

When a task is re-run after previously completing (which includes pushing to remote), the completion action fails with 'non-fast-forward' error because the local branch has been reset to a fresh state based on the target branch, but the remote still has the old commits from the previous run. This requires manual force push to resolve.

## Root Cause Analysis

The issue occurs in this sequence:
1. Task TASK-001 runs successfully, branch `orc/TASK-001` is pushed to origin
2. User wants to re-run the task (e.g., to incorporate review feedback)
3. Worktree is recreated from the target branch (main), getting a fresh start
4. Task executes, creating new commits
5. At completion, `Push()` in `internal/git/git.go:385` calls `devflow.Push()`
6. Push fails because local history diverges from `origin/orc/TASK-001`

The current `Push()` implementation (`internal/git/git.go:385-390`) wraps `devflow.Push()` which uses standard `git push` without force:
```go
func (g *Git) Push(remote, branch string, setUpstream bool) error {
    if IsProtectedBranch(branch, g.protectedBranches) {
        return fmt.Errorf("%w: cannot push to '%s'", ErrProtectedBranch, branch)
    }
    return g.ctx.Push(remote, branch, setUpstream)
}
```

## Design Decision: Sync with Remote Before Reset

After analysis, the best approach is **Option 3: Sync with remote before starting fresh**. Rather than force-pushing (destructive) or deleting remote branches (loses history), we should:

1. When re-running a completed task, detect if the remote branch exists
2. If it does, rebase the task branch onto the remote before resetting/restarting
3. This preserves remote history while allowing fresh work

However, for simpler cases where a user explicitly wants to re-run from scratch (discarding all previous work), we need a **force push with safeguards** approach as a fallback.

## Success Criteria

- [ ] Re-running a completed task succeeds without manual intervention
- [ ] Force push is only used for task branches (never for protected branches like main/master)
- [ ] User is warned when force push will occur (in non-quiet mode)
- [ ] Existing behavior for first-time runs is unchanged (no regression)
- [ ] The solution handles both scenarios:
  - [ ] Scenario A: Re-run keeping remote commits (default) - sync with remote first
  - [ ] Scenario B: Re-run from scratch (--fresh flag) - force push to overwrite
- [ ] Configuration option to control force push behavior (`completion.push.allow_force`)
- [ ] Push failure logs clear message explaining why it failed and how to resolve

## Testing Requirements

- [ ] Unit test: `TestPushForce_TaskBranch` - verify force push works for task branches
- [ ] Unit test: `TestPushForce_ProtectedBranch` - verify force push blocked for protected branches
- [ ] Unit test: `TestDetectRemoteBranchExists` - verify remote branch detection
- [ ] Unit test: `TestSyncWithRemoteBranch` - verify rebase onto remote before re-run
- [ ] Integration test: Re-run completed task without --fresh flag syncs with remote
- [ ] Integration test: Re-run completed task with --fresh flag force pushes

## Scope

### In Scope
- Add `PushForce()` method to Git wrapper with protection checks
- Add remote branch detection helper (`RemoteBranchExists`)
- Modify completion push logic to handle diverged branches
- Add `--fresh` flag to `orc run` for explicit force-push behavior
- Add configuration option `completion.push.allow_force`
- Update error messages to guide users on resolution

### Out of Scope
- Changing the worktree creation logic (keep current behavior)
- Automatic cleanup of remote branches after merge
- Multi-remote support (origin only for now)
- Interactive rebase for conflict resolution (fails and prompts user)

## Technical Approach

### 1. Add Git Helper Methods

**File: `internal/git/git.go`**

```go
// PushForce pushes with --force-with-lease for safety.
// This is safer than --force as it fails if the remote has unexpected commits.
// SAFETY: This will NOT push to protected branches.
func (g *Git) PushForce(remote, branch string, setUpstream bool) error {
    if IsProtectedBranch(branch, g.protectedBranches) {
        return fmt.Errorf("%w: cannot force push to '%s'", ErrProtectedBranch, branch)
    }
    // Use --force-with-lease instead of --force for extra safety
    args := []string{"push", "--force-with-lease"}
    if setUpstream {
        args = append(args, "-u")
    }
    args = append(args, remote, branch)
    _, err := g.ctx.RunGit(args...)
    return err
}

// RemoteBranchExists checks if a branch exists on the remote.
func (g *Git) RemoteBranchExists(remote, branch string) (bool, error) {
    _, err := g.ctx.RunGit("ls-remote", "--heads", remote, branch)
    // Parse output to determine if branch exists
    // ...
}

// SyncWithRemoteBranch rebases the current branch onto the remote branch.
// Used when re-running a task to preserve remote history.
func (g *Git) SyncWithRemoteBranch(remote, branch string) error {
    // Fetch the remote branch first
    // Then rebase onto it
}
```

### 2. Modify Completion Push Logic

**File: `internal/executor/pr.go`**

In `createPR()` around line 396:

```go
// Push task branch to remote
err := gitOps.Push("origin", taskBranch, true)
if err != nil {
    // Check if this is a non-fast-forward error
    if isNonFastForwardError(err) {
        // Option 1: Try to sync with remote (rebase)
        // Option 2: Force push if configured/flagged
        if e.orcConfig.Completion.Push.AllowForce || e.forceRerun {
            e.logger.Warn("remote branch has diverged, force pushing",
                "branch", taskBranch,
                "reason", "re-run of completed task")
            if err := gitOps.PushForce("origin", taskBranch, true); err != nil {
                return fmt.Errorf("force push branch: %w", err)
            }
        } else {
            return fmt.Errorf("push branch: %w\n"+
                "  The remote branch has diverged (likely from a previous run).\n"+
                "  Resolution options:\n"+
                "    1. Run with --fresh to force push and overwrite remote\n"+
                "    2. Manually: git push --force-with-lease origin %s\n"+
                "    3. Set completion.push.allow_force: true in config",
                err, taskBranch)
        }
    } else {
        return fmt.Errorf("push branch: %w", err)
    }
}
```

### 3. Add Configuration

**File: `internal/config/config.go`**

Add to `CompletionConfig`:
```go
type PushConfig struct {
    AllowForce bool `yaml:"allow_force"` // Allow force push for task branches
}

type CompletionConfig struct {
    // ... existing fields
    Push PushConfig `yaml:"push"`
}
```

### 4. Add CLI Flag

**File: `internal/cli/cmd_run.go`**

```go
cmd.Flags().Bool("fresh", false, "force re-run from scratch (overwrites remote branch)")
```

### Files to Modify

| File | Changes |
|------|---------|
| `internal/git/git.go` | Add `PushForce()`, `RemoteBranchExists()`, `SyncWithRemoteBranch()` |
| `internal/git/git_test.go` | Add tests for new methods |
| `internal/executor/pr.go` | Handle non-fast-forward push errors |
| `internal/config/config.go` | Add `completion.push.allow_force` config |
| `internal/config/defaults.go` | Set default for allow_force (false) |
| `internal/cli/cmd_run.go` | Add `--fresh` flag |
| `docs/specs/CONFIG_HIERARCHY.md` | Document new config option |

## Bug Analysis

### Reproduction Steps
1. Create and run a task to completion: `orc new "Test task" && orc run TASK-001`
2. Wait for completion (PR created, branch pushed)
3. Reset task status: `orc edit TASK-001 --status pending`
4. Re-run the task: `orc run TASK-001`
5. Task executes but fails at push with "non-fast-forward" error

### Current Behavior
Push fails with error:
```
error: failed to push some refs to 'origin'
hint: Updates were rejected because the tip of your current branch is behind
hint: its remote counterpart. If you want to integrate the remote changes,
hint: use 'git pull' before pushing again.
```

### Expected Behavior
Re-running a task should:
1. Default: Sync with remote branch to preserve history, then continue
2. With `--fresh`: Force push to overwrite remote (with clear warning)

### Root Cause
The worktree is recreated fresh from the target branch (main), discarding the previous run's commits. When pushing, the local branch has different ancestry than the remote branch, causing a non-fast-forward error.

### Verification
1. Re-run completed task without --fresh: syncs with remote, pushes successfully
2. Re-run completed task with --fresh: force pushes with warning
3. Protected branches still cannot be force pushed (verify error)
4. First-time runs work as before (no regression)
