# implement - Iteration 1

## Prompt

Implement the following feature/task:

**Task**: Complete auto-merge flow: wait for CI then merge after finalize

**Description**: After finalize phase completes successfully:
1. Push any finalize changes (conflict resolution, sync commits)
2. Poll gh pr checks until all CI checks pass (with timeout)
3. Merge PR directly with gh pr merge --squash

This removes dependency on GitHub's auto-merge feature (requires branch protection) and self-approval (blocked by GitHub). The gate becomes: finalize passes + CI passes = safe to merge.

Config options:
- completion.wait_for_ci: true (default) - wait for checks before merge
- completion.ci_timeout: 10m - max time to wait for CI
- completion.merge_on_ci_pass: true - auto-merge when CI passes



1. Plan your implementation approach
2. Implement the required changes
3. Write/update tests alongside code
4. Run tests and fix any failures

Keep iterating until implementation is complete and tests pass.

When done, output:
<phase_complete>true</phase_complete>


## Response

## Summary

I've implemented the complete auto-merge flow that waits for CI and merges after finalize. Here's what was added:

### 1. Config Changes (`internal/config/config.go`)

**New fields in `CompletionConfig`:**
- `WaitForCI bool` - Wait for CI checks to pass before merging (default: true)
- `CITimeout time.Duration` - Max time to wait for CI (default: 10 minutes)
- `MergeOnCIPass bool` - Auto-merge when CI passes (default: true)

**New helper methods:**
- `ShouldWaitForCI()` - Returns true for auto/fast profiles when WaitForCI is enabled
- `ShouldMergeOnCIPass()` - Returns true when both WaitForCI and MergeOnCIPass are enabled
- `GetCITimeout()` - Returns configured timeout or 10m default

### 2. CI Merger (`internal/executor/ci_merge.go`)

New `CIMerger` type that handles:
- Pushing finalize changes with `--force-with-lease`
- Polling CI checks via `gh pr checks --json`
- Parsing check status (pass/fail/pending)
- Merging PR with `gh pr merge --squash --delete-branch`
- Handling edge cases (already merged, not mergeable, timeout)

### 3. Finalize Integration (`internal/api/handlers_finalize.go`)

**Extended `FinalizeResult` with:**
- `CIPassed bool` - CI checks passed
- `CIDetails string` - CI status summary
- `Merged bool` - PR was merged
- `MergeCommit string` - SHA of merge commit
- `CITimedOut bool` - CI polling timed out
- `MergeError string` - Error during CI/merge

**Modified `runFinalizeAsync()` to:**
- Call `CIMerger.WaitForCIAndMerge()` after successful finalize
- Update progress events ("Waiting for CI", "Merged", etc.)
- Populate result fields without failing finalize on merge errors

### 4. Tests

**`internal/executor/ci_merge_test.go`:**
- Tests for `NewCIMerger`
- Tests for disabled CI wait
- Tests for missing PR URL
- Tests for CI check status parsing
- Tests for result fields

**`internal/config/config_test.go`:**
- `TestDefault_CIMergeConfig` - Verifies defaults
- `TestShouldWaitForCI` - Tests profile-based behavior
- `TestShouldMergeOnCIPass` - Tests combination logic
- `TestGetCITimeout` - Tests timeout with defaults/custom values

### Behavior

The flow now works as:
1. Finalize completes successfully (branch synced, tests passed)
2. If `wait_for_ci: true` (auto/fast profiles):
   - Push finalize changes
   - Poll CI checks every 30 seconds
   - Wait up to `ci_timeout` (default 10m)
3. If CI passes and `merge_on_ci_pass: true`:
   - Merge PR with squash
   - Delete branch
4. Report results in `FinalizeResult`

This removes dependency on GitHub's auto-merge feature (requires branch protection) and self-approval (blocked by GitHub).

<phase_complete>true</phase_complete>

---
Tokens: 10549592 input, 28110 output, 132850 cache_creation, 10416629 cache_read
Complete: true
Blocked: false
