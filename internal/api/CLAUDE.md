# API Package

Connect RPC server with WebSocket support for real-time updates.

## Architecture

```
Server
├── Connect RPC Services (13) → *_server.go
├── ProjectCache              → project_cache.go (multi-project routing)
├── EventServer               → event_server.go (internal events)
│   └── WebSocketHub          → websocket.go (forwards to web clients)
├── Interceptors              → interceptors.go
├── PR Poller                 → Background PR status updates
├── Finalize Tracker          → Async finalize operations
└── Event Publisher           → Real-time task updates
```

**Event Flow**: EventServer.Publish() → WebSocketHub.handleInternalEvent() → WebSocket clients. Critical: `SetWebSocketHub()` must be called at startup to wire these together.

**Project Routing**: Every request carries `project_id`. Services resolve the correct `storage.Backend` via `ProjectCache` (`project_cache.go`). See [Multi-Project Routing](#multi-project-routing) below.

## Connect RPC Services

Services are registered in `server_connect.go:17-116`. Each implements a handler interface from `orcv1connect`. All project-scoped services receive `SetProjectCache()` during registration.

| Service | File | Project-Scoped | Key Methods |
|---------|------|:--------------:|-------------|
| `TaskService` | `task_server.go` | Yes | CRUD, Run/Pause/Resume, Diff, Comments, Attachments, Branch Control |
| `InitiativeService` | `initiative_server.go` | Yes | CRUD, Link tasks, Dependency graph |
| `WorkflowService` | `workflow_server.go` | Yes | List, Get workflows and phases, Variables |
| `TranscriptService` | `transcript_server.go` | Yes | Get, Stream transcripts |
| `EventService` | `event_server.go` | Yes | Subscribe (streaming), GetEvents, GetTimeline |
| `ConfigService` | `config_server.go` | Yes | Config, Settings, Hooks, Skills, ClaudeMd, Constitution, Prompts, Agents, Scripts, Tools |
| `HostingService` | `hosting_server.go` | Yes | PR CRUD, Refresh, AutofixComment |
| `DashboardService` | `dashboard_server.go` | Yes | Stats, Metrics (TTL cache + singleflight) |
| `ProjectService` | `project_server.go` | No | Multi-project management (global) |
| `BranchService` | `branch_server.go` | Yes | Branch operations |
| `DecisionService` | `decision_server.go` | Yes | Gate decisions (approve/reject) |
| `NotificationService` | `notification_server.go` | Yes | Push notifications |
| `MCPService` | `mcp_server.go` | No | MCP server config (global) |

## Key Patterns

### Service Implementation

Each `*_server.go` embeds `Unimplemented*Handler`, holds `backend storage.Backend`, `projectCache *ProjectCache`, and service-specific deps. RPC methods receive `connect.Request[T]` and return `connect.Response[T]`.

### Multi-Project Routing

Every project-scoped server follows this pattern for multi-project support:

**1. Struct fields** -- each server has `backend` (default) and `projectCache`.

**2. `SetProjectCache()`** -- called during registration (`server_connect.go:27-71`).

**3. `getBackend(projectID)`** -- resolves the correct `storage.Backend` (DB access):
```go
func (s *dashboardServer) getBackend(projectID string) (storage.Backend, error) {
    if projectID != "" && s.projectCache != nil {
        return s.projectCache.GetBackend(projectID)  // route to project DB
    }
    if projectID != "" && s.projectCache == nil {
        return nil, fmt.Errorf("project_id specified but no project cache configured")
    }
    return s.backend, nil  // default backend
}
```

**4. `getWorkDir(projectID)`** (`config_server.go:75`) -- resolves filesystem path via `projectCache.GetProjectPath()`. Same routing logic as `getBackend` but returns a directory path. Used by services needing filesystem access (config files, prompts, agents, scripts) -- not just DB access.

**5. Usage in RPC methods** -- extract `project_id` from proto request:
```go
backend, err := s.getBackend(req.Msg.GetProjectId())
if err != nil { return nil, err }
// use backend instead of s.backend
```

**Error behavior -- no silent fallbacks:**

| Condition | Result |
|-----------|--------|
| `project_id` empty | Uses default `s.backend` / `s.workDir` |
| `project_id` set, cache exists | Routes to project-specific backend/path via LRU cache |
| `project_id` set, cache nil | **Error** (not fallback to default) |
| Project not in registry | **Error** from `ProjectCache.GetBackend()` / `GetProjectPath()` |

**ProjectCache** (`project_cache.go`): Thread-safe LRU cache mapping project IDs to `storage.Backend` instances. Opens databases on demand, evicts least-recently-used when at capacity (default: 10). Proto request messages across all `.proto` files include `project_id` as an optional field.

### HTTP File Routes (Non-RPC)

`file_handlers.go:55` provides `resolveProjectBackend(r *http.Request)` for non-RPC HTTP routes (attachments, screenshots, traces, export/import). Reads `project_id` from query params instead of proto messages:

```go
func (s *Server) resolveProjectBackend(r *http.Request) (storage.Backend, string, error) {
    projectID := r.URL.Query().Get("project_id")
    // returns (backend, workDir, error) -- same routing logic as getBackend/getWorkDir
}
```

### Project-Aware Server Methods

`server.go` task lifecycle methods accept `(id, projectID string)` and resolve backend/workDir from `projectCache` when projectID is non-empty:

| Method | Signature | Location |
|--------|-----------|----------|
| `startTask` | `(id, projectID string) error` | `server.go:757` |
| `pauseTask` | `(id, projectID string) (map[string]any, error)` | `server.go:600` |
| `resumeTask` | `(id, projectID string) (map[string]any, error)` | `server.go:627` |
| `cancelTask` | `(id, projectID string) (map[string]any, error)` | `server.go:827` |
| `GetSessionMetrics` | `(projectID string) SessionMetricsResponse` | `server.go:881` |

`TaskExecutorFunc` (`task_server.go:30`) signature: `func(taskID, projectID string) error`. This is how project context flows from Connect RPC through to `Server.startTask`.

WebSocket callers pass `""` for projectID (backwards compat with single-project mode).

### Finalize Tracker Project Support

`finalize_tracker.go:182`: `TriggerFinalizeOnApproval(taskID, projectID string)` resolves backend/workDir from projectCache. `runFinalizeAsync` (`finalize_tracker.go:256`) accepts pre-resolved `backend` and `workDir` parameters. The PR poller passes `""` since it operates on the default backend.

### Error Handling

Interceptors map internal errors to Connect codes (`interceptors.go:56-117`):

| Internal Error | Connect Code |
|---------------|--------------|
| Task not found | `NotFound` |
| Validation error | `InvalidArgument` |
| Task already running | `FailedPrecondition` |
| Claude timeout | `DeadlineExceeded` |
| Default | `Internal` |

### Branch Control Validation (`task_server.go`)

`CreateTask` and `UpdateTask` validate branch control fields:

| Validation | Location | Error Code |
|------------|----------|------------|
| `branch_name` format | `git.ValidateBranchName()` | `InvalidArgument` |
| `target_branch` format | `git.ValidateBranchName()` | `InvalidArgument` |
| `branch_name` change while RUNNING | `task_server.go:426` | `FailedPrecondition` |

`UpdateTask` uses `*_set` sentinel fields (`PrLabelsSet`, `PrReviewersSet`) to distinguish "set to empty" from "not provided" -- setting `*_set=false` clears the override.

### Server Streaming (Events)

`EventService.Subscribe` (`event_server.go:60-136`) provides real-time events via server streaming. Clients filter by task ID, initiative ID, or event types. Heartbeat support included.

## WebSocket Protocol (Legacy)

WebSocket handler at `/api/ws` (`websocket.go`) remains for backward compatibility. Supports `subscribe`/`unsubscribe`/`command`/`ping` client messages; forwards `event`/`subscribed`/`pong` server messages.

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
