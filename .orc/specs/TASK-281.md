# Specification: E2E sandbox git sync warnings are noisy

## Problem Statement

E2E sandbox projects created in `/tmp` have no git remote configured, causing repetitive WARN-level log messages during task execution. These warnings appear for every E2E test task and clutter the output, making it harder to spot real issues.

## Success Criteria

- [ ] Projects without a git remote do not emit WARN-level sync messages
- [ ] `git fetch` failures in remoteless projects are logged at DEBUG level with reason "no remote configured"
- [ ] `GetCommitCounts` failures in remoteless projects are logged at DEBUG level, not WARN
- [ ] Completion action sync failures for remoteless projects are logged at DEBUG level
- [ ] Real sync failures (network errors, auth failures) on projects WITH remotes still emit WARN-level logs
- [ ] New `HasRemote(name string)` method added to `Git` struct for checking remote existence
- [ ] Unit tests verify `HasRemote` returns false for repos without remotes
- [ ] Unit tests verify `HasRemote` returns true for repos with remotes
- [ ] E2E tests run without noisy git sync warnings in output

## Testing Requirements

- [ ] Unit test: `TestHasRemote_NoRemote` - verify returns false for git init without remote
- [ ] Unit test: `TestHasRemote_WithRemote` - verify returns true when origin is configured
- [ ] Integration test: Run `orc run` against a remoteless sandbox, verify no WARN messages about fetch/sync failures
- [ ] Existing E2E tests pass without modification (warnings are silent at DEBUG level)

## Scope

### In Scope
- Add `HasRemote(name string) (bool, error)` method to `Git` struct
- Update `syncWithTarget` and `syncOnTaskStart` to check for remote before fetch
- Change log levels from WARN to DEBUG for expected "no remote" scenarios
- Add clear reason string in DEBUG logs explaining why sync was skipped

### Out of Scope
- Adding remote configuration to E2E sandbox (would require mock git server)
- Changing sync behavior for projects that DO have remotes
- Adding config option to disable sync entirely (sync should just be smart about when it can run)
- Modifying global-setup.ts to add fake remotes

## Technical Approach

The core fix is to detect when a project has no remote configured and downgrade sync-related log messages from WARN to DEBUG level. This preserves the diagnostic information while eliminating noise for expected scenarios.

### Implementation Strategy

1. Add a `HasRemote(name string) (bool, error)` method to the `Git` struct that runs `git remote get-url <name>` and returns true if it succeeds
2. In `syncWithTarget` and `syncOnTaskStart`, check `HasRemote("origin")` before attempting fetch
3. If no remote exists:
   - Skip fetch operation entirely (not just log and continue)
   - Log at DEBUG level: "skipping sync: no remote 'origin' configured"
   - Return early from sync function (nothing to sync against)
4. If remote exists but fetch fails, continue logging at WARN level (this is a real problem)

### Files to Modify

| File | Changes |
|------|---------|
| `internal/git/git.go` | Add `HasRemote(name string) (bool, error)` method using `git remote get-url` |
| `internal/executor/pr.go` | Update `syncWithTarget` and `syncOnTaskStart` to check HasRemote before fetch, adjust log levels |
| `internal/git/git_test.go` | Add unit tests for `HasRemote` method |

### Key Code Changes

**internal/git/git.go:**
```go
// HasRemote checks if a remote with the given name is configured.
// Returns true if the remote exists, false otherwise.
func (g *Git) HasRemote(name string) (bool, error) {
    _, err := g.ctx.RunGit("remote", "get-url", name)
    if err != nil {
        // exit status 2 means remote doesn't exist (not an error condition)
        if strings.Contains(err.Error(), "exit status 2") ||
           strings.Contains(err.Error(), "No such remote") {
            return false, nil
        }
        return false, fmt.Errorf("check remote %s: %w", name, err)
    }
    return true, nil
}
```

**internal/executor/pr.go - syncWithTarget:**
```go
// Early check for remote existence
hasRemote, err := gitOps.HasRemote("origin")
if err != nil {
    e.logger.Debug("could not check for remote", "error", err)
    hasRemote = false
}
if !hasRemote {
    e.logger.Debug("skipping sync: no remote 'origin' configured",
        "target", targetBranch,
        "phase", phase)
    return nil
}

// Fetch latest from remote (only if we have one)
if err := gitOps.Fetch("origin"); err != nil {
    e.logger.Warn("fetch failed, continuing anyway", "error", err)
}
```

## Category-Specific Analysis (Chore)

### Current Behavior
- E2E sandbox runs `git init` creating a repo with no remotes
- When orc runs tasks, `syncWithTarget` and `syncOnTaskStart` attempt git operations that fail:
  - `git fetch origin` fails with "fatal: 'origin' does not appear to be a git repository"
  - `git rev-list --count --left-right HEAD...origin/main` fails because origin/main doesn't exist
  - These failures are logged at WARN level
- Warnings appear for every test task, cluttering CI output

### Expected Behavior
- Orc detects that no remote is configured before attempting sync operations
- For remoteless repos, sync is skipped silently (DEBUG level log)
- For repos with remotes, sync failures are still logged at WARN level (real problems)
- E2E test output is clean and focused on actual test results

### Root Cause
The sync functions in `pr.go` don't check whether a remote exists before attempting remote operations. They assume a remote is always present and treat failures as warnings rather than expected conditions.

### Verification
1. Run `make e2e` and verify no WARN messages about fetch/sync failures
2. Run `orc run TASK-XXX` on a real project with remote - verify sync still works
3. Run unit tests for `HasRemote` method
4. Create a local repo without remote, run `orc run` - verify DEBUG message appears and no WARN
