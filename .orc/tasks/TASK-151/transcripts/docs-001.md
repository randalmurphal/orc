# docs - Iteration 1

## Prompt

Update documentation for:

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

1. Update any relevant documentation files
2. Ensure CLAUDE.md reflects the changes if applicable
3. Add/update code comments where needed
4. Update README if user-facing changes were made

Keep iterating until documentation is complete.

When done, output:
<phase_complete>true</phase_complete>


## Response

All documentation updates are complete. Let me summarize what was added:

## Summary of Documentation Updates

### 1. `docs/architecture/EXECUTOR.md`
- Added new section **"CI Wait and Auto-Merge"** documenting:
  - Why direct merge (bypasses GitHub auto-merge limitations)
  - Auto-merge flow after finalize completes
  - CI status evaluation criteria
  - Polling behavior and settings
  - Merge methods (squash, merge, rebase)
  - Profile behavior (auto/fast only)
  - Configuration options
  - Error handling behavior
  - WebSocket events for progress

### 2. `docs/specs/CONFIG_HIERARCHY.md`
- Added CI config options to the Full Config Structure:
  - `completion.ci.wait_for_ci`
  - `completion.ci.ci_timeout`
  - `completion.ci.poll_interval`
  - `completion.ci.merge_on_ci_pass`
  - `completion.ci.merge_method`
- Added environment variables for CI settings:
  - `ORC_CI_WAIT`
  - `ORC_CI_TIMEOUT`
  - `ORC_CI_POLL_INTERVAL`
  - `ORC_CI_MERGE_ON_PASS`
  - `ORC_CI_MERGE_METHOD`

### 3. `docs/architecture/GIT_INTEGRATION.md`
- Added new section **"CI Wait and Auto-Merge"** documenting:
  - Flow after finalize
  - CI polling with `gh pr checks`
  - Bucket meanings
  - Configuration options
  - Profile restrictions table

### 4. `CLAUDE.md` (root)
- Added 4 new config options to Key Config Options table
- Added knowledge pattern entry for "CI wait and auto-merge" (TASK-151)

### 5. `internal/executor/CLAUDE.md`
- Added `ci_merge.go` and `ci_merge_test.go` to file structure table
- Added new section **"CI Merger (ci_merge.go)"** documenting:
  - Types (CIStatus, CICheckResult)
  - CIMerger usage and methods
  - Flow diagram
  - Configuration methods
  - CI check buckets table
  - WebSocket events
  - Error handling
- Added `ci_merge_test.go` to test coverage list

<phase_complete>true</phase_complete>

---
Tokens: 2934759 input, 10532 output, 98457 cache_creation, 2832295 cache_read
Complete: true
Blocked: false
