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

CWD-based task operations. These endpoints use the server's working directory as the project root.

**Note:** When the server is started from a non-orc directory, `/api/tasks` returns an empty list rather than an error. For explicit project-scoped operations, use `/api/projects/:id/tasks` instead.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks` | List tasks (`?page=N&limit=N`) - returns empty list if not in orc project |
| POST | `/api/tasks` | Create task |
| GET | `/api/tasks/:id` | Get task |
| PATCH | `/api/tasks/:id` | Update task (title, description, weight, metadata) |
| DELETE | `/api/tasks/:id` | Delete task |
| GET | `/api/tasks/:id/state` | Get execution state |
| GET | `/api/tasks/:id/plan` | Get task plan |
| GET | `/api/tasks/:id/transcripts` | Get transcripts |
| POST | `/api/tasks/:id/run` | Start task |
| POST | `/api/tasks/:id/pause` | Pause task |
| POST | `/api/tasks/:id/resume` | Resume task |
| POST | `/api/tasks/:id/rewind` | Rewind to phase (`{"phase": "implement"}`) |

**Run task response:**
```json
{
  "status": "started",
  "task_id": "TASK-001",
  "task": { /* full Task object with status="running" */ }
}
```

The `task` field contains the updated task with `status: "running"` set immediately. This allows clients to update their local state without waiting for WebSocket events, avoiding race conditions where the task might briefly appear deleted.

**Create task body (POST):**
```json
{
  "title": "Task title",
  "description": "Task description",
  "weight": "medium",
  "queue": "active",
  "priority": "normal"
}
```

All fields except `title` are optional. Defaults: `queue: "active"`, `priority: "normal"`.

**Update task body (PATCH):**
```json
{
  "title": "New title",
  "description": "Updated description",
  "weight": "medium",
  "queue": "backlog",
  "priority": "high",
  "metadata": {"key": "value"}
}
```

All fields are optional. Only provided fields are updated. Cannot update running tasks.

| Field | Valid Values |
|-------|--------------|
| `weight` | `trivial`, `small`, `medium`, `large`, `greenfield` |
| `queue` | `active`, `backlog` |
| `priority` | `critical`, `high`, `normal`, `low` |

Weight changes trigger automatic plan regeneration (completed/skipped phases are preserved if they exist in both plans).

### Task Attachments

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks/:id/attachments` | List attachments |
| POST | `/api/tasks/:id/attachments` | Upload attachment (multipart/form-data, max 32MB) |
| GET | `/api/tasks/:id/attachments/:filename` | Get attachment file |
| DELETE | `/api/tasks/:id/attachments/:filename` | Delete attachment |

**Attachment response:**
```json
{
  "filename": "screenshot.png",
  "size": 45678,
  "content_type": "image/png",
  "created_at": "2026-01-10T10:30:00Z",
  "is_image": true
}
```

**Upload:** Use `multipart/form-data` with file in the `file` field. Optional `filename` field overrides original filename.

**Download headers:**
- Images: `Content-Disposition: inline` (renders in browser)
- Other files: `Content-Disposition: attachment` (triggers download)

### Task Export

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tasks/:id/export` | Export artifacts |
| GET | `/api/tasks/:id/stream` | SSE transcript stream (legacy) |

**Export body:**
```json
{"task_definition": true, "final_state": true, "context_summary": true, "transcripts": false}
```

### Task Comments

Comments and notes on tasks from humans, agents, or system.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks/:id/comments` | List comments (`?author_type=human|agent|system`, `?phase=implement`) |
| POST | `/api/tasks/:id/comments` | Create comment |
| GET | `/api/tasks/:id/comments/stats` | Get comment statistics |
| GET | `/api/tasks/:id/comments/:commentId` | Get single comment |
| PUT | `/api/tasks/:id/comments/:commentId` | Update comment |
| DELETE | `/api/tasks/:id/comments/:commentId` | Delete comment |

**Create comment body:**
```json
{
  "author": "claude",
  "author_type": "agent",
  "content": "This approach uses the existing auth flow",
  "phase": "implement"
}
```

**Comment response:**
```json
{
  "id": "TC-a1b2c3d4",
  "task_id": "TASK-001",
  "author": "claude",
  "author_type": "agent",
  "content": "This approach uses the existing auth flow",
  "phase": "implement",
  "created_at": "2026-01-10T10:30:00Z",
  "updated_at": "2026-01-10T10:30:00Z"
}
```

**Author types:**
- `human` - Human user (default)
- `agent` - AI agent (Claude during execution)
- `system` - System-generated (automated processes)

---

## Projects

Multi-project support via global registry.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects` | List registered projects |
| GET | `/api/projects/default` | Get default project ID |
| PUT | `/api/projects/default` | Set default project |
| GET | `/api/projects/:id` | Get project details |
| GET | `/api/projects/:id/tasks` | List tasks for project |
| POST | `/api/projects/:id/tasks` | Create task in project |

### Default Project

Fallback project when no selection exists in URL or localStorage. Stored in `~/.orc/projects.yaml`.

**Get default project:**
```json
// Response
{"default_project": "abc123"}
```

**Set default project:**
```json
// Request
{"project_id": "abc123"}

