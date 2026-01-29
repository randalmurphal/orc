# API Package

Connect RPC server with WebSocket support for real-time updates.

## Architecture

```
Server
├── Connect RPC Services (13) → *_server.go
├── EventServer               → event_server.go (internal events)
│   └── WebSocketHub          → websocket.go (forwards to web clients)
├── Interceptors              → interceptors.go
├── PR Poller                 → Background PR status updates
├── Finalize Tracker          → Async finalize operations
└── Event Publisher           → Real-time task updates
```

**Event Flow**: EventServer.Publish() → WebSocketHub.handleInternalEvent() → WebSocket clients. Critical: `SetWebSocketHub()` must be called at startup to wire these together.

## Connect RPC Services

Services are registered in `server_connect.go:15-79`. Each implements a handler interface from `orcv1connect`.

| Service | File | Key Methods |
|---------|------|-------------|
| `TaskService` | `task_server.go` | CRUD, Run/Pause/Resume, Diff, Comments, Attachments |
| `InitiativeService` | `initiative_server.go` | CRUD, Link tasks, Dependency graph |
| `WorkflowService` | `workflow_server.go` | List, Get workflows and phases |
| `TranscriptService` | `transcript_server.go` | Get, Stream transcripts |
| `EventService` | `event_server.go` | Subscribe (streaming), GetEvents, GetTimeline |
| `ConfigService` | `config_server.go` | Get/Update orc config |
| `HostingService` | `hosting_server.go` | PR CRUD, Refresh, AutofixComment (GitHub + GitLab via Provider interface) |
| `DashboardService` | `dashboard_server.go` | Stats, Metrics (TTL cache + singleflight) |
| `ProjectService` | `project_server.go` | Multi-project management |
| `BranchService` | `branch_server.go` | Branch operations |
| `DecisionService` | `decision_server.go` | Gate decisions (approve/reject) |
| `NotificationService` | `notification_server.go` | Push notifications |
| `MCPService` | `mcp_server.go` | MCP server config |

## Key Patterns

### Service Implementation

```go
type taskServer struct {
    orcv1connect.UnimplementedTaskServiceHandler
    backend   storage.Backend
    config    *config.Config
    logger    *slog.Logger
    publisher events.Publisher
}

func (s *taskServer) GetTask(
    ctx context.Context,
    req *connect.Request[orcv1.GetTaskRequest],
) (*connect.Response[orcv1.GetTaskResponse], error) {
    // Implementation
}
```

### Error Handling

Interceptors map internal errors to Connect codes (`interceptors.go:56-117`):

| Internal Error | Connect Code |
|---------------|--------------|
| Task not found | `NotFound` |
| Validation error | `InvalidArgument` |
| Task already running | `FailedPrecondition` |
| Claude timeout | `DeadlineExceeded` |
| Default | `Internal` |

### Server Streaming (Events)

`EventService.Subscribe` provides real-time events via server streaming (`event_server.go:60-136`):

```go
func (s *eventServer) Subscribe(
    ctx context.Context,
    req *connect.Request[orcv1.SubscribeRequest],
    stream *connect.ServerStream[orcv1.SubscribeResponse],
) error {
    // Subscribe to publisher, forward events to stream
}
```

Clients can filter by task ID, initiative ID, or event types. Heartbeat support included.

## WebSocket Protocol (Legacy)

WebSocket handler at `/api/ws` (`websocket.go`) remains for backward compatibility.

### Client Messages

```json
{"type": "subscribe", "task_id": "TASK-001"}
{"type": "subscribe", "task_id": "*"}
{"type": "unsubscribe"}
{"type": "command", "task_id": "TASK-001", "action": "pause"}
{"type": "ping"}
```

### Server Messages

```json
{"type": "subscribed", "task_id": "TASK-001"}
{"type": "event", "event": "task_updated", "task_id": "...", "data": {...}}
{"type": "pong"}
```

## PR Status Polling

Background poller (`pr_poller.go`) monitors PR status changes:

- **Interval**: 60s default
- **Rate limit**: 30s per task minimum
- **Status derivation**: MERGED > CLOSED > Draft > changes_requested > approved > pending_review
- **Auto-trigger**: Optionally triggers finalize on PR approval

## Finalize Tracker

In-memory tracker for async finalize operations (`finalize_tracker.go`):

- **States**: pending, running, completed, failed
- **Progress**: Step, progress message, percentage
- **Cleanup**: Auto-cleans completed entries after 5 minutes
- **Events**: Publishes progress via event publisher

## Startup and Shutdown

**Startup** (`server.go:274-355`):
1. Seed built-in workflows and agents
2. Create event publisher
3. Register Connect handlers
4. Start PR poller
5. Prune stale worktrees

**Shutdown**:
1. Cancel server context
2. Cancel all finalize operations
3. Stop PR poller
4. Graceful HTTP shutdown (5s timeout)

## Interceptors

| Interceptor | Purpose |
|-------------|---------|
| `ErrorInterceptor` | Maps internal errors to Connect codes |
| `LoggingInterceptor` | Logs RPC method, duration, errors |
| `StreamLoggingInterceptor` | Logs streaming RPCs |
| `RecoverInterceptor` | Catches panics, returns internal error |

## Testing

```bash
mkdir -p internal/api/static  # Required for go:embed
go test ./internal/api/... -v
```

Or use `make test` which handles prerequisites.
