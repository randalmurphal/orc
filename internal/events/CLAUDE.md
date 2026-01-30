# Events Package

Real-time event publishing with dual-path delivery: immediate WebSocket broadcast + batched database persistence.

## File Structure

| File | Purpose |
|------|---------|
| `types.go` | Event/EventType definitions, 17 event type constants |
| `publisher.go` | `Publisher` interface, `MemoryPublisher` (in-memory broadcast) |
| `persistent.go` | `PersistentPublisher` (wraps MemoryPublisher + DB persistence) |
| `cli_publisher.go` | CLI-specific event publishing |
| `publish_helper.go` | Helper utilities for common event patterns |

## Key Types

| Type | Location | Purpose |
|------|----------|---------|
| `Event` | `types.go:70` | Core event: Type, TaskID, Data (any), Time |
| `EventType` | `types.go:8` | String enum (17 types) |
| `Publisher` | `publisher.go:11` | Interface: Publish, Subscribe, Unsubscribe, Close |
| `MemoryPublisher` | `publisher.go:24` | In-memory broadcast to subscriber channels |
| `PersistentPublisher` | `persistent.go:19` | Wraps MemoryPublisher + batched DB writes |

## Event Types

| Category | Types |
|----------|-------|
| Execution | `state`, `transcript`, `phase`, `error`, `complete`, `tokens` |
| Progress | `activity`, `heartbeat`, `warning` |
| Task CRUD | `task_created`, `task_updated`, `task_deleted` |
| Initiative CRUD | `initiative_created`, `initiative_updated`, `initiative_deleted` |
| Gates | `decision_required`, `decision_resolved` |
| Other | `session_update`, `files_changed` |

## Data Flow

```
Event Created → PersistentPublisher.Publish()
  ├→ MemoryPublisher (immediate broadcast to WebSocket subscribers)
  └→ Buffer → DB flush (every 10 events or 5 seconds)
```

## Subscription Model

- **Per-task**: `Subscribe(taskID)` receives events for that task
- **Global**: `Subscribe("*")` receives ALL task events (monitoring)
- **Non-blocking**: Full subscriber buffers are skipped (default buffer: 100)

## Integration Points

| Consumer | Purpose |
|----------|---------|
| `api/event_server.go` | WebSocket streaming to frontend |
| `executor/` | Publishes phase, error, state, gate events |
| `trigger/` | Publishes trigger_started/completed/failed events |
| `cli/` | Publishes task creation events |
