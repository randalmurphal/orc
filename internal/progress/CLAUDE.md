# Progress Package

Progress display for orc task execution, providing visual feedback to users.

## File Structure

| File | Purpose |
|------|---------|
| `display.go` | Progress display implementation |
| `display_test.go` | Display tests |

## Display

The `Display` struct provides consistent progress output:

```go
display := progress.New("TASK-001", false /* quiet */)

// Phase lifecycle
display.PhaseStart("implement", 30)
display.Update(5, 15000)  // iteration, tokens
display.PhaseComplete("implement", "abc123")
display.PhaseFailed("implement", err)

// Activity tracking
display.SetActivity(progress.ActivityWaitingAPI)
display.Heartbeat()
display.IdleWarning(2 * time.Minute)
display.TurnTimeout(10 * time.Minute)
display.Cancelled()

// Gates
display.GatePending("review", "ai")
display.GateApproved("review")
display.GateRejected("review", "needs tests")

// Task lifecycle
display.TaskComplete(50000, 15*time.Minute, &progress.FileChangeStats{
    FilesChanged: 5,
    Additions:    150,
    Deletions:    20,
})
display.TaskFailed(err)
display.TaskInterrupted()

// Messages
display.Info("Starting worktree setup")
display.Warning("Token usage high")
display.Error("Failed to connect")  // Always shown, even in quiet mode
```

## Activity States

| State | Description | Display |
|-------|-------------|---------|
| `ActivityIdle` | No activity | - |
| `ActivityWaitingAPI` | Waiting for API | "Waiting for Claude API..." |
| `ActivityStreaming` | Receiving response | Progress dots |
| `ActivityRunningTool` | Running tool | "Running tool..." |
| `ActivityProcessing` | Processing | - |

## Output Modes

- **Normal mode**: Full progress output with emoji indicators
- **Quiet mode**: Suppresses most output, but errors and warnings still shown
- **Errors always shown**: `Error()`, `TaskFailed()`, `IdleWarning()`, `TurnTimeout()`, `Cancelled()` are never suppressed

## Example Output

```
🚀 Starting phase: implement (max 30 iterations)
⏳ Waiting for Claude API...
.... (2m30s)
⚠️  No activity for 2m - API may be slow or stuck
✅ Phase implement complete (commit: abc1234, elapsed: 5m30s)

🎉 Task TASK-001 completed!
   Total tokens: 50000
   Total time: 15m30s
   Modified: 5 files (+150/-20)
```

## Testing

```bash
go test ./internal/progress/... -v
```