// Response
{"default_project": "abc123"}
```

Returns 404 if the specified project doesn't exist.

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

### Test Results (Playwright)

Endpoints for Playwright test results, screenshots, and traces. Test results are stored in `.orc/tasks/{id}/test-results/`.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks/:id/test-results` | Get test results summary |
| POST | `/api/tasks/:id/test-results` | Save test report |
| POST | `/api/tasks/:id/test-results/init` | Initialize test results directory |
| GET | `/api/tasks/:id/test-results/screenshots` | List all screenshots |
| POST | `/api/tasks/:id/test-results/screenshots` | Upload screenshot (multipart/form-data) |
| GET | `/api/tasks/:id/test-results/screenshots/:filename` | Get screenshot file |
| GET | `/api/tasks/:id/test-results/report` | Get Playwright HTML report |
| GET | `/api/tasks/:id/test-results/traces/:filename` | Get trace file |

**Test results response:**
```json
{
  "has_results": true,
  "report": {
    "version": 1,
    "framework": "playwright",
    "started_at": "2026-01-10T10:30:00Z",
    "completed_at": "2026-01-10T10:35:00Z",
    "duration": 300000,
    "summary": {
      "total": 10,
      "passed": 9,
      "failed": 1,
      "skipped": 0
    },
    "suites": [...]
  },
  "screenshots": [
    {
      "filename": "dashboard-initial.png",
      "page_name": "dashboard initial",
      "size": 45678,
      "created_at": "2026-01-10T10:32:00Z"
    }
  ],
  "has_traces": true,
  "trace_files": ["trace-1.zip"],
  "has_html_report": true
}
```

**Save test report body:**
```json
{
  "version": 1,
  "framework": "playwright",
  "started_at": "2026-01-10T10:30:00Z",
  "completed_at": "2026-01-10T10:35:00Z",
  "duration": 300000,
  "summary": {
    "total": 10,
    "passed": 10,
    "failed": 0,
    "skipped": 0
  },
  "suites": [
    {
      "name": "Login Flow",
      "tests": [
        {
          "name": "should login successfully",
          "status": "passed",
          "duration": 1500,
          "screenshots": ["login-success.png"]
        }
      ]
    }
  ]
}
```

**Screenshot upload:** Use `multipart/form-data` with file in the `file` field. Optional `filename` field overrides original filename.

### Dashboard

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/dashboard/stats` | Get dashboard statistics |

**Dashboard stats response:**
```json
{
  "running": 1,
  "paused": 0,
  "blocked": 2,
  "completed": 15,
  "failed": 1,
  "today": 3,
  "total": 19,
  "tokens": 245000,
  "cache_creation_input_tokens": 5000,
  "cache_read_input_tokens": 120000,
  "cost": 12.50
}
```

| Field | Description |
|-------|-------------|
| `tokens` | Total input + output tokens |
| `cache_creation_input_tokens` | Tokens written to prompt cache (aggregated) |
| `cache_read_input_tokens` | Tokens served from prompt cache (aggregated) |
| `cost` | Estimated cost in USD |

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
| `transcript` | `TranscriptEvent` | Streaming conversation (see below) |
| `tokens` | `TokenUpdate` | Token usage (includes cached tokens) |
| `complete` | `{status, duration}` | Task finished |
| `error` | `{message, fatal}` | Error occurred |
| `task_created` | `{task: Task}` | Task created via CLI/filesystem |
| `task_updated` | `{task: Task}` | Task modified via CLI/filesystem |
| `task_deleted` | `{task_id: string}` | Task deleted via CLI/filesystem |

### Transcript Event Types

The `transcript` event supports real-time streaming of Claude's output:

```json
// Streaming chunk (sent as response generates)
{
  "type": "chunk",
  "content": "partial response text",
  "phase": "implement",
  "iteration": 1
}

// Complete response (sent when response finishes)
{
  "type": "response",
  "content": "full response text",
  "phase": "implement",
  "iteration": 1
}
```

**Client handling:**
- `chunk` events append to streaming buffer; reset buffer when phase/iteration changes
- `response` events signal completion; reload transcript files from API
- Use `getTranscripts(taskId)` or `getProjectTranscripts(projectId, taskId)` to fetch saved transcripts

### TokenUpdate Schema

```json
{
  "input_tokens": 1500,
  "output_tokens": 500,
  "cache_creation_input_tokens": 200,
  "cache_read_input_tokens": 12000,
  "total_tokens": 14200
}
```

| Field | Description |
|-------|-------------|
| `input_tokens` | Uncached input tokens |
| `output_tokens` | Generated output tokens |
| `cache_creation_input_tokens` | Tokens written to prompt cache (optional) |
| `cache_read_input_tokens` | Tokens served from prompt cache (optional) |
| `total_tokens` | Sum of all token types |

Tokens are incremental (add to existing totals, don't replace).

### Global Subscriptions

Subscribe to `"*"` to receive file watcher events for all tasks:

```json
{"type": "subscribe", "task_id": "*"}
```

File watcher events (`task_created`, `task_updated`, `task_deleted`) are only broadcast to global subscribers. These events are triggered when tasks are created, modified, or deleted outside the API (e.g., via CLI or direct filesystem edits).

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
