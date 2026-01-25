# Orchestrator Package

Multi-task coordination for parallel Claude agent execution.

## Overview

The orchestrator manages multiple Claude sessions running in parallel, each in its own git worktree. It provides dependency-aware scheduling, worker management, and event publishing.

## Key Types

| Type | Purpose |
|------|---------|
| `Orchestrator` | Main coordinator |
| `Scheduler` | Dependency-aware task queue |
| `WorkerPool` | Manages task workers |
| `Worker` | Executes single task |

## File Structure

| File | Purpose |
|------|---------|
| `orchestrator.go` | Main orchestrator logic |
| `scheduler.go` | Priority queue with dependencies |
| `worker.go` | Worker pool and task execution |
| `worker_unix.go` | Unix process group handling |
| `worker_windows.go` | Windows stub (no-op) |

## Architecture

```
Orchestrator
├── Scheduler (priority queue)
│   ├── AddTask()
│   ├── NextReady() -> tasks with deps satisfied
│   └── MarkCompleted()
├── WorkerPool
│   ├── SpawnWorker() -> creates worker in worktree
│   ├── StopWorker()
│   └── GetWorkers()
└── Main Loop
    ├── checkWorkers() -> handle completions/failures
    └── scheduleNext() -> spawn ready tasks
```

## Usage

```go
// Create orchestrator
cfg := &orchestrator.Config{
    MaxConcurrent: 4,
    PollInterval:  2 * time.Second,
}
o := orchestrator.New(cfg, orcConfig, publisher, gitOps, promptSvc, logger)

// Add tasks
o.AddTask("TASK-001", "First task", nil, orchestrator.PriorityDefault)
o.AddTask("TASK-002", "Second task", []string{"TASK-001"}, orchestrator.PriorityDefault)

// Or add from initiative
init, _ := initiative.Load("INIT-001")
o.AddTasksFromInitiative(init)

// Start orchestration
ctx := context.Background()
o.Start(ctx)

// Wait for completion
o.Wait()

// Or stop manually
o.Stop()
```

## Scheduling

Tasks are scheduled based on:
1. **Dependencies**: All dependencies must be completed
2. **Priority**: Higher priority runs first
3. **Creation time**: Earlier tasks run first (within same priority)

Priority levels:
- `PriorityUrgent` (1000): Critical tasks
- `PriorityDefault` (100): Normal tasks
- `PriorityBackground` (10): Low priority

## Worker Lifecycle

1. `SpawnWorker`: Creates worktree, starts claude process
2. Worker updates task execution state with phase prompt
3. Claude runs with `--dangerously-skip-permissions`
4. On completion: phase marked complete in task execution state
5. Worker continues to next phase or marks task complete
6. Worker self-removes from pool map (immediate capacity release)
7. Cleanup: worktree removed (if configured)

### Worker Pool Cleanup

Workers remove themselves from the `WorkerPool.workers` map immediately when their run completes (success or failure). This ensures pool capacity is freed without waiting for the next orchestrator tick.

**Key behaviors:**
- `RemoveWorker()` is idempotent - safe to call multiple times
- Orchestrator handlers check `GetWorker()` first (worker may have self-cleaned)
- `ActiveCount()` only counts workers with `running` status

### Process Group Handling

Workers create child processes (MCP servers like Playwright, chromium) that must be terminated when the worker stops. Process groups ensure clean termination of all descendant processes.

**Implementation:**
- `setProcAttr(cmd)` sets `SysProcAttr.Setpgid = true` on command creation
- `Worker.Stop()` calls `cancel()` AND `killProcessGroup()`
- `killProcessGroup()` sends SIGKILL to negative PID (entire process group)
- Context cancellation also triggers process group cleanup

**Platform-specific:**
- **Unix (Linux/macOS)**: Uses `syscall.Kill(-pid, SIGKILL)` to terminate process group
- **Windows**: No-op (Windows uses job objects differently; context cancellation sufficient)

**Why this matters:**
Without process groups, `exec.CommandContext` only kills the direct child process on cancellation. Child processes spawned by Claude (MCP servers) are reparented to init (PID 1) and continue running, causing resource leaks.

## Events

Published events:
- `EventPhaseStart`: Phase execution started
- `EventPhaseComplete`: Phase completed
- `EventTaskComplete`: All phases completed
- `EventTaskFailed`: Task failed

## Configuration

```go
type Config struct {
    MaxConcurrent int           // Max parallel tasks (default: 4)
    PollInterval  time.Duration // State check interval (default: 2s)
    WorkerTimeout time.Duration // Per-task timeout (0 = none)
}
```

## Testing

```bash
go test ./internal/orchestrator/... -v
```
