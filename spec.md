# Specification: Orphan detection flags system processes as false positives

## Problem Statement
The resource tracker's orphan detection incorrectly flags system processes (systemd-timedated, snapper, snapperd) as orphans because they legitimately have PPID=1 (systemd is their parent). The current logic assumes any new process with PPID=1 is an orc-spawned process that was reparented after its parent died, but this assumption is false for system services that start coincidentally during task execution.

## Success Criteria
- [ ] System processes (systemd-*, snapper*, dbus-daemon, cron jobs, etc.) are not flagged as orphans
- [ ] MCP-related processes (playwright, chromium, firefox, webkit, puppeteer, selenium, node MCP servers) that are reparented to init are still detected
- [ ] Claude Code processes (claude-code, npx, npm related to claude) are still detected if orphaned
- [ ] Filter list is configurable via regex patterns in ResourceTrackerConfig
- [ ] Existing behavior is preserved when `LogOrphanedMCPOnly: true` (only MCP orphans logged)
- [ ] False positives from evidence logs no longer appear:
  - `/usr/lib/systemd/systemd-timedated` excluded
  - `/usr/lib/snapper/systemd-helper --cleanup` excluded
  - `/usr/sbin/snapperd` excluded

## Testing Requirements
- [ ] Unit test: `TestOrphanDetectionFiltersSystemProcesses` - verifies systemd/snapper processes excluded
- [ ] Unit test: `TestOrphanDetectionAllowsOrcProcesses` - verifies orc-related processes (claude, playwright, node MCP) still detected
- [ ] Unit test: `TestOrphanDetectionCustomExcludePatterns` - verifies custom exclude patterns work
- [ ] Unit test: `TestIsSystemProcess` - verifies system process detection helper function
- [ ] Existing tests continue to pass (TestOrphanDetection, TestOrphanDetectionMCPOnly, etc.)

## Scope
### In Scope
- Add system process exclusion patterns to filter known false positives
- Add `orcProcessPattern` to identify orc-spawned processes we care about
- Modify `DetectOrphans()` to filter out system processes
- Add configurable exclude patterns to `ResourceTrackerConfig`
- Add helper function `IsSystemProcess()` for testing/debugging

### Out of Scope
- Process group tracking (requires significant executor changes)
- Tracking orc process tree ancestry (complex, platform-specific)
- Windows-specific system process filtering (Windows already has limited functionality)
- Kernel thread filtering (already filtered by empty command)

## Technical Approach

### Strategy: Allowlist + Blocklist Filtering

The fix uses a two-pronged approach:
1. **Blocklist**: Exclude known system process patterns from orphan detection
2. **Allowlist**: Only flag new processes that match orc-related patterns (MCP browsers, node/npx, claude)

This dual approach ensures we:
- Don't miss actual orphaned orc processes (allowlist catches them)
- Don't flag unrelated system activity (blocklist excludes them)

### Implementation Details

1. **Add system process exclusion pattern**:
   ```go
   var systemProcessPattern = regexp.MustCompile(`(?i)(^/usr/(lib|sbin)/systemd|systemd-|^/usr/(lib|sbin)/snapper|snapperd|dbus-daemon|dbus-broker|polkitd|udisksd|upowerd|packagekitd|fwupd|thermald|irqbalance|crond?$|atd$|anacron)`)
   ```

2. **Add orc process pattern** (processes we want to track):
   ```go
   var orcProcessPattern = regexp.MustCompile(`(?i)(playwright|chromium|chrome|firefox|webkit|puppeteer|selenium|claude|node.*mcp|npx.*mcp|npm.*mcp)`)
   ```

3. **Modify `DetectOrphans()` logic**:
   - Skip processes matching `systemProcessPattern`
   - When `LogOrphanedMCPOnly: false`, only flag processes matching `orcProcessPattern` OR with suspicious characteristics (very recent start, high memory)
   - When `LogOrphanedMCPOnly: true`, existing behavior (only MCP pattern)

4. **Add `ExcludePatterns` to config** for user customization:
   ```go
   type ResourceTrackerConfig struct {
       Enabled            bool
       MemoryThresholdMB  int
       LogOrphanedMCPOnly bool
       ExcludePatterns    []string // Additional patterns to exclude
   }
   ```

### Files to Modify
- `internal/executor/resource_tracker.go`:
  - Add `systemProcessPattern` regex constant
  - Add `orcProcessPattern` regex constant
  - Add `ExcludePatterns` field to `ResourceTrackerConfig`
  - Add `IsSystemProcess(command string) bool` helper function
  - Add `IsOrcProcess(command string) bool` helper function
  - Modify `DetectOrphans()` to filter system processes and only flag orc-related processes
  - Update `ProcessInfo` struct to add `IsOrc` field (parallel to `IsMCP`)
- `internal/executor/resource_tracker_test.go`:
  - Add `TestOrphanDetectionFiltersSystemProcesses`
  - Add `TestOrphanDetectionAllowsOrcProcesses`
  - Add `TestOrphanDetectionCustomExcludePatterns`
  - Add `TestIsSystemProcess`
  - Update existing tests to account for new filtering

## Bug Analysis

### Reproduction Steps
1. Enable resource tracking in orc config: `diagnostics.resource_tracking.enabled: true`
2. Run any orc task: `orc run TASK-XXX`
3. During task execution, if any systemd timer or service activates (e.g., snapper cleanup, time sync), it gets captured in the "after" snapshot
4. These processes have PPID=1 (systemd is their parent) and are flagged as orphans

### Current Behavior
```
WARN orphaned processes detected count=3 processes="/usr/lib/systemd/systemd-timedated (PID=1063943), /usr/lib/snapper/systemd-helper --cleanup (PID=1074076), /usr/sbin/snapperd (PID=1074078)"
```

### Expected Behavior
- System processes should be silently ignored
- Only actual orc-spawned processes (playwright, chromium, node MCP servers, claude processes) should be flagged if orphaned
- Warning should only appear when genuine orphaned processes are detected

### Root Cause
Line 132 in `resource_tracker.go`:
```go
isOrphan := p.PPID == 1 || !afterPIDSet[p.PPID]
```

This assumes any process with PPID=1 is reparented (orphaned), but system services legitimately have PPID=1 because systemd (PID 1) is their actual parent process manager.

### Verification
1. Run `orc run` with resource tracking enabled on a system with active systemd timers
2. Observe no warnings for systemd-*, snapper*, or other system processes
3. Manually spawn an orphaned playwright process and verify it IS detected
4. Run existing test suite to confirm no regressions
