# API Package

REST API server with WebSocket support for real-time updates.

## File Structure

| File | Purpose | Lines |
|------|---------|-------|
| `server.go` | Server setup, routing, core methods | ~400 |
| `websocket.go` | WebSocket connection handling | ~280 |
| `middleware.go` | CORS middleware | ~50 |
| `response.go` | JSON response helpers, error handling | ~80 |

### Handler Files (18 total)

| File | Endpoints | Description |
|------|-----------|-------------|
| `handlers_tasks.go` | `/api/tasks/*` | Task CRUD operations |
| `handlers_attachments.go` | `attachments` | Task file attachments (upload, download, delete) |
| `handlers_tasks_control.go` | `run`, `pause`, `resume` | Task execution control |
| `handlers_tasks_state.go` | `state`, `plan`, `transcripts`, `stream` | Task state and streaming |
| `handlers_projects.go` | `/api/projects/*` | Project-scoped task operations |
| `handlers_prompts.go` | `/api/prompts/*` | Prompt template management |
| `handlers_hooks.go` | `/api/hooks/*` | Hook configuration |
| `handlers_skills.go` | `/api/skills/*` | Skill management (SKILL.md format) |
| `handlers_settings.go` | `/api/settings/*` | Settings management |
| `handlers_tools.go` | `/api/tools/*` | Tool permissions |
| `handlers_agents.go` | `/api/agents/*` | Sub-agent management |
| `handlers_scripts.go` | `/api/scripts/*` | Script registry |
| `handlers_claudemd.go` | `/api/claudemd/*` | CLAUDE.md hierarchy |
| `handlers_mcp.go` | `/api/mcp/*` | MCP server configuration |
| `handlers_templates.go` | `/api/templates/*` | Template management |
| `handlers_config.go` | `/api/config/*` | Orc configuration |
| `handlers_dashboard.go` | `/api/dashboard/*` | Dashboard statistics |
| `handlers_diff.go` | `/api/tasks/:id/diff/*` | Git diff visualization for task changes |
| `handlers_github.go` | `/api/tasks/:id/github/*` | GitHub PR operations and status |
| `handlers_initiatives.go` | `/api/initiatives/*` | Initiative management |

### Background Services

