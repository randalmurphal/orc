# Session Update WebSocket Event

## Overview

The `session_update` WebSocket event provides real-time session metrics to connected clients. It broadcasts aggregate statistics about the current orc session, including token usage, cost, duration, and active tasks.

## Event Schema

```json
{
  "type": "event",
  "event": "session_update",
  "task_id": "*",
  "time": "2026-01-21T12:34:56Z",
  "data": {
    "duration_seconds": 3650,
    "total_tokens": 127500,
    "input_tokens": 85000,
    "output_tokens": 42500,
    "estimated_cost_usd": 2.51,
    "tasks_running": 2,
    "is_paused": false
  }
}
```

## Broadcast Triggers

Session updates are broadcast in the following scenarios:

### 1. Immediate Broadcasts (< 100ms)
- **Task Start**: When any task begins execution
- **Task Complete**: When any task finishes (completed, failed, or paused)
- **Pause State Change**: When executor pause state changes (via `OnPauseChanged`)
- **Client Reconnect**: When a client subscribes to `task_id: "*"` (global subscription)

### 2. Periodic Heartbeat
- **Every 10 seconds** while tasks are running
- Automatically starts when first task starts
- Automatically stops when last task completes
- No broadcasts when idle (zero tasks running)

## Implementation Details

### Components

#### 1. SessionBroadcaster (`internal/executor/session_broadcaster.go`)
- Manages periodic and event-driven broadcasts
- Tracks session start time, running task count, pause state
- Uses 10-second ticker for heartbeat updates
- Thread-safe: All methods can be called concurrently

#### 2. EventPublisher (`internal/executor/publish.go`)
- `Session()` method publishes session updates
- Uses `GlobalTaskID` ("*") so all global subscribers receive updates

#### 3. WebSocket Handler (`internal/api/websocket.go`)
- Forwards session_update events to subscribed clients
- Sends initial session state on global subscription (line 221-231)
- Handles broadcast to multiple clients simultaneously

### Integration Points

#### Executor Integration
```go
// internal/executor/executor.go:367-376
func (e *Executor) SetPublisher(p events.Publisher) {
    e.publisher = p
    if p != nil {
        e.sessionBroadcaster = NewSessionBroadcaster(
            NewEventPublisher(p),
            e.backend,
            e.globalDB,
            e.config.WorkDir,
            e.logger,
        )
    }
}
```

#### Task Execution Hooks
```go
// internal/executor/task_execution.go:74-75, 961-962
e.sessionBroadcaster.OnTaskStart(ctx)
defer e.sessionBroadcaster.OnTaskComplete(ctx)
```

## Client Usage

### Subscribe to Session Updates

```javascript
const ws = new WebSocket('ws://localhost:8080/api/ws');

ws.onopen = () => {
  // Subscribe to all events (global subscription)
  ws.send(JSON.stringify({
    type: 'subscribe',
    task_id: '*'
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  // Initial subscription confirmation
  if (msg.type === 'subscribed') {
    console.log('Subscribed to global events');
    return;
  }

  // Session update events
  if (msg.type === 'event' && msg.event === 'session_update') {
    const metrics = msg.data;
    console.log('Session metrics:', {
      duration: `${metrics.duration_seconds}s`,
      tokens: metrics.total_tokens,
      cost: `$${metrics.estimated_cost_usd.toFixed(2)}`,
      running: metrics.tasks_running,
      paused: metrics.is_paused
    });
  }
};
```

### React Hook Example

```typescript
import { useEffect, useState } from 'react';

interface SessionMetrics {
  duration_seconds: number;
  total_tokens: number;
  input_tokens: number;
  output_tokens: number;
  estimated_cost_usd: number;
  tasks_running: number;
  is_paused: boolean;
}

export function useSessionMetrics() {
  const [metrics, setMetrics] = useState<SessionMetrics | null>(null);

  useEffect(() => {
    const ws = new WebSocket('ws://localhost:8080/api/ws');

    ws.onopen = () => {
      ws.send(JSON.stringify({ type: 'subscribe', task_id: '*' }));
    };

    ws.onmessage = (event) => {
      const msg = JSON.parse(event.data);
      if (msg.type === 'event' && msg.event === 'session_update') {
        setMetrics(msg.data);
      }
    };

    return () => ws.close();
  }, []);

  return metrics;
}
```

