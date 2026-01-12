# API Reference

REST API endpoints for the orc orchestrator. Base URL: `http://localhost:8080`

## Quick Navigation

| Category | Endpoints | Purpose |
|----------|-----------|---------|
| [Tasks](#tasks-global) | `/api/tasks/*` | Task CRUD and execution |
| [Projects](#projects) | `/api/projects/*` | Multi-project task operations |
| [Initiatives](#initiatives) | `/api/initiatives/*` | Task grouping and decisions |
| [Configuration](#configuration) | `/api/prompts/*`, `/api/hooks/*`, etc. | Project configuration |
| [Integration](#integration) | `/api/github/*`, `/api/mcp/*` | External integrations |
| [Real-time](#websocket-protocol) | `/api/ws` | WebSocket events |

---

## Tasks (Global)

CWD-based task operations.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks` | List tasks (`?page=N&limit=N`) |
| POST | `/api/tasks` | Create task |
| GET | `/api/tasks/:id` | Get task |
| DELETE | `/api/tasks/:id` | Delete task |
| GET | `/api/tasks/:id/state` | Get execution state |
| GET | `/api/tasks/:id/plan` | Get task plan |
| GET | `/api/tasks/:id/transcripts` | Get transcripts |
| POST | `/api/tasks/:id/run` | Start task |
| POST | `/api/tasks/:id/pause` | Pause task |
| POST | `/api/tasks/:id/resume` | Resume task |
| POST | `/api/tasks/:id/rewind` | Rewind to phase (`{"phase": "implement"}`) |

### Task Export

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tasks/:id/export` | Export artifacts |
| GET | `/api/tasks/:id/stream` | SSE transcript stream (legacy) |

**Export body:**
```json
{"task_definition": true, "final_state": true, "context_summary": true, "transcripts": false}
```

---

## Projects

Multi-project support via global registry.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects` | List registered projects |
| GET | `/api/projects/:id` | Get project details |
| GET | `/api/projects/:id/tasks` | List tasks for project |
| POST | `/api/projects/:id/tasks` | Create task in project |

### Project Task Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects/:id/tasks/:taskId` | Get task |
| DELETE | `/api/projects/:id/tasks/:taskId` | Delete task |
| GET | `/api/projects/:id/tasks/:taskId/state` | Get execution state |
| GET | `/api/projects/:id/tasks/:taskId/plan` | Get task plan |
| GET | `/api/projects/:id/tasks/:taskId/transcripts` | Get transcripts |
| POST | `/api/projects/:id/tasks/:taskId/run` | Start task |
| POST | `/api/projects/:id/tasks/:taskId/pause` | Pause task |
| POST | `/api/projects/:id/tasks/:taskId/resume` | Resume task |
| POST | `/api/projects/:id/tasks/:taskId/rewind` | Rewind to phase |

---

## Initiatives

Group related tasks with shared decisions.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/initiatives` | List (`?status=active`, `?shared=true`) |
| POST | `/api/initiatives` | Create initiative |
| GET | `/api/initiatives/:id` | Get initiative |
| PUT | `/api/initiatives/:id` | Update initiative |
| DELETE | `/api/initiatives/:id` | Delete initiative |
| GET | `/api/initiatives/:id/tasks` | List initiative tasks |
| POST | `/api/initiatives/:id/tasks` | Add task to initiative |
| POST | `/api/initiatives/:id/decisions` | Add decision |
| GET | `/api/initiatives/:id/ready` | Get tasks ready to run |

---

## Configuration

### Prompts

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/prompts` | List prompts |
| GET | `/api/prompts/variables` | Get template variables |
| GET | `/api/prompts/:phase` | Get prompt for phase |
| GET | `/api/prompts/:phase/default` | Get default prompt |
| PUT | `/api/prompts/:phase` | Save prompt override |
| DELETE | `/api/prompts/:phase` | Delete prompt override |

### Hooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/hooks` | List all hooks (map of event to hooks) |
| GET | `/api/hooks/types` | Get valid hook event types |
| POST | `/api/hooks` | Create hook (event + matcher + command) |
| GET | `/api/hooks/:event` | Get hooks for event type |
| PUT | `/api/hooks/:event` | Update hooks for event |
| DELETE | `/api/hooks/:event` | Delete all hooks for event |

### Skills

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/skills` | List skills |
| POST | `/api/skills` | Create skill (name, description, content) |
| GET | `/api/skills/:name` | Get skill with content |
| PUT | `/api/skills/:name` | Update skill |
| DELETE | `/api/skills/:name` | Delete skill |

### Settings

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/settings` | Get merged settings (global + project) |
| GET | `/api/settings/project` | Get project settings only |
| PUT | `/api/settings` | Update project settings |

### Tools

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tools` | List available tools |
| GET | `/api/tools?by_category=true` | List tools grouped by category |
| GET | `/api/tools/permissions` | Get tool allow/deny lists |
| PUT | `/api/tools/permissions` | Update tool permissions |

### Agents

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/agents` | List sub-agents |
| POST | `/api/agents` | Create sub-agent |
| GET | `/api/agents/:name` | Get agent details |
| PUT | `/api/agents/:name` | Update agent |
| DELETE | `/api/agents/:name` | Delete agent |

### Scripts

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/scripts` | List registered scripts |
| POST | `/api/scripts` | Register script |
| POST | `/api/scripts/discover` | Auto-discover scripts |
| GET | `/api/scripts/:name` | Get script details |
| PUT | `/api/scripts/:name` | Update script |
| DELETE | `/api/scripts/:name` | Remove script from registry |

### CLAUDE.md

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/claudemd` | Get project CLAUDE.md |
| PUT | `/api/claudemd` | Update project CLAUDE.md |
| GET | `/api/claudemd/hierarchy` | Get full hierarchy (global, user, project) |

### Orc Config

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/config` | Get orc configuration |
| PUT | `/api/config` | Update orc configuration |
| GET | `/api/config/export` | Get export configuration |
| PUT | `/api/config/export` | Update export configuration |

---

## Integration

### GitHub PR

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tasks/:id/github/pr` | Create PR for task branch |
| GET | `/api/tasks/:id/github/pr` | Get PR details, comments, checks |
| POST | `/api/tasks/:id/github/pr/merge` | Merge PR |
| POST | `/api/tasks/:id/github/pr/comments/sync` | Sync local comments to PR |
| POST | `/api/tasks/:id/github/pr/comments/:commentId/autofix` | Queue auto-fix |
| GET | `/api/tasks/:id/github/pr/checks` | Get CI check status |

**Merge body:**
```json
{"method": "squash", "delete_branch": true}
```

### MCP Servers

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/mcp` | List MCP servers from .mcp.json |
| POST | `/api/mcp` | Create MCP server |
| GET | `/api/mcp/:name` | Get MCP server details |
| PUT | `/api/mcp/:name` | Update MCP server |
| DELETE | `/api/mcp/:name` | Delete MCP server |

### Cost Tracking

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/cost/summary` | Get cost summary (`?period=day|week|month|all`) |

---

## Knowledge

Project knowledge queue (patterns, gotchas, decisions).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/knowledge` | List entries (`?status=pending`, `?type=pattern`) |
| GET | `/api/knowledge/status` | Get queue statistics |
| GET | `/api/knowledge/stale` | List stale entries (`?days=90`) |
| POST | `/api/knowledge` | Create knowledge entry |
| GET | `/api/knowledge/:id` | Get entry |
| POST | `/api/knowledge/:id/approve` | Approve entry |
| POST | `/api/knowledge/:id/reject` | Reject entry |
| POST | `/api/knowledge/:id/validate` | Validate (reset staleness) |
| DELETE | `/api/knowledge/:id` | Delete entry |
| POST | `/api/knowledge/approve-all` | Approve all pending |

---

## WebSocket Protocol

Connect to `/api/ws` for real-time updates.

### Client Messages

```json
{"type": "subscribe", "task_id": "TASK-001"}
{"type": "unsubscribe"}
{"type": "command", "task_id": "TASK-001", "action": "pause"}
{"type": "ping"}
```

### Server Messages

```json
{"type": "subscribed", "task_id": "TASK-001"}
{"type": "event", "event_type": "state", "data": {...}}
{"type": "event", "event_type": "transcript", "data": {...}}
{"type": "event", "event_type": "phase", "data": {...}}
{"type": "pong"}
```

### Event Types

| Event | Data | Purpose |
|-------|------|---------|
| `state` | `TaskState` | Full task state update |
| `phase` | `{phase, status}` | Phase started/completed/failed |
| `transcript` | `TranscriptLine` | Streaming conversation |
| `tokens` | `TokenUpdate` | Token usage |
| `complete` | `{status, duration}` | Task finished |
| `error` | `{message, fatal}` | Error occurred |

---

## Error Responses

All errors return JSON:

```json
{
  "error": "error message",
  "code": "ERROR_CODE",
  "details": {}
}
```

Common status codes:
- `400` - Bad request (invalid parameters)
- `404` - Resource not found
- `409` - Conflict (e.g., task already running)
- `500` - Internal server error