| File | Purpose |
|------|---------|
| `pr_poller.go` | Background PR status polling (see [PR Status Polling](#pr-status-polling)) |

## Server Architecture

```
Server
├── Routes (chi router)
│   ├── /api/tasks/* → handlers_tasks*.go, handlers_attachments.go, handlers_diff.go, handlers_github.go
│   ├── /api/initiatives/* → handlers_initiatives.go
│   ├── /api/projects/* → handlers_projects.go
│   ├── /api/prompts/* → handlers_prompts.go
│   ├── /api/hooks/* → handlers_hooks.go
│   ├── /api/skills/* → handlers_skills.go
│   ├── /api/settings/* → handlers_settings.go
│   ├── /api/tools/* → handlers_tools.go
│   ├── /api/agents/* → handlers_agents.go
│   ├── /api/scripts/* → handlers_scripts.go
│   ├── /api/claudemd/* → handlers_claudemd.go
│   ├── /api/mcp/* → handlers_mcp.go
│   ├── /api/templates/* → handlers_templates.go
│   ├── /api/config/* → handlers_config.go
│   ├── /api/dashboard/* → handlers_dashboard.go
│   └── /api/ws → websocket.go
├── WebSocket Hub
│   └── Client connections, subscriptions
├── PR Poller
│   └── Background PR status updates
└── Event Publisher
    └── Real-time task updates
```

## Key Patterns

### Response Helpers (response.go)

```go
// Success response
s.jsonResponse(w, data)

// Error response with proper status code
s.handleOrcError(w, err)  // Inspects error type for status
s.jsonError(w, "message", http.StatusBadRequest)
```

### Handler Methods

All handlers are methods on `*Server`:
```go
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
    // Handler implementation
}
```

### CWD-Based Task Operations

The `/api/tasks/*` endpoints operate on the server's working directory. If the server is started from a non-orc directory:
- `GET /api/tasks` returns an empty list (not an error)
- Create/modify operations will fail

**Recommendation:** Use project-scoped endpoints (`/api/projects/:id/tasks/*`) for explicit project targeting.

### Project-Scoped Operations

Project handlers resolve project path and delegate to task handlers:
```go
func (s *Server) handleProjectListTasks(w http.ResponseWriter, r *http.Request) {
    projectID := chi.URLParam(r, "id")
    project, err := s.projectRegistry.Get(projectID)
    // Use project.Path for task operations
}
```

### Safe Type Assertions

ResponseWriter flushing uses safe assertions:
```go
if f, ok := w.(http.Flusher); ok {
    f.Flush()
}
```

### Scope Parameter

Claude Code endpoints support `?scope=global` for user-level config (`~/.claude/`):
- `/api/skills?scope=global` - Global skills from `~/.claude/skills/`
- `/api/hooks?scope=global` - Global hooks from `~/.claude/settings.json`
- `/api/agents?scope=global` - Global agents from `~/.claude/agents/*.md`
- `/api/mcp?scope=global` - Global MCP from `~/.claude/.mcp.json`
- `/api/claudemd?scope=global` - Global CLAUDE.md

Without scope parameter, endpoints return project-level config (`.claude/`).

## WebSocket Protocol

### Client Messages
```json
{"type": "subscribe", "task_id": "TASK-001"}
{"type": "subscribe", "task_id": "*"}              // Global subscription (all tasks)
{"type": "unsubscribe"}
{"type": "command", "task_id": "TASK-001", "action": "pause"}
{"type": "ping"}
```

### Server Messages
```json
{"type": "subscribed", "task_id": "TASK-001"}
{"type": "event", "event_type": "state", "data": {...}}
{"type": "event", "event_type": "transcript", "data": {...}}
{"type": "event", "event_type": "task_created", "data": {"task": {...}}}
{"type": "event", "event_type": "task_updated", "data": {"task": {...}}}
{"type": "event", "event_type": "task_deleted", "data": {"task_id": "TASK-001"}}
{"type": "pong"}
```

### File Watcher Events

The API server runs a file watcher that monitors `.orc/tasks/` for changes made outside the API (CLI, filesystem). Events are published to WebSocket clients subscribed to `"*"`:

| Event | Trigger | Data |
|-------|---------|------|
| `task_created` | New `task.yaml` detected | `{task: Task}` |
| `task_updated` | `task.yaml`, `plan.yaml`, or `spec.md` modified | `{task: Task}` |
| `task_deleted` | Task directory removed (verified) | `{task_id: string}` |
| `state` | `state.yaml` modified | `{raw: string}` |

**Flow:** CLI/filesystem change → file watcher → debounce (500ms) → content hash check → publish event → WebSocket broadcast

## PR Status Polling

The server runs a background poller that monitors PR status for tasks with open PRs.

### How It Works

```
PRPoller (60s interval)
├── Load all tasks
├── Filter tasks with open PRs (not merged/closed)
├── For each task:
│   ├── Find PR by branch name
│   ├── Get reviews (track latest per author)
│   ├── Get CI check runs
│   └── Update task.yaml with status
└── Trigger callback on status change
```

### Configuration

| Setting | Default | Description |
|---------|---------|-------------|
| Poll interval | 60s | Time between polling cycles |
| Rate limit | 30s | Minimum time between polls for same task |

### Status Derivation

PR status is derived from GitHub PR state and reviews:

```go
DeterminePRStatus(pr, summary):
  if pr.State == "MERGED"     → PRStatusMerged
  if pr.State == "CLOSED"     → PRStatusClosed
  if pr.Draft                 → PRStatusDraft
  if summary has changes_requested → PRStatusChangesRequested
  if summary has approvals    → PRStatusApproved
  else                        → PRStatusPendingReview
```

### Manual Refresh

Trigger immediate refresh via `POST /api/tasks/:id/github/pr/refresh`.

### Status Change Callback

When PR status changes, the poller triggers `onStatusChange(taskID, prInfo)` which can publish WebSocket events for real-time UI updates.

## Testing

**Prerequisite**: The API package uses `go:embed` for static files. Before running tests directly:
```bash
mkdir -p internal/api/static
echo "# Placeholder for go:embed" > internal/api/static/.gitkeep
```

Or use `make test` which handles this automatically.

```bash
# Run API tests
go test ./internal/api/... -v

# Test specific handler
go test ./internal/api/... -run TestHandlerName -v
```

Test files:
- `server_test.go` - Integration tests for endpoints
- `middleware_test.go` - CORS middleware tests
- `response_test.go` - Response helper tests
- `websocket_test.go` - WebSocket protocol tests
