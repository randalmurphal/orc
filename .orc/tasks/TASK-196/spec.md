# Specification: Fix Auto-merge fails when main branch is checked out locally

## Problem Statement

When a task completes and tries to auto-merge via `gh pr merge`, it fails if the target branch (usually `main`) is checked out in the main repository. This is the common case - users typically have main checked out when running orc from a worktree. The error occurs because `gh pr merge` attempts to fast-forward the local main branch after the server-side merge, which fails since git won't checkout a branch already in use by another worktree.

## Success Criteria

- [ ] `orc run TASK-XXX` completes auto-merge successfully when main is checked out in main repo
- [ ] Auto-merge works when running from a worktree with target branch checked out in parent repo
- [ ] No behavior change when target branch is NOT checked out locally (merge still works)
- [ ] Merge method (squash/merge/rebase) is respected
- [ ] Delete branch option (`--delete-branch`) still works
- [ ] Error message is clear if merge fails for other reasons (not worktree conflict)
- [ ] Existing tests continue to pass

## Testing Requirements

- [ ] Unit test: Mock gh API call to verify correct endpoint and parameters
- [ ] Unit test: Verify merge method translates correctly (squash → squash, merge → merge, rebase → rebase)
- [ ] Unit test: Verify delete_branch triggers correct API call after merge
- [ ] Integration test: Verify MergePR succeeds when called (may need mock or skip in CI)
- [ ] Manual test: Run `orc run TASK-XXX` with main checked out locally, verify PR merges

## Scope

### In Scope
- Modify `MergePR` in `internal/executor/ci_merge.go` to use GitHub REST API instead of `gh pr merge` CLI
- Handle merge method parameter translation (squash/merge/rebase)
- Handle branch deletion after merge if configured
- Proper error handling for API failures

### Out of Scope
- Changing how PRs are created (works fine)
- Changing CI polling logic (works fine)
- Supporting merge queues (future enhancement)
- Retrying on merge conflicts (already handled upstream)

## Technical Approach

Use the GitHub REST API directly via `gh api` to merge PRs server-side, bypassing the local git checkout issue entirely.

### API Endpoint
```
PUT /repos/{owner}/{repo}/pulls/{pull_number}/merge
```

### Request Body
```json
{
  "merge_method": "squash|merge|rebase",
  "commit_title": "optional",
  "commit_message": "optional"
}
```

### Response (success)
```json
{
  "sha": "merge_commit_sha",
  "merged": true,
  "message": "Pull Request successfully merged"
}
```

### Branch Deletion
After successful merge, if `delete_branch` is configured, delete the branch via:
```
DELETE /repos/{owner}/{repo}/git/refs/heads/{branch}
```

### Files to Modify

- `internal/executor/ci_merge.go`: Replace `gh pr merge` with `gh api PUT /repos/.../pulls/.../merge`
  - Extract PR number from URL
  - Build JSON payload with merge_method
  - Call `gh api` instead of `gh pr merge`
  - On success, optionally delete branch via API
  - Update task with merge info (already done)

## Category-Specific Analysis (Bug)

### Reproduction Steps
1. Have `main` branch checked out in main repo: `cd ~/repos/orc && git checkout main`
2. Run orc from worktree: `cd .orc/worktrees/orc-TASK-XXX`
3. Complete task execution to reach auto-merge phase
4. Observe error:
   ```
   gh pr merge: exit status 1: failed to run git: fatal: 'main' is already used by worktree at '/home/randy/repos/orc'
   ```

### Current Behavior
`gh pr merge --squash <PR_URL>` attempts to:
1. Merge PR on GitHub (succeeds)
2. Fast-forward local target branch (fails because branch is checked out in another worktree)

### Expected Behavior
PR merges successfully regardless of which branch is checked out locally. The merge is server-side only; local git state doesn't need to change.

### Root Cause
The `gh pr merge` CLI command has a "helpful" feature that tries to update the local copy of the target branch after a successful merge. When running from a worktree while the target branch is checked out in the main repo (common workflow), git refuses to checkout that branch with error "already used by worktree".

### Verification
1. Create a test task in a project with main checked out
2. Run task through to completion with auto-merge enabled
3. Verify PR is merged on GitHub
4. Verify no error about worktree
5. Verify task status shows merged with correct commit SHA
