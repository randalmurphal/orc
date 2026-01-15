# API Reference

REST API endpoints for the orc orchestrator. Base URL: `http://localhost:8080`

## Quick Navigation

| Category | Endpoints | Purpose |
|----------|-----------|---------|
| [Tasks](#tasks-global) | `/api/tasks/*` | Task CRUD and execution |
| [Projects](#projects) | `/api/projects/*` | Multi-project task operations |
| [Initiatives](#initiatives) | `/api/initiatives/*` | Task grouping and decisions |
| [Configuration](#configuration) | `/api/prompts/*`, `/api/hooks/*`, etc. | Project configuration |
| [Integration](#integration) | `/api/github/*`, `/api/mcp/*`, `/api/plugins/*` | External integrations |
| [Plugins](#plugins) | `/api/plugins/*`, `/api/marketplace/*` | Plugin management & marketplace |
| [Real-time](#websocket-protocol) | `/api/ws` | WebSocket events |

---

## Tasks (Global)

CWD-based task operations. These endpoints use the server's working directory as the project root.

**Note:** When the server is started from a non-orc directory, `/api/tasks` returns an empty list rather than an error. For explicit project-scoped operations, use `/api/projects/:id/tasks` instead.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks` | List tasks (`?page=N&limit=N&initiative=INIT-001&dependency_status=blocked`) - returns empty list if not in orc project |
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
| POST | `/api/tasks/:id/finalize` | Trigger finalize phase (async) |
| GET | `/api/tasks/:id/finalize` | Get finalize status |

**Run task response:**
```json
{
  "status": "started",
  "task_id": "TASK-001",
  "task": { /* full Task object with status="running" */ }
}
```

The `task` field contains the updated task with `status: "running"` set immediately. This allows clients to update their local state without waiting for WebSocket events, avoiding race conditions where the task might briefly appear deleted.

**Blocked task response (409 Conflict):**

If a task has incomplete dependencies (tasks in `blocked_by` that aren't completed), the endpoint returns 409 Conflict:

```json
{
  "error": "task_blocked",
  "message": "Task is blocked by incomplete dependencies",
  "blocked_by": [
    {"id": "TASK-060", "title": "Add initiative_id field", "status": "planned"},
    {"id": "TASK-061", "title": "Add Initiatives section", "status": "running"}
  ],
  "force_available": true
}
```

| Field | Description |
|-------|-------------|
| `error` | Error code: `task_blocked` |
| `message` | Human-readable error message |
| `blocked_by` | Array of incomplete blocking tasks with id, title, status |
| `force_available` | Always `true` - indicates override is possible |

**Force override:** Add `?force=true` query parameter to run the task despite blockers:
```
POST /api/tasks/TASK-062/run?force=true
```

**Create task body (POST):**

Supports both JSON and multipart/form-data. Use multipart when attaching files during creation.

*JSON format:*
```json
{
  "title": "Task title",
  "description": "Task description",
  "weight": "medium",
  "queue": "active",
  "priority": "normal",
  "category": "feature",
  "initiative_id": "INIT-001",
  "blocked_by": ["TASK-001", "TASK-002"],
  "related_to": ["TASK-003"]
}
```

*Multipart form-data fields:*
| Field | Type | Description |
|-------|------|-------------|
| `title` | string | Task title (required) |
| `description` | string | Task description |
| `weight` | string | trivial/small/medium/large/greenfield |
| `queue` | string | active/backlog |
| `priority` | string | critical/high/normal/low |
| `category` | string | feature/bug/refactor/chore/docs/test |
| `initiative_id` | string | Initiative ID to link task to (e.g., INIT-001) |
| `blocked_by` | string | Comma-separated task IDs that must complete first |
| `related_to` | string | Comma-separated related task IDs |
| `attachments` | file[] | Files to attach (repeatable) |

All fields except `title` are optional. Defaults: `queue: "active"`, `priority: "normal"`, `category: "feature"`, `initiative_id: ""` (standalone).

**Query parameters for GET `/api/tasks`:**
| Parameter | Description | Values |
|-----------|-------------|--------|
| `page` | Page number for pagination | Integer |
| `limit` | Items per page (max 100) | Integer |
| `initiative` | Filter by initiative ID | Initiative ID (e.g., `INIT-001`) |
| `dependency_status` | Filter by dependency status | `blocked`, `ready`, `none` |

**Dependency status values:**
| Value | Description |
|-------|-------------|
| `blocked` | Tasks with incomplete blockers (waiting on other tasks) |
| `ready` | Tasks where all dependencies are satisfied |
| `none` | Tasks with no dependencies defined |

**Update task body (PATCH):**
```json
{
  "title": "New title",
  "description": "Updated description",
  "weight": "medium",
  "queue": "backlog",
  "priority": "high",
  "category": "bug",
  "initiative_id": "INIT-001",
  "blocked_by": ["TASK-001"],
  "related_to": ["TASK-002", "TASK-003"],
  "metadata": {"key": "value"}
}
```

All fields are optional. Only provided fields are updated. Cannot update running tasks.

| Field | Valid Values |
|-------|--------------|
| `weight` | `trivial`, `small`, `medium`, `large`, `greenfield` |
| `queue` | `active`, `backlog` |
| `priority` | `critical`, `high`, `normal`, `low` |
| `category` | `feature`, `bug`, `refactor`, `chore`, `docs`, `test` |
| `initiative_id` | Initiative ID (e.g., `INIT-001`) or `""` to unlink |
| `blocked_by` | Array of task IDs that must complete first |
| `related_to` | Array of related task IDs (informational) |

Weight changes trigger automatic plan regeneration (completed/skipped phases are preserved if they exist in both plans).

**Initiative linking:** Setting `initiative_id` links the task to an initiative. The task is auto-added to the initiative's task list (bidirectional sync). Use `""` to unlink a task from its initiative.

**Dependency validation:**
- Referenced task IDs must exist
- Self-references are rejected
- Circular dependencies are detected and rejected

### Task Finalize

Trigger and monitor the finalize phase, which syncs with the target branch, resolves conflicts, and runs tests.

**Trigger finalize (POST):**
```json
// Request body (optional)
{
  "force": false,         // Force finalize even if task status normally disallows it
  "gate_override": false  // Override gate checks
}

// Response
{
  "task_id": "TASK-001",
  "status": "pending",
  "message": "Finalize started"
}
```

The finalize runs asynchronously. Subscribe to WebSocket events for real-time progress updates.

**Finalize status (GET):**
```json
// While running
{
  "task_id": "TASK-001",
  "status": "running",
  "started_at": "2026-01-10T10:30:00Z",
  "updated_at": "2026-01-10T10:31:00Z",
  "step": "Syncing with target",
  "progress": "Merging changes from main",
  "step_percent": 50
}

// On completion
{
  "task_id": "TASK-001",
  "status": "completed",
  "started_at": "2026-01-10T10:30:00Z",
  "updated_at": "2026-01-10T10:35:00Z",
  "step": "Complete",
  "progress": "Finalize completed successfully",
  "step_percent": 100,
  "result": {
    "synced": true,
    "conflicts_resolved": 0,
    "conflict_files": [],
    "tests_passed": true,
    "risk_level": "low",
    "files_changed": 12,
    "lines_changed": 350,
    "needs_review": false,
    "commit_sha": "abc123def",
    "target_branch": "main"
  }
}

// On failure
{
  "task_id": "TASK-001",
  "status": "failed",
  "step": "Failed",
  "error": "merge conflict in main.go"
}

// Not started
{
  "task_id": "TASK-001",
  "status": "not_started",
  "message": "No finalize operation found"
}
```

| Status | Description |
|--------|-------------|
| `pending` | Finalize queued, about to start |
| `running` | Finalize in progress |
| `completed` | Finalize succeeded |
| `failed` | Finalize failed with error |
| `not_started` | No finalize has been triggered |

**FinalizeResult fields:**

| Field | Type | Description |
|-------|------|-------------|
| `synced` | boolean | Whether branch was synced with target |
| `conflicts_resolved` | number | Number of merge conflicts resolved |
| `conflict_files` | string[] | List of files that had conflicts |
| `tests_passed` | boolean | Whether tests passed after sync |
| `risk_level` | string | Risk assessment: `low`, `medium`, `high` |
| `files_changed` | number | Total files modified in diff |
| `lines_changed` | number | Total lines added/removed |
| `needs_review` | boolean | Whether human review is recommended |
| `commit_sha` | string | Final merged commit SHA |
| `target_branch` | string | Branch merged into |

**WebSocket events:** Finalize broadcasts `finalize` events during execution:
```json
// Progress update
{
  "type": "event",
  "event_type": "finalize",
  "data": {
    "task_id": "TASK-001",
    "status": "running",
    "step": "Running tests",
    "progress": "Executing test suite",
    "step_percent": 75,
    "updated_at": "2026-01-10T10:32:00Z"
  }
}

// Completion event (includes result)
{
  "type": "event",
  "event_type": "finalize",
  "data": {
    "task_id": "TASK-001",
    "status": "completed",
    "step": "Complete",
    "step_percent": 100,
    "updated_at": "2026-01-10T10:35:00Z",
    "result": {
      "synced": true,
      "conflicts_resolved": 0,
      "tests_passed": true,
      "risk_level": "low",
      "files_changed": 12,
      "commit_sha": "abc123def",
      "target_branch": "main"
    }
  }
}
```

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

### Task Dependencies

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks/:id/dependencies` | Get dependency graph for task |
| GET | `/api/tasks/dependency-graph` | Get visualization graph for multiple tasks |

**Dependencies response:**
```json
{
  "task_id": "TASK-001",
  "blocked_by": [
    {"id": "TASK-060", "title": "Setup auth", "status": "completed", "exists": true}
  ],
  "blocks": [
    {"id": "TASK-062", "title": "Add OAuth", "status": "planned", "exists": true}
  ],
  "related_to": [
    {"id": "TASK-063", "title": "Update docs", "status": "created", "exists": true}
  ],
  "referenced_by": [
    {"id": "TASK-064", "title": "See TASK-001 for context", "status": "planned", "exists": true}
  ],
  "unmet_dependencies": ["TASK-060"],
  "can_run": false
}
```

| Field | Description |
|-------|-------------|
| `blocked_by` | Tasks that must complete before this task (stored) |
| `blocks` | Tasks waiting on this task (computed inverse) |
| `related_to` | Related tasks for reference (stored) |
| `referenced_by` | Tasks whose descriptions mention this task (auto-detected) |
| `unmet_dependencies` | Blockers that are not yet completed |
| `can_run` | True if no unmet dependencies |

### Task Dependency Graph Visualization

Returns nodes and edges for visualizing dependencies across an arbitrary set of tasks.

**GET `/api/tasks/dependency-graph`**

Query parameters:
- `ids` (required) - Comma-separated list of task IDs to include

**Example:**
```
GET /api/tasks/dependency-graph?ids=TASK-060,TASK-061,TASK-062,TASK-063
```

**Response:**
```json
{
  "nodes": [
    {"id": "TASK-060", "title": "Add initiative_id field", "status": "done"},
    {"id": "TASK-061", "title": "Add sidebar navigation", "status": "ready"},
    {"id": "TASK-062", "title": "Add filter dropdown", "status": "blocked"},
    {"id": "TASK-063", "title": "Add initiative badges", "status": "ready"}
  ],
  "edges": [
    {"from": "TASK-060", "to": "TASK-061"},
    {"from": "TASK-061", "to": "TASK-062"}
  ]
}
```

| Field | Description |
|-------|-------------|
| `nodes` | Array of requested tasks with display status |
| `nodes[].status` | Simplified status: `done`, `running`, `blocked`, `ready`, `pending`, `paused`, `failed` |
| `edges` | Dependency relationships within the requested set only |

**Status mapping:**
| Internal Status | Display Status |
|-----------------|----------------|
| `completed`, `finished` | `done` |
| `running`, `finalizing` | `running` |
| `blocked` | `blocked` |
| `paused` | `paused` |
| `failed` | `failed` |
| `created`, `planned` | `ready` |
| (other) | `pending` |

**Error responses:**
- 400: Missing `ids` parameter or no valid task IDs provided

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

Group related tasks with shared decisions. Initiatives can depend on other initiatives.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/initiatives` | List (`?status=active`, `?shared=true`) |
| POST | `/api/initiatives` | Create initiative |
| GET | `/api/initiatives/:id` | Get initiative (includes computed `blocks` field) |
| PUT | `/api/initiatives/:id` | Update initiative |
| DELETE | `/api/initiatives/:id` | Delete initiative |
| GET | `/api/initiatives/:id/tasks` | List initiative tasks |
| POST | `/api/initiatives/:id/tasks` | Add task to initiative |
| DELETE | `/api/initiatives/:id/tasks/:taskId` | Remove task from initiative |
| POST | `/api/initiatives/:id/decisions` | Add decision |
| GET | `/api/initiatives/:id/ready` | Get tasks ready to run |
| GET | `/api/initiatives/:id/dependency-graph` | Get dependency graph visualization |

### Initiative Dependency Graph

Returns nodes and edges for visualizing task dependencies within an initiative.

**GET `/api/initiatives/:id/dependency-graph`**

Query parameters:
- `shared` - If `true`, load from shared initiatives directory

**Response:**
```json
{
  "nodes": [
    {
      "id": "TASK-060",
      "title": "Add initiative_id field to task schema",
      "status": "done"
    },
    {
      "id": "TASK-061",
      "title": "Add initiative sidebar section",
      "status": "ready"
    },
    {
      "id": "TASK-062",
      "title": "Add initiative filter dropdown",
      "status": "blocked"
    }
  ],
  "edges": [
    {"from": "TASK-060", "to": "TASK-061"},
    {"from": "TASK-061", "to": "TASK-062"}
  ]
}
```

| Field | Description |
|-------|-------------|
| `nodes` | Array of tasks in the initiative with display status |
| `nodes[].status` | Simplified status: `done`, `running`, `blocked`, `ready`, `pending`, `paused`, `failed` |
| `edges` | Dependency relationships (from = blocker, to = blocked task) |

Only edges where both tasks are in the initiative are included.

**Create initiative body (POST):**
```json
{
  "title": "React Migration",
  "vision": "Migrate all components to React 18",
  "blocked_by": ["INIT-001"],
  "owner": {
    "initials": "JD",
    "display_name": "John Doe"
  },
  "shared": false
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `title` | Initiative title (required) | - |
| `vision` | Vision statement | `""` |
| `blocked_by` | Initiative IDs that must complete first | `[]` |
| `owner` | Owner identity | `{}` |
| `shared` | Create in shared directory | `false` |

**Update initiative body (PUT):**
```json
{
  "title": "Updated Title",
  "vision": "Updated vision",
  "status": "active",
  "blocked_by": ["INIT-001", "INIT-002"]
}
```

All fields are optional. Setting `blocked_by` replaces the entire list.

**Initiative response:**
```json
{
  "id": "INIT-002",
  "title": "React Migration",
  "status": "active",
  "vision": "Migrate all components to React 18",
  "blocked_by": ["INIT-001"],
  "blocks": ["INIT-003"],
  "tasks": [...],
  "decisions": [...],
  "created_at": "2026-01-10T10:30:00Z",
  "updated_at": "2026-01-10T12:45:00Z"
}
```

| Field | Description |
|-------|-------------|
| `blocked_by` | Initiative IDs that must complete before this initiative (stored) |
| `blocks` | Initiative IDs waiting on this initiative (computed) |

**Validation:**
- Referenced initiative IDs must exist
- Self-references are rejected
- Circular dependencies are detected and rejected

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

### Settings (Claude Code)

Claude Code settings from `settings.json` files. Both global (`~/.claude/settings.json`) and project (`.claude/settings.json`) settings are editable via the UI.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/settings` | Get merged settings (global + project) |
| GET | `/api/settings/global` | Get global settings only |
| GET | `/api/settings/project` | Get project settings only |
| PUT | `/api/settings` | Update project settings |
| PUT | `/api/settings/global` | Update global settings |

**Settings body (PUT):**
```json
{
  "env": {
    "KEY": "value"
  },
  "statusLine": {
    "type": "command",
    "command": "echo -n '[$USER:${HOSTNAME%%.*}]:${PWD##*/}'"
  }
}
```

**Editable fields:**
- `env` - Environment variables (key-value pairs)
- `statusLine.type` - Type of statusline (`command`)
- `statusLine.command` - Shell command for statusline output

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

Orc orchestrator configuration from `.orc/config.yaml`. All settings are editable via the UI.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/config` | Get orc configuration (`?with_sources=true` for source tracking) |
| PUT | `/api/config` | Update orc configuration |
| GET | `/api/config/export` | Get export configuration |
| PUT | `/api/config/export` | Update export configuration |

**Config response:**
```json
{
  "version": "1.0.0",
  "profile": "auto",
  "automation": {
    "profile": "auto",
    "gates_default": "ai",
    "retry_enabled": true,
    "retry_max": 3
  },
  "execution": {
    "model": "claude-sonnet-4-20250514",
    "max_iterations": 10,
    "timeout": "30m"
  },
  "git": {
    "branch_prefix": "orc/",
    "commit_prefix": "[orc]"
  },
  "worktree": {
    "enabled": true,
    "dir": ".orc/worktrees",
    "cleanup_on_complete": true,
    "cleanup_on_fail": false
  },
  "completion": {
    "action": "pr",
    "target_branch": "main",
    "delete_branch": true
  },
  "timeouts": {
    "phase_max": "1h",
    "turn_max": "5m",
    "idle_warning": "2m",
    "heartbeat_interval": "10s",
    "idle_timeout": "10m"
  }
}
```

**Config update body (PUT):**
```json
{
  "profile": "safe",
  "automation": {
    "gates_default": "human",
    "retry_enabled": true,
    "retry_max": 5
  },
  "execution": {
    "model": "claude-opus-4-20250514",
    "max_iterations": 20,
    "timeout": "1h"
  },
  "git": {
    "branch_prefix": "feature/",
    "commit_prefix": "[feature]"
  },
  "worktree": {
    "enabled": true,
    "cleanup_on_complete": true,
    "cleanup_on_fail": false
  },
  "completion": {
    "action": "merge",
    "target_branch": "develop",
    "delete_branch": true
  },
  "timeouts": {
    "phase_max": "2h",
    "turn_max": "10m"
  }
}
```

All fields are optional. Only provided fields are updated. Setting `profile` applies a preset and then other fields override.

---

## Integration

### GitHub PR

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tasks/:id/github/pr` | Create PR for task branch |
| GET | `/api/tasks/:id/github/pr` | Get PR details, comments, checks |
| POST | `/api/tasks/:id/github/pr/merge` | Merge PR |
| POST | `/api/tasks/:id/github/pr/refresh` | Refresh PR status (reviews, checks, approval state) |
| POST | `/api/tasks/:id/github/pr/comments/sync` | Sync local comments to PR |
| POST | `/api/tasks/:id/github/pr/comments/:commentId/autofix` | Queue auto-fix |
| GET | `/api/tasks/:id/github/pr/checks` | Get CI check status |

**PR Status Polling:**
- PRs are automatically polled every 60 seconds for tasks with open PRs
- Status includes: review state (pending_review, changes_requested, approved), CI checks, mergeability
- PR status is stored in `task.yaml` under the `pr` field
- Manual refresh via `POST /api/tasks/:id/github/pr/refresh`
- 30 second rate limit between polls for the same task

**Auto-Trigger Finalize on Approval:**
- When PR status changes to `approved` and automation profile is `auto`:
  - Finalize phase is automatically triggered asynchronously
  - Controlled by `completion.finalize.auto_trigger_on_approval` config
  - Skips trivial tasks (finalize not applicable)
  - Only triggers if finalize hasn't already completed
  - WebSocket broadcasts progress via `finalize` events

**PR Status Values:**
| Status | Description |
|--------|-------------|
| `draft` | PR is in draft state |
| `pending_review` | PR awaiting review |
| `changes_requested` | Reviewers requested changes |
| `approved` | PR has been approved |
| `merged` | PR has been merged |
| `closed` | PR was closed without merging |

**Refresh response:**
```json
{
  "pr": { "number": 123, "state": "OPEN", ... },
  "status": {
    "url": "https://github.com/owner/repo/pull/123",
    "number": 123,
    "status": "approved",
    "checks_status": "success",
    "mergeable": true,
    "review_count": 2,
    "approval_count": 2,
    "last_checked_at": "2024-01-01T10:00:00Z"
  },
  "task_id": "TASK-001"
}
```

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

### Plugins

Manage Claude Code plugins from `.claude/plugins/`. Supports both local plugin management and marketplace browsing.

#### Local Plugins

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/plugins` | List installed plugins (`?scope=global\|project`) |
| GET | `/api/plugins/resources` | Get aggregated resources (MCP servers, hooks, commands) |
| GET | `/api/plugins/updates` | Check for available updates |
| GET | `/api/plugins/:name` | Get plugin details (`?scope=global\|project`) |
| GET | `/api/plugins/:name/commands` | List plugin commands |
| POST | `/api/plugins/:name/enable` | Enable plugin (`?scope=global\|project`) |
| POST | `/api/plugins/:name/disable` | Disable plugin (`?scope=global\|project`) |
| POST | `/api/plugins/:name/update` | Update plugin to latest version |
| DELETE | `/api/plugins/:name` | Uninstall plugin (`?scope=global\|project`) |

**Plugin info response:**
```json
{
  "name": "orc",
  "description": "Task orchestration for Claude Code",
  "scope": "project",
  "enabled": true,
  "has_commands": true,
  "command_count": 5
}
```

**Plugin detail response:**
```json
{
  "name": "orc",
  "description": "Task orchestration for Claude Code",
  "author": {"name": "Author Name", "url": "https://example.com"},
  "homepage": "https://github.com/example/plugin",
  "keywords": ["orchestration", "tasks"],
  "path": "/home/user/.claude/plugins/orc",
  "scope": "project",
  "enabled": true,
  "version": "1.0.0",
  "has_commands": true,
  "has_hooks": false,
  "has_scripts": true,
  "commands": [{"name": "init", "description": "Initialize project"}],
  "mcp_servers": [],
  "hooks": []
}
```

**Plugin resources response:**
```json
{
  "mcp_servers": [{"name": "server", "command": "...", "plugin_name": "orc", "plugin_scope": "project"}],
  "hooks": [{"event": "pre_prompt", "command": "...", "plugin_name": "orc", "plugin_scope": "global"}],
  "commands": [{"name": "init", "description": "...", "plugin_name": "orc", "plugin_scope": "project"}]
}
```

#### Marketplace

Browse and install plugins from the marketplace. Uses a separate `/api/marketplace/plugins` prefix to avoid route conflicts with local plugin management.

**Note:** When the official Claude Code plugin marketplace is unavailable, the API returns sample plugins with `is_mock: true` and a helpful message.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/marketplace/plugins` | Browse plugins (`?page=N&limit=N`) |
| GET | `/api/marketplace/plugins/search` | Search plugins (`?q=query`) |
| GET | `/api/marketplace/plugins/:name` | Get marketplace plugin details |
| POST | `/api/marketplace/plugins/:name/install` | Install plugin (`?scope=global\|project`) |

**Browse response:**
```json
{
  "plugins": [
    {
      "name": "orc",
      "description": "Task orchestration plugin",
      "author": {"name": "Author", "url": "https://example.com"},
      "version": "1.0.0",
      "repository": "https://github.com/example/orc-plugin",
      "downloads": 1250,
      "keywords": ["orchestration", "tasks"]
    }
  ],
  "total": 50,
  "page": 1,
  "limit": 20,
  "cached": true,
  "cache_age_seconds": 300,
  "is_mock": false,
  "message": null
}
```

**When marketplace is unavailable:**
```json
{
  "plugins": [...],
  "total": 6,
  "page": 1,
  "limit": 20,
  "is_mock": true,
  "message": "Showing sample plugins. The official Claude Code plugin marketplace is not yet available. Install plugins manually via 'claude plugin add <github-repo>'."
}
```

**Install body (optional):**
```json
{"version": "1.0.0"}
```

**Install response:**
```json
{
  "plugin": {...},
  "requires_restart": true,
  "message": "Plugin installed. Restart Claude Code to load."
}
```

### Task Diff

Git diff visualization for task implementation changes. Compares the task branch against a base branch (default: `main`).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks/:id/diff` | Get full diff with file list and hunks |
| GET | `/api/tasks/:id/diff/stats` | Get diff statistics only |
| GET | `/api/tasks/:id/diff/file/{path}` | Get diff for a single file |

**Query parameters:**
- `base` - Base branch to compare against (default: `main`)
- `files` - If `true`, return file list without hunks (for `/diff` endpoint only)

**Working tree support:** When the task branch has not diverged from the base branch (same commit), but there are uncommitted changes in the working tree, the diff will include those uncommitted changes. The `head` field in the response will show `"working tree"` in this case.

**Reference resolution:** Branch refs are automatically resolved. If a local branch doesn't exist but `origin/<branch>` does, the remote tracking branch is used.

**Full diff response:**
```json
{
  "base": "main",
  "head": "orc/TASK-001",
  "stats": {
    "files_changed": 3,
    "additions": 150,
    "deletions": 20
  },
  "files": [
    {
      "path": "internal/api/handlers.go",
      "status": "modified",
      "additions": 50,
      "deletions": 10,
      "binary": false,
      "syntax": "go",
      "hunks": [
        {
          "old_start": 10,
          "old_lines": 5,
          "new_start": 10,
          "new_lines": 8,
          "lines": [
            {"type": "context", "content": " func init() {", "old_line": 10, "new_line": 10},
            {"type": "deletion", "content": "-    oldCode()", "old_line": 11},
            {"type": "addition", "content": "+    newCode()", "new_line": 11}
          ]
        }
      ]
    }
  ]
}
```

**File status values:** `modified`, `added`, `deleted`, `renamed`, `copied`

**Line type values:** `context`, `addition`, `deletion`

**Stats-only response:**
```json
{
  "files_changed": 3,
  "additions": 150,
  "deletions": 20
}
```

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
| `finalize` | `FinalizeUpdate` | Finalize phase progress (see below) |
| `task_created` | `{task: Task}` | Task created via CLI/filesystem |
| `task_updated` | `{task: Task}` | Task modified via CLI/filesystem |
| `task_deleted` | `{task_id: string}` | Task deleted via CLI/filesystem |
| `initiative_created` | `{initiative: Initiative}` | Initiative created via CLI/filesystem |
| `initiative_updated` | `{initiative: Initiative}` | Initiative modified via CLI/filesystem |
| `initiative_deleted` | `{initiative_id: string}` | Initiative deleted via CLI/filesystem |

### Finalize Event Data

```json
{
  "task_id": "TASK-001",
  "status": "running",
  "step": "Syncing with target",
  "progress": "Merging changes",
  "step_percent": 50,
  "updated_at": "2026-01-10T10:31:00Z"
}
```

| Field | Description |
|-------|-------------|
| `status` | Current status: `pending`, `running`, `completed`, `failed` |
| `step` | Current step name (e.g., "Syncing with target", "Running tests") |
| `progress` | Human-readable progress message |
| `step_percent` | Progress percentage (0-100) |
| `error` | Error message (only present on failure) |
| `result` | Finalize result (only present on completion) |

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

File watcher events (`task_created`, `task_updated`, `task_deleted`, `initiative_created`, `initiative_updated`, `initiative_deleted`) are only broadcast to global subscribers. These events are triggered when tasks or initiatives are created, modified, or deleted outside the API (e.g., via CLI or direct filesystem edits).

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
