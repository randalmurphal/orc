# Specification: Add process/resource tracking to diagnose system freezes

## Problem Statement

System freezes occur across Linux, Mac, and WSL after running multiple orc tasks. Root cause: Claude CLI sessions spawn MCP servers (Playwright, browsers) that become orphaned when sessions close because llmkit kills only the parent process, not the entire process group.

## Success Criteria

- [ ] Process snapshot taken before each task execution with PID, PPID, command, and memory usage
- [ ] Process snapshot taken after each task execution
- [ ] New orphaned processes detected by comparing snapshots (processes spawned during task that survive after session close)
- [ ] Orphaned processes logged with warning level including process details
- [ ] Memory usage logged before/after each phase execution
- [ ] Memory growth > 100MB between phases triggers warning log
- [ ] MCP-related processes (playwright, chromium, firefox, webkit) specifically flagged in orphan detection
- [ ] Resource tracking can be disabled via config (`diagnostics.resource_tracking.enabled: false`)
- [ ] All tracking works cross-platform (Linux, macOS, Windows/WSL)

## Testing Requirements

- [ ] Unit test: `TestProcessSnapshot` - verifies snapshot captures correct fields
- [ ] Unit test: `TestOrphanDetection` - verifies orphan detection logic with mock data
- [ ] Unit test: `TestMemoryTracking` - verifies memory delta calculation
- [ ] Unit test: `TestResourceTrackerConfig` - verifies config enables/disables tracking
- [ ] Integration test: `TestResourceTrackingDuringTask` - runs a trivial task and verifies logs are emitted

## Scope

### In Scope
- Process snapshot functionality (before/after task execution)
- Orphan detection by comparing snapshots
- Memory usage tracking before/after phases
- Warning logs for detected orphans and memory growth
- Config option to disable tracking
- Cross-platform support via standard library (os, runtime)

### Out of Scope
- Automatic killing of orphaned processes (too risky - could kill unrelated processes)
- Process group killing fix in llmkit (separate PR, noted in task description)
- MCP server lifecycle hooks (would require MCP protocol changes)
- Real-time process monitoring during execution (polling would impact performance)
- UI components for displaying resource data (CLI logging only)

## Technical Approach

### Architecture

Create a new `internal/executor/resource_tracker.go` package with:

1. **ProcessSnapshot struct** - captures system state
2. **ResourceTracker** - manages snapshot lifecycle and comparison
3. **Integration hooks** - called from `ExecuteTask` and phase execution

### Files to Create

- `internal/executor/resource_tracker.go`: Main resource tracking logic
- `internal/executor/resource_tracker_test.go`: Unit tests
- `internal/config/diagnostics.go`: Config struct extensions (if not existing)

### Files to Modify

- `internal/executor/task_execution.go`: Add pre/post task tracking calls
  - Call `tracker.SnapshotBefore()` at start of `ExecuteTask()`
  - Call `tracker.SnapshotAfter()` and `tracker.DetectOrphans()` after task completion

- `internal/executor/executor.go`: Instantiate ResourceTracker, pass to phase executors
  - Add `resourceTracker *ResourceTracker` field to Executor
  - Initialize in `New()` based on config

- `internal/config/config.go`: Add diagnostics config section
  ```go
  Diagnostics struct {
      ResourceTracking struct {
          Enabled         bool `yaml:"enabled"`
          MemoryThreshold int  `yaml:"memory_threshold_mb"` // Default 100
      } `yaml:"resource_tracking"`
  } `yaml:"diagnostics"`
  ```

### Key Implementation Details

**Process Snapshot:**
```go
type ProcessInfo struct {
    PID     int
    PPID    int
    Command string
    MemoryMB float64
    IsMCP   bool // true if command matches playwright/chromium/webkit/firefox
}

type ProcessSnapshot struct {
    Timestamp time.Time
    Processes []ProcessInfo
    TotalMemoryMB float64
}
```

**Orphan Detection Logic:**
1. Record all PIDs before task starts
2. Record all PIDs after task ends
3. New PIDs = after - before
4. Filter new PIDs: parent PID no longer exists OR parent is init (PID 1)
5. Flag MCP-related processes for special attention

**Cross-Platform Process Enumeration:**
- Linux: Parse `/proc/[pid]/stat` and `/proc/[pid]/status`
- macOS: Use `ps aux` output parsing
- Windows: Use `syscall` or `os/exec` with `tasklist`

## Category-Specific Section: Bug Analysis

### Reproduction Steps
1. Run multiple orc tasks with UI testing enabled (spawns Playwright MCP)
2. After 5-10 tasks, check process list: `ps aux | grep -E 'playwright|chromium'`
3. Observe accumulating browser/playwright processes
4. Eventually system becomes sluggish, may freeze

### Current Behavior
- Claude CLI session ends
- llmkit calls `session.Close()` which kills Claude CLI process
- MCP servers (Playwright) and browsers spawned by MCP continue running
- These processes accumulate over multiple task executions
- System memory and CPU exhausted, causing freeze

### Expected Behavior
- After task completion, only the original orc process should remain
- Any processes spawned during task execution should be cleaned up
- Memory usage should return to baseline between tasks
- System should remain responsive after running many tasks

### Root Cause
llmkit's `session.Close()` uses `cmd.Process.Kill()` which only kills the direct child process (Claude CLI), not the entire process tree. This is standard Go behavior but insufficient for process trees with grandchildren.

The fix requires:
1. **This task**: Detect and log orphans so the problem is visible
2. **Future llmkit task**: Use `Setpgid` and `syscall.Kill(-pgid, syscall.SIGTERM)` to kill process groups

### Verification

After implementation:
1. Run `orc run TASK-XXX` with a task that uses Playwright MCP
2. Check logs for resource tracking output:
   ```
   INFO  resource snapshot taken processes=45 memory_mb=1234
   INFO  resource snapshot taken processes=48 memory_mb=1456
   WARN  orphaned processes detected count=3 processes="[chromium 12345, playwright 12346, chromium 12347]"
   WARN  memory growth exceeded threshold delta_mb=222 threshold_mb=100
   ```
3. Verify orphaned processes are correctly identified (not false positives from unrelated system activity)
