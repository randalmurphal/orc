# API Package

REST API server with WebSocket support for real-time updates.

## Architecture

```
Server
├── Routes (chi router) → handlers_*.go (19 handler files)
├── WebSocket Hub → Client connections, subscriptions
├── PR Poller → Background PR status updates
├── Finalize Tracker → In-memory state + cancellation
└── Event Publisher → Real-time task updates
```

## Key Patterns

### Response Helpers

```go
s.jsonResponse(w, data)              // Success
s.handleOrcError(w, err)             // Error with status
s.jsonError(w, "msg", http.StatusBadRequest)
```

### Handler Methods

```go
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
    // All handlers are Server methods
}
```

### Auto-Commit

API handlers auto-commit .orc/ mutations:

| Helper | Used By |
|--------|---------|
| `autoCommitTask(t, action)` | Task CRUD |
| `autoCommitConfig(desc)` | Config handlers |
| `autoCommitPrompt(phase, action)` | Prompt handlers |
| `autoCommitInitiative(init, action)` | Initiative handlers |

Respects `tasks.disable_auto_commit` config.

### Scope Parameter

Claude Code endpoints support `?scope=global` for `~/.claude/`:

```
/api/skills?scope=global
/api/hooks?scope=global
/api/agents?scope=global
```

## WebSocket Protocol

### Client Messages

```json
{"type": "subscribe", "task_id": "TASK-001"}
{"type": "subscribe", "task_id": "*"}
{"type": "unsubscribe"}
{"type": "ping"}
```

### Server Messages

```json
{"type": "subscribed", "task_id": "TASK-001"}
{"type": "event", "event_type": "task_updated", "data": {...}}
{"type": "event", "event_type": "transcript", "data": {...}}
{"type": "pong"}
```

### Events

| Event | Data |
|-------|------|
| `task_created` | `{task: Task}` |
| `task_updated` | `{task: Task}` |
| `task_deleted` | `{task_id: string}` |
| `state` | `{raw: string}` |
| `finalize` | `{task_id, status, step}` |

## PR Status Polling

Background poller (60s interval, 30s rate limit per task):
1. Find tasks with open PRs
2. Get PR reviews and CI status
3. Update task with derived status
4. Trigger finalize on approval (auto profile)

**Status derivation:** MERGED > CLOSED > Draft > changes_requested > approved > pending_review

## Startup Tasks

Server performs housekeeping on startup:
1. `pruneStaleWorktrees()` - Remove stale git worktree entries (directories deleted without `git worktree remove`)

## Graceful Shutdown

Server manages background goroutines via `serverCtx`:
1. `serverCtxCancel()` - Signal stop
2. `finTracker.cancelAll()` - Cancel finalize ops
3. `prPoller.Stop()` - Stop polling (idempotent via sync.Once)

## Testing

```bash
mkdir -p internal/api/static  # Required for go:embed
go test ./internal/api/... -v
```

Or use `make test` which handles prerequisites.

## Reference

See [ENDPOINTS.md](ENDPOINTS.md) for full endpoint documentation.