## Testing

### Unit Tests
- **SessionBroadcaster Tests**: `internal/executor/session_broadcaster_test.go`
  - `TestSessionBroadcaster_OnTaskStart` - Verifies immediate broadcast on task start
  - `TestSessionBroadcaster_OnTaskComplete` - Verifies immediate broadcast on task complete
  - `TestSessionBroadcaster_OnPauseChanged` - Verifies broadcast on pause state change
  - `TestSessionBroadcaster_MultipleTasks` - Verifies running task count tracking
  - `TestSessionBroadcaster_TickerStopsWhenIdle` - Verifies ticker stops when no tasks running
  - `TestSessionBroadcaster_DurationTracking` - Verifies session duration calculation
  - `TestSessionBroadcaster_GetCurrentMetrics` - Verifies metrics retrieval without broadcast
  - `TestSessionBroadcaster_ConcurrentAccess` - Verifies thread safety

### Integration Tests
- **WebSocket Tests**: `internal/api/websocket_test.go`
  - `TestWSHandler_GlobalSubscription_InitialSessionUpdate` (lines 452-518)
    - Verifies initial session_update sent on global subscription
    - Validates all required fields present in event data

### Manual Testing
```bash
# Terminal 1: Start server
orc serve

# Terminal 2: Subscribe to session updates
wscat -c ws://localhost:8080/api/ws
> {"type":"subscribe","task_id":"*"}

# Terminal 3: Run a task
orc run TASK-001

# Observe session_update events in Terminal 2:
# - Immediate update when task starts (tasks_running: 1)
# - Periodic updates every 10s while task runs
# - Immediate update when task completes (tasks_running: 0)
```

## Performance Characteristics

- **Broadcast Latency**: < 100ms for state changes (task start/complete/pause)
- **Heartbeat Interval**: 10 seconds (configurable via `SessionBroadcastInterval`)
- **Ticker Overhead**: Minimal (stops automatically when idle)
- **Thread Safety**: All operations are mutex-protected
- **Memory**: O(1) - only current session state tracked

## Troubleshooting

### No Session Updates Received

1. **Check subscription**: Ensure subscribed to `task_id: "*"` (global subscription)
2. **Verify executor has publisher**: `executor.SessionBroadcaster()` must not be nil
3. **Check task execution**: Session updates only broadcast when tasks run
4. **Inspect logs**: Look for "session update broadcast" debug messages

### Updates Not Stopping When Idle

- The ticker should stop automatically when `TasksRunning` reaches zero
- If updates continue, check for stale running task count
- Verify `OnTaskComplete()` is called (check for panics in executor)

### Incorrect Metrics

- **Token counts**: Aggregated from global DB (`db.GlobalDB.GetCostSummary`)
- **Duration**: Calculated from session start time (first task start)
- **Tasks running**: Atomic counter updated on task start/complete
- **Pause state**: Atomic bool updated via `OnPauseChanged()`

## Related Files

- `internal/events/types.go:55` - EventSessionUpdate constant
- `internal/events/types.go:148-163` - SessionUpdate struct definition
- `internal/executor/session_broadcaster.go` - SessionBroadcaster implementation
- `internal/executor/session_broadcaster_test.go` - Comprehensive unit tests
- `internal/executor/publish.go:149-152` - EventPublisher.Session() method
- `internal/executor/task_execution.go:74-75, 961-962` - Task lifecycle hooks
- `internal/api/websocket.go:221-231` - Initial session_update on subscription
- `internal/api/websocket_test.go:452-518` - WebSocket integration test
