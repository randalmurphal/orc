# Specification: Memory growth threshold (100MB) triggers too frequently

## Problem Statement

The memory growth warning threshold of 100MB for resource tracking triggers constantly during normal task execution, generating excessive noise in logs. Observed deltas of 452MB to 6GB are common during tasks that launch Claude sessions and browser processes for UI testing.

## Success Criteria

- [ ] Default memory threshold increased from 100MB to 500MB
- [ ] Warning logs no longer appear for typical task execution (under ~500MB growth)
- [ ] Tasks with genuine memory issues (multi-GB sustained growth) still trigger warnings
- [ ] Configuration remains backward-compatible (existing `memory_threshold_mb` settings honored)
- [ ] Unit tests verify the new default value
- [ ] Documentation updated to reflect the new default

## Testing Requirements

- [ ] Unit test: Verify `Default()` returns `MemoryThresholdMB: 500`
- [ ] Unit test: Verify `CheckMemoryGrowth()` only logs warnings when delta exceeds threshold
- [ ] Unit test: Verify custom threshold from config is respected (backward compatibility)

## Scope

### In Scope
- Increase default `memory_threshold_mb` from 100 to 500
- Update default value in `internal/config/config.go`
- Update documentation in:
  - `docs/specs/CONFIG_HIERARCHY.md`
  - `internal/executor/docs/RESOURCE_TRACKER.md`
  - Project `CLAUDE.md` files

### Out of Scope
- Per-weight memory thresholds (Option 2 from task description)
- Sustained growth detection across multiple snapshots (Option 3)
- Log level changes (Option 4 - would hide useful warnings)
- Changing the memory tracking mechanism itself
- Any changes to orphan process detection

## Technical Approach

The fix is straightforward: increase the default threshold to a value that reflects normal task memory usage while still catching genuine problems.

**Rationale for 500MB:**
- Normal Claude sessions + Node.js processes: ~200-400MB
- Browser processes for UI testing: ~200-500MB additional
- 500MB provides headroom for normal operation while still catching multi-GB leaks
- Conservative enough to flag issues without being noisy

### Files to Modify

1. `internal/config/config.go:774`: Change `MemoryThresholdMB: 100` to `MemoryThresholdMB: 500`
2. `docs/specs/CONFIG_HIERARCHY.md:294`: Update comment from 100 to 500
3. `internal/executor/docs/RESOURCE_TRACKER.md:97`: Update example from 100 to 500
4. `CLAUDE.md` (both root and worktree): Update config docs table

## Bug Analysis

### Reproduction Steps
1. Run any orc task with UI testing (`requires_ui_testing: true`)
2. Or run any task that takes >30 seconds (normal Claude session duration)
3. Observe resource tracking logs at task completion

### Current Behavior
Warning logs appear constantly with messages like:
```
WARN memory growth exceeded threshold delta_mb=452.0 threshold_mb=100
WARN memory growth exceeded threshold delta_mb=748.1 threshold_mb=100
WARN memory growth exceeded threshold delta_mb=2601.5 threshold_mb=100
```

### Expected Behavior
Warnings should only appear for genuinely concerning memory growth (500MB+), not for normal task operation.

### Root Cause
The default threshold of 100MB was set too low for typical task execution. Claude sessions, Node.js processes, and browser instances for Playwright testing regularly consume several hundred megabytes combined.

### Verification
After fix:
1. Run `orc run` on a UI task - no memory warnings for normal execution
2. Manually set `memory_threshold_mb: 50` - verify warnings appear (backward compat)
3. Verify config default shows 500 via `orc config show`
