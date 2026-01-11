# Phase 3: Execution Issues

## Issue: No real-time transcript visibility during task execution

**Severity**: High
**Status**: Fixed

**Steps to Reproduce:**
1. Run `orc run TASK-XXX`
2. Observe output during execution
3. Try to see what Claude is actually doing

**Expected Behavior:**
- Should be able to see Claude's actual conversation in real-time (at least with --verbose)
- If interrupted, should have some visibility into what happened
- The transcript should be accessible even during execution

**Actual Behavior (before fix):**
- Only see INFO level logs: "executing phase phase=implement task=TASK-001"
- No visibility into what Claude is reading, writing, or reasoning about
- Transcripts only saved AFTER phase completion
- If interrupted, no transcript is saved at all (zero visibility)
- The `--verbose` flag exists but doesn't stream transcript content

**Impact:**
- Cannot debug why tasks fail or get stuck
- Cannot monitor progress of long-running tasks
- No forensics available for interrupted tasks
- Users have no idea what's happening during execution

**Root Cause:**
- `progress.Display` only shows high-level updates (phase start/complete, iteration count)
- `EventPublisher.Transcript()` exists but events only go to WebSocket for UI
- CLI doesn't subscribe to transcript events
- No streaming to stdout option

**Fix Applied:**
- [x] Created `internal/events/cli_publisher.go` - CLIPublisher that streams transcript events to stdout
- [x] Added `--stream` flag to `orc run` command
- [x] Streaming also enabled by `--verbose` flag
- [x] Added tests in `internal/events/cli_publisher_test.go`
- [x] Verified working: `orc run TASK-XXX --stream` now shows full prompt/response

**Usage:**
```bash
orc run TASK-001 --stream      # Stream transcripts to stdout
orc run TASK-001 --verbose     # Also enables streaming
```

---

## Issue: Completion action fails with missing remote branch

**Severity**: Medium
**Status**: Open

**Steps to Reproduce:**
1. Have a git repo with origin remote but no remote tracking branch (e.g., new repo not pushed)
2. Run `orc run TASK-XXX`
3. Task completes but shows warning about sync failure

**Expected Behavior:**
Should either:
- Skip sync if remote branch doesn't exist
- Provide clearer message about why sync failed
- Check for remote branch existence before attempting

**Actual Behavior:**
```
WARN completion action failed error="sync with target: rebase onto origin/main: fatal: invalid upstream 'origin/main'"
```

**Impact:**
- Task still completes successfully
- But UX is confusing - user sees warning but task marked as complete

**Fix Applied:**
- [ ] Not yet fixed

---
