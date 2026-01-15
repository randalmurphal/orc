# Specification: Guard PRPoller.Stop() against double-close panic

## Problem Statement

`PRPoller.Stop()` directly calls `close(p.stopCh)` without protection against multiple calls. If `Stop()` is called twice (e.g., from concurrent shutdown paths or error recovery), Go will panic with "close of closed channel".

## Success Criteria

- [ ] `PRPoller.Stop()` can be called multiple times without panic
- [ ] Uses `sync.Once` pattern consistent with `ActivityTracker.Stop()` in the same codebase
- [ ] Existing behavior preserved: first call closes channel and waits for goroutine, subsequent calls return immediately
- [ ] No race conditions on the stop path
- [ ] Unit test verifies double-call safety

## Testing Requirements

- [ ] Unit test: `TestPRPoller_StopTwice` - calls `Stop()` twice in succession, verifies no panic
- [ ] Unit test: `TestPRPoller_StopConcurrent` - calls `Stop()` from multiple goroutines simultaneously
- [ ] Existing tests continue to pass (no regression in normal start/stop flow)

## Scope

### In Scope
- Add `sync.Once` field to `PRPoller` struct
- Modify `Stop()` to use `stopOnce.Do()` pattern
- Add unit tests for double-call and concurrent-call safety

### Out of Scope
- Changes to `Start()` behavior
- Changes to polling logic
- Multiple start/stop cycles (struct reuse) - not a current requirement
- Other pollers or similar components (can be addressed in separate tasks if needed)

## Technical Approach

Follow the existing pattern from `ActivityTracker` (`internal/executor/activity.go:141-146`):

```go
func (t *ActivityTracker) Stop() {
    t.stopOnce.Do(func() {
        close(t.stopCh)
    })
    t.wg.Wait()
}
```

### Files to Modify

- `internal/api/pr_poller.go`:
  - Add `stopOnce sync.Once` field to `PRPoller` struct (line ~26)
  - Wrap `close(p.stopCh)` in `p.stopOnce.Do()` in `Stop()` method (line ~74)

- `internal/api/pr_poller_test.go`:
  - Add `TestPRPoller_StopTwice` test function
  - Add `TestPRPoller_StopConcurrent` test function

## Bug Analysis

### Reproduction Steps
1. Create a `PRPoller` instance via `NewPRPoller()`
2. Start it with `Start(ctx)`
3. Call `Stop()` twice (directly or via concurrent shutdown paths)
4. Observe panic: `panic: close of closed channel`

### Current Behavior
`Stop()` unconditionally calls `close(p.stopCh)`, which panics if the channel is already closed.

### Expected Behavior
`Stop()` should be idempotent - first call closes the channel and waits for the goroutine, subsequent calls return immediately without error or panic.

### Root Cause
Missing synchronization primitive (`sync.Once`) to guard the channel close operation.

### Verification
1. Run new unit tests: `go test -run TestPRPoller_Stop ./internal/api/`
2. Run all existing PR poller tests to verify no regression
3. Manually test server shutdown path (though this is harder to trigger double-stop in practice)
