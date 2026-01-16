# Specification: Add process group handling to orchestrator workers

## Problem Statement

When an orchestrator worker is stopped (via context cancellation), the `claude` process receives a SIGKILL signal, but its child processes (MCP servers like Playwright, chromium, etc.) are orphaned and continue running. This causes resource leaks and can interfere with subsequent task executions.

## Success Criteria

- [ ] Worker's `exec.CommandContext` sets `SysProcAttr.Setpgid = true` to create new process groups
- [ ] Worker's `Stop()` method kills the entire process group (not just the parent process)
- [ ] Context cancellation triggers proper process group cleanup
- [ ] Unit tests verify process group creation and cleanup behavior
- [ ] All existing worker tests continue to pass
- [ ] No orphaned processes remain after worker stop (can be verified with resource tracker)

## Testing Requirements

- [ ] Unit test: Verify `SysProcAttr.Setpgid` is set on command
- [ ] Unit test: Verify `Stop()` sends signal to process group (negative PID)
- [ ] Unit test: Verify idempotent cleanup behavior (double-stop is safe)
- [ ] Integration test: Verify resource tracker detects fewer orphans after this change

## Scope

### In Scope
- Modify `Worker.run()` in `internal/orchestrator/worker.go` to set process group attributes
- Add process group kill logic to `Worker.Stop()` method
- Add helper function for cross-platform process group handling
- Add unit tests for new functionality
- Update orchestrator CLAUDE.md with the pattern

### Out of Scope
- Modifying other spawn points (spec/spawn.go, setup/spawn.go, planner/planner.go)
- Windows process group handling (Windows uses job objects, not POSIX process groups)
- Modifying llmkit/claude package (that's a separate repo)
- Resource tracker changes (it already detects orphans, this task fixes the cause)

## Technical Approach

### Key Insight

Go's `exec.CommandContext` only kills the direct child process on context cancellation. Child processes spawned by the command (like MCP servers) are reparented to init (PID 1) and continue running.

The solution is to:
1. Put the child process in its own process group using `Setpgid: true`
2. Kill the entire process group on cancellation using `syscall.Kill(-pgid, signal)`

### Implementation

1. **Add process group attribute when creating command** (`worker.go:173`):
```go
w.cmd = exec.CommandContext(w.ctx, "claude", args...)
w.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
```

2. **Add process group kill to Worker.Stop()**:
```go
func (w *Worker) Stop() {
    w.cancel()
    w.killProcessGroup()
}

func (w *Worker) killProcessGroup() {
    w.mu.Lock()
    cmd := w.cmd
    w.mu.Unlock()

    if cmd == nil || cmd.Process == nil {
        return
    }

    pid := cmd.Process.Pid
    if pid > 0 {
        // Kill the entire process group
        syscall.Kill(-pid, syscall.SIGKILL)
    }
}
```

3. **Add cleanup on context cancellation in run loop**:
The run loop already handles context cancellation at line 192-195. Add process group cleanup there.

### Files to Modify

| File | Change |
|------|--------|
| `internal/orchestrator/worker.go` | Add `SysProcAttr.Setpgid`, add `killProcessGroup()` method, modify `Stop()` |
| `internal/orchestrator/worker_unix.go` | Platform-specific process group kill (Linux/macOS) |
| `internal/orchestrator/worker_windows.go` | Stub for Windows (no process groups, just cancel) |
| `internal/orchestrator/worker_test.go` | Add tests for process group behavior |
| `internal/orchestrator/CLAUDE.md` | Document the process group pattern |

### Platform Considerations

Process groups are a POSIX concept. For cross-platform compatibility:
- Linux/macOS: Use `syscall.SysProcAttr{Setpgid: true}` and `syscall.Kill(-pid, signal)`
- Windows: No change needed (Windows uses job objects differently; context cancellation works adequately for single processes)

Use build tags to separate platform-specific code:
```go
//go:build !windows
// +build !windows
```

## Feature Analysis

### User Story
As a user running the orchestrator with multiple concurrent tasks, I want workers to cleanly terminate all child processes when stopped, so that no orphaned processes (especially MCP servers) consume resources after task completion.

### Acceptance Criteria
1. When `worker.Stop()` is called, the `claude` process AND all its descendants are terminated
2. When context is cancelled (timeout, user interrupt), all processes in the worker's process group are killed
3. Resource tracker no longer detects orphaned MCP processes after normal task completion
4. The change is transparent - existing code that calls `Stop()` doesn't need modification
5. No behavioral change on Windows (maintains current behavior)
