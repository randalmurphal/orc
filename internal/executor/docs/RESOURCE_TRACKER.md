# Resource Tracker Reference

Tracks process and memory state before/after task execution to detect orphaned processes (MCP servers, browsers) that survive beyond Claude sessions.

## Problem Being Solved

When orc runs tasks with MCP servers (Playwright, etc.):

```
orc (Go process)
└── Claude CLI (spawned by llmkit)
    └── MCP servers (Playwright, etc)
        └── Chromium browsers
```

When Claude CLI session ends, llmkit kills the direct child but not the entire process group. MCP servers and browsers become orphaned and accumulate over multiple tasks, eventually causing system freezes.

## Types

```go
type ProcessInfo struct {
    PID      int      // Process ID
    PPID     int      // Parent process ID
    Command  string   // Process command (truncated to 100 chars)
    MemoryMB float64  // RSS memory in MB
    IsMCP    bool     // True if matches MCP patterns
}

type ProcessSnapshot struct {
    Timestamp     time.Time
    Processes     []ProcessInfo
    TotalMemoryMB float64
    ProcessCount  int
}

type ResourceTrackerConfig struct {
    Enabled            bool  // Enable tracking (default: true)
    MemoryThresholdMB  int   // Warn if growth > threshold (default: 100)
    LogOrphanedMCPOnly bool  // Only log MCP-related orphans (default: false)
}
```

## Usage Flow

```go
// Create tracker from config
rtConfig := ResourceTrackerConfig{
    Enabled:           cfg.Diagnostics.ResourceTracking.Enabled,
    MemoryThresholdMB: cfg.Diagnostics.ResourceTracking.MemoryThresholdMB,
}
tracker := NewResourceTracker(rtConfig, logger)

// Before task execution (in task_execution.go)
tracker.SnapshotBefore()

// Task runs...

// After task execution (in executor.go runResourceAnalysis)
tracker.SnapshotAfter()
tracker.DetectOrphans()      // Returns orphaned processes, logs warning
tracker.CheckMemoryGrowth()  // Returns delta, logs warning if > threshold
tracker.Reset()              // Clear for next task
```

## Orphan Detection Logic

A process is considered orphaned if:
1. It didn't exist before task started (new PID)
2. AND one of:
   - Its parent is init (PID 1) - reparented after parent died
   - Its parent process no longer exists

## MCP Process Detection

```go
var mcpProcessPattern = regexp.MustCompile(
    `(?i)(playwright|chromium|chrome|firefox|webkit|puppeteer|selenium)`)

func IsMCPProcess(command string) bool
```

## Platform Support

| Platform | Method |
|----------|--------|
| Linux | `/proc/[pid]/stat`, `/proc/[pid]/status`, `/proc/[pid]/cmdline` |
| macOS | `ps -axo pid,ppid,rss,comm` |
| Windows | `wmic process get ...` or `tasklist /fo csv` |

## Configuration

```yaml
# config.yaml
diagnostics:
  resource_tracking:
    enabled: true            # Enable process/memory tracking
    memory_threshold_mb: 100 # Warn if memory grows by >100MB
    log_orphaned_mcp_only: false  # Log all orphans, not just MCP
```

## Log Output

```
INFO resource snapshot taken (before) processes=145 memory_mb=2456.3
INFO resource snapshot taken (after) processes=148 memory_mb=2892.1
WARN orphaned processes detected count=3 processes="chromium (PID=12345) [MCP], playwright-server (PID=12346) [MCP], webkit (PID=12347) [MCP]"
WARN memory growth exceeded threshold delta_mb=435.8 threshold_mb=100 before_mb=2456.3 after_mb=2892.1
```

## Integration Points

| Location | Method | Purpose |
|----------|--------|---------|
| `task_execution.go:ExecuteTask()` | `SnapshotBefore()` | Capture baseline |
| `executor.go:runResourceAnalysis()` | `SnapshotAfter()`, `DetectOrphans()`, `CheckMemoryGrowth()`, `Reset()` | Post-task analysis |
