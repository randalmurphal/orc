# API Reference

REST API endpoints for the orc orchestrator. Base URL: `http://localhost:8080`

## Quick Navigation

| Category | Endpoints | Purpose |
|----------|-----------|---------|
| [Multi-Project](#multi-project-support) | (all services) | Project routing via `project_id` |
| [Tasks](#tasks-global) | `/api/tasks/*` | Task CRUD and execution |
| [Projects](#projects) | `/api/projects/*` | Multi-project registry and task operations |
| [Branches](#branchservice) | Connect RPC | Git branch management |
| [Initiatives](#initiatives) | `/api/initiatives/*` | Task grouping and decisions |
| [Decisions](#decisions) | `/api/decisions/*` | Gate approval/rejection |
| [Configuration](#configuration) | `/api/prompts/*`, `/api/hooks/*`, etc. | Project configuration |
| [Integration](#integration) | `/api/github/*`, `/api/mcp/*`, `/api/plugins/*` | External integrations |
| [Plugins](#plugins) | `/api/plugins/*`, `/api/marketplace/*` | Plugin management & marketplace |
| [Session](#session) | `/api/session` | Current session metrics |
| [Dashboard](#dashboard) | `/api/dashboard/*`, `/api/stats/*` | Statistics, activity, and file analytics |
| [Notifications](#notifications) | Connect RPC | User notification management |
| [Events](#events) | `/api/events` | Timeline event queries |
| [Workflows](#workflows) | `/api/workflows/*`, `/api/phase-templates/*` | Workflow and phase template configuration |
| [Workflow Runs](#workflow-runs) | `/api/workflow-runs/*` | Workflow execution instances |
| [Real-time](#websocket-protocol) | `/api/ws` | WebSocket events |

---

## Multi-Project Support

All Connect RPC services accept a `project_id` field in their request messages. This field routes the request to the correct project-specific database.

| Behavior | Condition |
|----------|-----------|
| Legacy single-project mode | `project_id` is empty or omitted |
| Project-scoped operation | `project_id` is set to a valid project ID |

**How it works:**
- The server maintains an LRU cache of project databases
- When `project_id` is provided, the request is routed to that project's SQLite database
- When `project_id` is empty, the server uses the CWD-based legacy backend (single project)
- All project-scoped services follow this pattern: TaskService, InitiativeService, WorkflowService, TranscriptService, EventService, ConfigService, HostingService, DashboardService, DecisionService, NotificationService, BranchService

**Services with project_id support:**

| Service | Proto File | Request Messages Updated |
|---------|-----------|--------------------------|
| TaskService | `task.proto` | All request messages |
| InitiativeService | `initiative.proto` | All request messages |
| HostingService | `hosting.proto` | CreatePR, GetPR, MergePR, RefreshPR, SyncComments, AutofixComment, GetChecks, ListPRs, GetPRComments |
| DashboardService | `dashboard.proto` | GetStats, GetActivity, GetPerDayStats, GetOutcomes, GetTopInitiatives, GetTopFiles, GetComparison, GetCostSummary, GetCostByModel, GetCostTimeseries, GetBudget |
| DecisionService | `decision.proto` | ListDecisions, ResolveDecision, GetDecision, ListDecisionHistory |
| NotificationService | `notification.proto` | ListNotifications, DismissNotification, DismissAllNotifications |
| BranchService | `project.proto` | ListBranches, GetBranch, UpdateBranchStatus, DeleteBranch, CleanupStaleBranches |
| ConfigService | `config.proto` | All request messages (GetConfig, UpdateConfig, GetSettings, UpdateSettings, GetSettingsHierarchy, ListHooks, CreateHook, UpdateHook, DeleteHook, ListSkills, CreateSkill, UpdateSkill, DeleteSkill, GetClaudeMd, UpdateClaudeMd, GetConstitution, UpdateConstitution, DeleteConstitution, ListPrompts, GetPrompt, GetDefaultPrompt, UpdatePrompt, DeletePrompt, ListPromptVariables, ListAgents, GetAgent, CreateAgent, UpdateAgent, DeleteAgent, ListScripts, DiscoverScripts, GetScript, CreateScript, UpdateScript, DeleteScript, ListTools, GetToolPermissions, UpdateToolPermissions, GetConfigStats) |
| WorkflowService | `workflow.proto` | All request messages including run requests (ListWorkflowRuns, GetWorkflowRun, StartWorkflowRun, CancelWorkflowRun, SaveWorkflowLayout) |
| TranscriptService | `transcript.proto` | All request messages |
| EventService | `events.proto` | All request messages |
| ProjectService | `project.proto` | N/A (manages projects themselves, not project-scoped) |

**REST API mapping:** For REST endpoints, `project_id` is passed as a query parameter (`?project_id=abc123`) or derived from the URL path (`/api/projects/:id/tasks`). File serving endpoints (`/files/tasks/{id}/attachments/*`, `/files/tasks/{id}/test-results/*`) and export/import endpoints (`/api/export`, `/api/import`) also accept `?project_id=...` for project routing.

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
| POST | `/api/tasks/:id/run` | Start task (`?force=true` to bypass blockers) |
| POST | `/api/tasks/:id/pause` | Pause task |
| POST | `/api/tasks/:id/resume` | Resume task |
| POST | `/api/tasks/:id/skip-block` | Clear blocked_by dependencies |
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

**Workflow auto-assignment:** When `weight` is specified without an explicit `workflow_id`, the workflow is auto-assigned based on weight:
| Weight | Workflow ID |
|--------|-------------|
| trivial | `implement-trivial` |
| small | `implement-small` |
| medium | `implement-medium` |
| large | `implement-large` |

Explicit `workflow_id` takes precedence over weight-derived assignment.

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

### Skip Block

Clear dependency blockers to make a blocked task runnable.

**POST `/api/tasks/:id/skip-block`**

No request body required.

**Response:**
```json
{
  "status": "success",
  "task_id": "TASK-001",
  "message": "Block skipped successfully",
  "cleared_blockers": ["TASK-060", "TASK-061"]
}
```

| Field | Description |
|-------|-------------|
| `status` | Always `success` on 200 |
| `task_id` | The task that was unblocked |
| `message` | Human-readable confirmation |
| `cleared_blockers` | Array of task IDs that were in `blocked_by` |

**Side effects:**
- Clears `blocked_by` field to empty array
- Sets `is_blocked` to `false`
- If task status was `blocked`, resets to `planned`

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
| PATCH | `/api/tasks/:id/comments/:commentId` | Update comment |
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

Multi-project support via global registry. Projects are managed through both REST endpoints and the Connect RPC `ProjectService`.

**REST Endpoints:**

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects` | List registered projects |
| GET | `/api/projects/default` | Get default project ID |
| PUT | `/api/projects/default` | Set default project |
| GET | `/api/projects/:id` | Get project details |
| DELETE | `/api/projects/:id` | Remove project from registry |
| GET | `/api/projects/:id/tasks` | List tasks for project |
| POST | `/api/projects/:id/tasks` | Create task in project |

**Connect RPC: ProjectService** (`proto/orc/v1/project.proto`)

| RPC Method | Description |
|------------|-------------|
| ListProjects | List all registered projects |
| GetProject | Get project by ID |
| GetDefaultProject | Get the default project (ID + project details) |
| SetDefaultProject | Set default project by ID |
| AddProject | Register a new project (name + path) |
| RemoveProject | Remove a project from the registry |

**Project object:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique project identifier |
| `name` | string | Project display name |
| `path` | string | Filesystem path to project root |
| `created_at` | timestamp | When the project was registered |
| `is_default` | bool | Whether this is the default project |

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

### BranchService

Connect RPC service for managing git branches. All requests accept `project_id`. See [Multi-Project Support](#multi-project-support).

**Connect RPC: BranchService** (`proto/orc/v1/project.proto`)

| RPC Method | Description |
|------------|-------------|
| ListBranches | List branches (filter by type, status, include orphaned) |
| GetBranch | Get branch by name |
| UpdateBranchStatus | Update branch status (active, merged, stale, orphaned) |
| DeleteBranch | Delete a branch (with optional force flag) |
| CleanupStaleBranches | Cleanup stale branches by age threshold (supports dry run) |

**Branch object:**

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Branch name |
| `type` | BranchType | `INITIATIVE`, `STAGING`, or `TASK` |
| `created_at` | timestamp | When branch was created |
| `last_activity` | timestamp | Last activity timestamp |
| `status` | BranchStatus | `ACTIVE`, `MERGED`, `STALE`, or `ORPHANED` |
| `owner_id` | string (optional) | Associated task ID or initiative ID |
| `commits_ahead` | int32 | Commits ahead of target branch |
| `commits_behind` | int32 | Commits behind target branch |
| `target_branch` | string (optional) | Target branch name |

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

## Decisions

Gate approval/rejection for human gates in headless (API/WebSocket) mode. When a task hits a human gate during API-driven execution, a `decision_required` WebSocket event is emitted and the decision is stored for resolution.

All DecisionService RPC requests accept `project_id` to target a specific project database. See [Multi-Project Support](#multi-project-support).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/decisions` | List all pending gate decisions |
| POST | `/api/decisions/:id` | Approve or reject a pending gate decision |

### List Pending Decisions

**GET `/api/decisions`**

Returns all pending gate decisions awaiting resolution. Use this endpoint to load pending decisions on page refresh. For real-time updates, subscribe to WebSocket `decision_required` events.

**Response (200):**
```json
[
  {
    "decision_id": "gate_TASK-001_review_1737504000000000000",
    "task_id": "TASK-001",
    "task_title": "Add user authentication",
    "phase": "review",
    "gate_type": "human",
    "question": "Please verify the following criteria:",
    "context": "Code review passes\nTests pass",
    "requested_at": "2026-01-22T10:00:00Z"
  }
]
```

| Field | Description |
|-------|-------------|
| `decision_id` | Unique ID for this decision |
| `task_id` | Associated task ID |
| `task_title` | Task title for display |
| `phase` | Phase awaiting approval |
| `gate_type` | Gate type (`human` or `ai`) |
| `question` | Prompt to show the user |
| `context` | Additional context (may be empty) |
| `requested_at` | When the decision was requested (ISO8601) |

**Notes:**
- Returns empty array `[]` if no decisions are pending
- Decisions are removed from the list when resolved via POST

### Resolve Decision

**POST `/api/decisions/:id`**

Resolves a pending gate decision by approving or rejecting it.

**Request body:**
```json
{
  "approved": true,
  "reason": "LGTM, all issues addressed"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `approved` | boolean | Yes | `true` to approve, `false` to reject |
| `reason` | string | No | Optional explanation for the decision |

**Success response (200):**
```json
{
  "decision_id": "gate_TASK-001_review",
  "task_id": "TASK-001",
  "approved": true,
  "new_status": "planned"
}
```

| Field | Description |
|-------|-------------|
| `decision_id` | The resolved decision ID |
| `task_id` | Associated task ID |
| `approved` | Whether the decision was approved |
| `new_status` | New task status: `planned` (approved) or `failed` (rejected) |

**Error responses:**

| Status | Condition |
|--------|-----------|
| 400 | Invalid request body |
| 400 | Task is not in blocked status |
| 404 | Decision not found (already resolved or never existed) |
| 404 | Associated task not found |
| 500 | Failed to save task or state |

**Side effects:**
- Task status changes to `planned` (approved) or `failed` (rejected)
- Gate decision is recorded in task state and database
- `decision_resolved` WebSocket event is emitted
- Decision is removed from pending store (subsequent POSTs return 404)

**Notes:**
- Pending decisions are stored in-memory; server restart clears them
- Multiple concurrent pending decisions can exist for different tasks
- The task does NOT auto-resume after approval; use `POST /api/tasks/:id/resume` or `orc resume` CLI
- CLI approval via `orc approve` continues to work independently

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

### Hooks (GlobalDB CRUD)

Hooks are stored in the `hook_scripts` table in GlobalDB. Built-in hooks (`is_builtin=true`) cannot be modified or deleted.

| RPC Method | Description |
|------------|-------------|
| `ListHooks` | List all hooks, ordered by built-in status then name |
| `CreateHook` | Create hook (name, content, event_type required; rejects duplicates) |
| `UpdateHook` | Update hook by ID (rejects built-in modifications) |
| `DeleteHook` | Delete hook by ID (rejects built-in deletions) |

**Event types**: `PreToolUse`, `PostToolUse`, `Notification`, `Stop`

**Error codes**: `InvalidArgument` (missing fields), `AlreadyExists` (duplicate name), `NotFound`, `PermissionDenied` (built-in)

### Skills (GlobalDB CRUD)

Skills are stored in the `skills` table in GlobalDB. Built-in skills (`is_builtin=true`) cannot be modified or deleted.

| RPC Method | Description |
|------------|-------------|
| `ListSkills` | List all skills, ordered by built-in status then name |
| `CreateSkill` | Create skill (name, content required; rejects duplicates) |
| `UpdateSkill` | Update skill by ID (rejects built-in modifications) |
| `DeleteSkill` | Delete skill by ID (rejects built-in deletions) |

**Error codes**: `InvalidArgument` (missing fields), `AlreadyExists` (duplicate name), `NotFound`, `PermissionDenied` (built-in)

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

Sub-agent definitions stored in SQLite with runtime statistics.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/agents` | List agents with stats (`?scope=global\|project`) |
| POST | `/api/agents` | Create agent |
| GET | `/api/agents/:name` | Get agent details |
| PUT | `/api/agents/:name` | Update agent |
| DELETE | `/api/agents/:name` | Delete agent (custom only) |

**Query parameters for GET `/api/agents`:**

| Parameter | Description | Values |
|-----------|-------------|--------|
| `scope` | Filter by scope | `global` (from `~/.claude/`), `project` (from SQLite) |

When no scope is specified, returns agents from both project (SQLite) and global sources.

**Agent response:**
```json
{
  "name": "code-reviewer",
  "description": "Reviews code for quality issues",
  "model": "sonnet",
  "prompt": "You are a code reviewer...",
  "tools": {
    "allow": ["Read", "Grep", "Edit"]
  },
  "scope": "PROJECT",
  "status": "idle",
  "stats": {
    "tokens_today": 45000,
    "tasks_done": 12,
    "success_rate": 0.92
  }
}
```

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Agent identifier |
| `description` | string | When to use this agent |
| `model` | string | Model override (`sonnet`, `opus`, `haiku`) |
| `prompt` | string | System prompt for the agent |
| `tools` | object | Tool permissions with `allow` list |
| `scope` | string | `PROJECT` or `GLOBAL` |
| `status` | string | `active` (running tasks) or `idle` |
| `stats.tokens_today` | number | Tokens used today for this model |
| `stats.tasks_done` | number | Completed tasks (all time) |
| `stats.success_rate` | number | Success rate 0.0-1.0 |

**Notes:**
- Stats are computed per model, not per agent (agents sharing a model share stats)
- Status is `active` if any task is running with the agent's model
- Returns empty array (not error) when no agents exist

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
  },
  "jira": {
    "url": "https://acme.atlassian.net",
    "email": "user@acme.com",
    "token_env_var": "ORC_JIRA_TOKEN",
    "epic_to_initiative": true,
    "default_weight": "small",
    "default_queue": "active",
    "custom_fields": {"customfield_10020": "jira_sprint"},
    "default_projects": ["PROJ"],
    "status_overrides": {},
    "category_overrides": {},
    "priority_overrides": {}
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
  },
  "jira": {
    "url": "https://acme.atlassian.net",
    "email": "user@acme.com",
    "default_projects": ["PROJ1", "PROJ2"]
  }
}
```

All fields are optional. Only provided fields are updated. Setting `profile` applies a preset and then other fields override.

---

## Integration

### Hosting / Pull Requests

All HostingService RPC requests accept `project_id` to target a specific project database. See [Multi-Project Support](#multi-project-support).

| RPC Method | Service | Description |
|------------|---------|-------------|
| CreatePR | HostingService | Create PR for task branch |
| GetPR | HostingService | Get PR details |
| MergePR | HostingService | Merge PR |
| RefreshPR | HostingService | Refresh PR status (reviews, checks, approval state) |
| SyncComments | HostingService | Sync local comments to PR |
| AutofixComment | HostingService | Queue auto-fix for a PR comment |
| GetChecks | HostingService | Get CI check status |

**PR Status Polling:**
- PRs are automatically polled every 60 seconds for tasks with open PRs
- Status includes: review state (pending_review, changes_requested, approved), CI checks, mergeability
- PR status is stored in database under the task's `pr` field
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
    "url": "https://github.com/owner/repo/pull/123 (or GitLab equivalent)",
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

**Autofix comment:**

Triggers an auto-fix for a specific PR comment. Fetches the comment from the hosting provider (GitHub or GitLab), sets up retry context, and spawns an executor to re-run the implement phase.

```
POST /api/tasks/:id/github/pr/comments/:commentId/autofix
```

**Response:**
```json
{
  "result": {
    "success": true
  }
}
```

| Field | Description |
|-------|-------------|
| `result.success` | `true` if autofix was started successfully |

**Behavior:**
- Returns immediately (~10ms) after spawning executor
- Task status changes to `running`
- Comment content is injected into `{{RETRY_CONTEXT}}` template variable
- Long comments (>10KB) are truncated

**Error responses:**
| Status | Code | Condition |
|--------|------|-----------|
| 400 | `InvalidArgument` | Missing task_id or comment_id |
| 404 | `NotFound` | Task or comment not found |
| 409 | `FailedPrecondition` | Task already running or completed |
| 401 | `Unauthenticated` | Hosting provider not authenticated |
| 429 | `ResourceExhausted` | Hosting provider API rate limited |

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

### Session

Current session metrics for the TopBar component. Session data is scoped to the server instance (not persisted across restarts).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/session` | Get session metrics (duration, tokens, cost, task counts) |

**Session metrics response:**
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "started_at": "2026-01-21T10:30:00Z",
  "duration_seconds": 3600,
  "total_tokens": 125000,
  "input_tokens": 45000,
  "output_tokens": 80000,
  "estimated_cost_usd": 2.45,
  "tasks_completed": 5,
  "tasks_running": 2,
  "is_paused": false
}
```

| Field | Description |
|-------|-------------|
| `session_id` | UUID generated at server startup |
| `started_at` | Server start time (RFC3339) |
| `duration_seconds` | Seconds elapsed since server start |
| `total_tokens` | Sum of input + output tokens for today |
| `input_tokens` | Input tokens consumed today |
| `output_tokens` | Output tokens generated today |
| `estimated_cost_usd` | Total cost for today's activity |
| `tasks_completed` | Count of tasks with `completed` status |
| `tasks_running` | Count of tasks with `running` status |
| `is_paused` | Always `false` (executor-level pause not exposed) |

**Notes:**
- Token counts aggregate from today's phase-level activity (UTC day boundary)
- Returns zeros for all numeric fields when no tasks exist (graceful empty state)
- Response time target: < 100ms for typical projects (< 100 tasks)

### Dashboard

All DashboardService RPC requests accept `project_id` to target a specific project database. See [Multi-Project Support](#multi-project-support).

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/dashboard/stats` | Get dashboard statistics |
| GET | `/api/stats/activity` | Get task activity data for heatmap |
| GET | `/api/stats/per-day` | Get daily task counts for bar chart (`?days=7`) |
| GET | `/api/stats/outcomes` | Get task outcome distribution for donut chart (`?period=30d`) |
| GET | `/api/stats/top-initiatives` | Get most active initiatives (`?limit=10&period=all`) |
| GET | `/api/stats/top-files` | Get most frequently modified files (`?limit=N&period=30d`) |
| GET | `/api/stats/comparison` | Get period comparison stats (`?period=7d`) |

**Dashboard stats response:**

Query parameters:
- `period` - Time period: `24h`, `7d`, `30d`, `all` (default: `all`)
- When period is set, returns period comparison data

```json
{
  "running": 1,
  "orphaned": 0,
  "paused": 0,
  "blocked": 2,
  "completed": 15,
  "failed": 1,
  "today": 3,
  "total": 19,
  "tokens": 245000,
  "cache_creation_input_tokens": 5000,
  "cache_read_input_tokens": 120000,
  "cost": 12.50,
  "avg_task_time_seconds": 3420.5,
  "success_rate": 93.75,
  "period": "7d",
  "previous_period": {
    "completed": 12,
    "tokens": 198000,
    "cost": 9.80,
    "avg_task_time_seconds": 3100.2,
    "success_rate": 85.7
  },
  "changes": {
    "completed": 25.0,
    "tokens": 23.7,
    "cost": 27.6
  }
}
```

| Field | Description |
|-------|-------------|
| `running` | Tasks currently executing |
| `orphaned` | Running tasks with stale heartbeat (potential orphans) |
| `paused` | Tasks paused by user |
| `blocked` | Tasks waiting for gate approval |
| `completed` | Total completed tasks (or within period if specified) |
| `failed` | Tasks that failed |
| `today` | Tasks completed today |
| `total` | Total tasks across all statuses |
| `tokens` | Total input + output tokens |
| `cache_creation_input_tokens` | Tokens written to prompt cache (aggregated) |
| `cache_read_input_tokens` | Tokens served from prompt cache (aggregated) |
| `cost` | Estimated cost in USD |
| `avg_task_time_seconds` | Average task completion time (only present when period specified) |
| `success_rate` | Percentage of completed vs completed+failed (only present when period specified) |
| `period` | Applied time filter (only present when period specified) |
| `previous_period` | Stats from the equivalent previous period (for comparison) |
| `changes` | Percentage changes from previous period |

**Activity data response (GET `/api/stats/activity`):**

Query parameters:
- `weeks` - Number of weeks to return (default: 16, min: 1, max: 52)

```json
{
  "start_date": "2025-09-23",
  "end_date": "2026-01-16",
  "data": [
    {"date": "2025-09-23", "count": 5, "level": 2},
    {"date": "2025-09-24", "count": 0, "level": 0}
  ],
  "stats": {
    "total_tasks": 247,
    "current_streak": 12,
    "longest_streak": 45,
    "busiest_day": {"date": "2025-12-15", "count": 23}
  }
}
```

| Field | Description |
|-------|-------------|
| `start_date` | First date in range (YYYY-MM-DD) |
| `end_date` | Last date in range (YYYY-MM-DD) |
| `data` | Array of `weeks * 7` daily activity entries |
| `data[].date` | Date in YYYY-MM-DD format |
| `data[].count` | Number of tasks completed on this date |
| `data[].level` | Activity level 0-4 for heatmap coloring |
| `stats.total_tasks` | Total completed tasks in range |
| `stats.current_streak` | Consecutive days ending today/yesterday |
| `stats.longest_streak` | Longest consecutive days in range |
| `stats.busiest_day` | Day with most completions (null if none) |

**Activity level thresholds:**

| Level | Task Count | Description |
|-------|------------|-------------|
| 0 | 0 | No activity |
| 1 | 1-2 | Light activity |
| 2 | 3-5 | Moderate activity |
| 3 | 6-10 | High activity |
| 4 | 11+ | Very high activity |

**Error responses:**
- 400: `weeks` parameter invalid (not a number, < 1, or > 52)

### Per-Day Stats

Returns daily task completion counts for bar chart visualization.

**GET `/api/stats/per-day`**

Query parameters:
- `days` - Number of days to return (default: 7, min: 1, max: 30)

**Response:**
```json
{
  "days": 7,
  "data": [
    {"date": "2026-01-16", "day": "Thu", "count": 5},
    {"date": "2026-01-17", "day": "Fri", "count": 3},
    {"date": "2026-01-18", "day": "Sat", "count": 0}
  ],
  "summary": {
    "total": 15,
    "average": 2.1,
    "max": {"date": "2026-01-16", "count": 5}
  }
}
```

| Field | Description |
|-------|-------------|
| `days` | Number of days in response |
| `data` | Array of exactly N daily entries |
| `data[].date` | Date in YYYY-MM-DD format |
| `data[].day` | Short day name (Mon, Tue, etc.) |
| `data[].count` | Number of tasks completed on this date |
| `summary.total` | Total completions in period |
| `summary.average` | Average completions per day |
| `summary.max` | Day with most completions |

**Error responses:**
- 400: `days must be a number between 1 and 30`

### Task Outcomes

Returns task outcome distribution for donut chart visualization.

**GET `/api/stats/outcomes`**

Query parameters:
- `period` - Time filter: `24h`, `7d`, `30d`, `all` (default: `all`)

**Response:**
```json
{
  "period": "30d",
  "outcomes": {
    "completed": 45,
    "with_retries": 8,
    "failed": 3
  },
  "total": 56,
  "success_rate": 94.6
}
```

| Field | Description |
|-------|-------------|
| `period` | Applied time filter |
| `outcomes.completed` | Tasks completed on first attempt |
| `outcomes.with_retries` | Tasks completed after retry attempts |
| `outcomes.failed` | Tasks that failed |
| `total` | Total tasks with a final outcome |
| `success_rate` | Percentage of successful completions |

**Note:** The period filter applies to task completion time. Only completed or failed tasks are included.

**Error responses:**
- 400: `period must be one of: 24h, 7d, 30d, all`

### Top Initiatives

Returns most active initiatives ranked by task count.

**GET `/api/stats/top-initiatives`**

Query parameters:
- `limit` - Maximum initiatives to return (default: 10, min: 1, max: 25)
- `period` - Time filter: `24h`, `7d`, `30d`, `all` (default: `all`)

**Response:**
```json
{
  "period": "all",
  "initiatives": [
    {
      "id": "INIT-001",
      "title": "User Authentication",
      "task_count": 12,
      "completed_count": 10,
      "completion_rate": 83.3,
      "total_tokens": 450000,
      "total_cost_usd": 18.50
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `period` | Applied time filter |
| `initiatives[].id` | Initiative ID |
| `initiatives[].title` | Initiative title |
| `initiatives[].task_count` | Total tasks linked to this initiative |
| `initiatives[].completed_count` | Completed tasks (subject to period filter) |
| `initiatives[].completion_rate` | Percentage of tasks completed |
| `initiatives[].total_tokens` | Total token usage |
| `initiatives[].total_cost_usd` | Total cost in USD |

**Note:** When period filter is applied, only tasks completed within that period count toward `completed_count` and derived metrics. The `task_count` shows total linked tasks regardless of period.

**Error responses:**
- 400: `limit must be a number between 1 and 25`
- 400: `period must be one of: 24h, 7d, 30d, all`

### Top Files Leaderboard

Returns most frequently modified files across completed tasks, ranked by change count. Aggregates file modification statistics from git diffs of completed task branches.

**GET `/api/stats/top-files`** (Connect RPC: `DashboardService.GetTopFiles`)

Query parameters:
- `limit` - Maximum number of files to return (default: 10, min: 1, max: 50)
- `task_id` - Optional filter to get files for a specific task only

**Response:**
```json
{
  "files": [
    {
      "path": "internal/api/dashboard_server.go",
      "change_count": 3,
      "additions": 450,
      "deletions": 120
    },
    {
      "path": "web/src/components/Board.tsx",
      "change_count": 2,
      "additions": 180,
      "deletions": 45
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `files` | Array of files sorted by change_count (descending) |
| `files[].path` | File path relative to project root |
| `files[].change_count` | Number of completed tasks that modified this file |
| `files[].additions` | Total lines added across all tasks |
| `files[].deletions` | Total lines deleted across all tasks |

**How it works:**
1. Loads all completed tasks with branches (or single task if `task_id` specified)
2. For each task, gets git diff against `main` branch via DiffService
3. Aggregates file statistics: counts modifications, sums additions/deletions
4. Returns top N files sorted by change_count descending

**Notes:**
- Only completed tasks with branches are included
- Tasks without branches are skipped gracefully
- Git diff errors are logged and skipped (graceful degradation)
- Returns empty `files` array (not error) when no data matches
- Binary files are included with 0 additions/deletions

### Stats Comparison

Returns comparison between current and previous period.

**GET `/api/stats/comparison`**

Query parameters:
- `period` - Comparison period: `7d`, `30d` (default: `7d`)

**Response:**
```json
{
  "current": {
    "tasks_completed": 15,
    "total_tokens": 245000,
    "total_cost_usd": 12.50,
    "success_rate": 93.8
  },
  "previous": {
    "tasks_completed": 12,
    "total_tokens": 198000,
    "total_cost_usd": 9.80,
    "success_rate": 85.7
  },
  "changes": {
    "tasks": 25.0,
    "tokens": 23.7,
    "cost": 27.6,
    "success_rate": 9.5
  }
}
```

| Field | Description |
|-------|-------------|
| `current` | Stats for the current period (last N days) |
| `previous` | Stats for the equivalent previous period (N to 2N days ago) |
| `current/previous.tasks_completed` | Number of completed tasks |
| `current/previous.total_tokens` | Total token usage |
| `current/previous.total_cost_usd` | Total cost in USD |
| `current/previous.success_rate` | Percentage of successful completions |
| `changes.tasks` | Percentage change in task count |
| `changes.tokens` | Percentage change in token usage |
| `changes.cost` | Percentage change in cost |
| `changes.success_rate` | Percentage change in success rate |

**Note:** Percentage changes are calculated as `(current - previous) / previous * 100`. When previous is 0, returns 100% if current > 0.

**Error responses:**
- 400: `period must be one of: 7d, 30d`

### Cost Tracking

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/cost/summary` | Get cost summary (`?period=day|week|month|all`) |
| GET | `/api/cost/by-model` | Get costs grouped by model (`?project_id=&since=`) |
| GET | `/api/cost/timeseries` | Get time-bucketed costs (`?granularity=day|week|month`) |
| GET | `/api/cost/budget` | Get project budget status |
| PUT | `/api/cost/budget` | Set project budget |

**Cost summary response:**
```json
{
  "period": "week",
  "start": "2026-01-10",
  "end": "2026-01-17",
  "total_cost_usd": 28.50,
  "total_input_tokens": 890000,
  "total_output_tokens": 310000,
  "total_tokens": 1200000,
  "entry_count": 45,
  "by_project": {"project-abc": 20.00, "project-xyz": 8.50},
  "by_phase": {"implement": 18.00, "spec": 6.50, "test": 4.00}
}
```

**Costs by model response:**
```json
{
  "opus": 24.00,
  "sonnet": 4.20,
  "haiku": 0.30
}
```

**Cost timeseries response:**
```json
{
  "granularity": "day",
  "data": [
    {
      "project_id": "project-abc",
      "model": "opus",
      "phase": "",
      "date": "2026-01-15",
      "total_cost_usd": 8.50,
      "total_input_tokens": 280000,
      "total_output_tokens": 95000,
      "total_cache_tokens": 120000,
      "turn_count": 15,
      "task_count": 3
    }
  ]
}
```

**Budget status response:**
```json
{
  "project_id": "project-abc",
  "monthly_limit_usd": 100.00,
  "current_month_spent": 82.50,
  "current_month": "2026-01",
  "percent_used": 82.5,
  "alert_threshold": 80,
  "over_budget": false,
  "at_alert_threshold": true
}
```

**Set budget body:**
```json
{
  "project_id": "project-abc",
  "monthly_limit_usd": 100.00,
  "alert_threshold_percent": 80
}
```

**Note:** The database layer for cost tracking with model identification is implemented (TASK-406). API endpoint handlers are pending future work.

---

## Notifications

Connect RPC service for user notifications. All requests accept `project_id`. See [Multi-Project Support](#multi-project-support).

**Connect RPC: NotificationService** (`proto/orc/v1/notification.proto`)

| RPC Method | Description |
|------------|-------------|
| ListNotifications | List all notifications for a project |
| DismissNotification | Dismiss a single notification by ID |
| DismissAllNotifications | Dismiss all notifications for a project |

**Notification object:**

| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Notification ID |
| `type` | string | Notification type |
| `title` | string | Notification title |
| `message` | string (optional) | Notification body |
| `source_type` | string (optional) | Source type (e.g., task, initiative) |
| `source_id` | string (optional) | Source entity ID |
| `created_at` | timestamp | When notification was created |
| `expires_at` | timestamp (optional) | When notification expires |

---

## Events

Query historical events for timeline display. Events are persisted via the event publisher and can be filtered by task, initiative, time range, and event types.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/events` | List events with optional filters |

**Query parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `task_id` | string | - | Filter events by task ID |
| `initiative_id` | string | - | Filter events by initiative (joins with tasks table) |
| `since` | string | - | Start of time range (ISO8601/RFC3339) |
| `until` | string | - | End of time range (ISO8601/RFC3339) |
| `types` | string | - | Comma-separated event types to include |
| `limit` | number | 100 | Max events to return (1-1000) |
| `offset` | number | 0 | Offset for pagination |

**Response:**
```json
{
  "events": [
    {
      "id": 123,
      "task_id": "TASK-001",
      "task_title": "Implement login flow",
      "phase": "implement",
      "iteration": 2,
      "event_type": "phase_completed",
      "data": {"duration_ms": 45000},
      "source": "executor",
      "created_at": "2026-01-10T10:35:00Z"
    }
  ],
  "total": 150,
  "limit": 100,
  "offset": 0,
  "has_more": true
}
```

| Field | Description |
|-------|-------------|
| `events` | Array of events (empty array if no matches) |
| `events[].id` | Unique event ID |
| `events[].task_id` | Associated task ID |
| `events[].task_title` | Task title (from tasks table join) |
| `events[].phase` | Phase name (optional) |
| `events[].iteration` | Phase iteration (optional) |
| `events[].event_type` | Event type (e.g., `phase_completed`, `task_started`) |
| `events[].data` | Event-specific data (optional) |
| `events[].source` | Event source (e.g., `executor`, `api`, `cli`) |
| `events[].created_at` | Event timestamp (ISO8601) |
| `total` | Total matching events (for pagination) |
| `limit` | Applied limit |
| `offset` | Applied offset |
| `has_more` | True if more events exist beyond current page |

**Error responses:**

| Status | Condition |
|--------|-----------|
| 400 | Invalid `since`/`until` timestamp format |
| 400 | `limit` < 1 or > 1000 |
| 400 | `offset` < 0 |
| 500 | Database error |

**Notes:**
- Invalid `task_id` or `initiative_id` returns empty results (not an error)
- `initiative_id` filter works by joining with the tasks table
- Events are sorted by `created_at` descending (newest first)

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

## Workflows

Configurable workflow definitions with composable phases.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/workflows` | List all workflows |
| POST | `/api/workflows` | Create workflow |
| GET | `/api/workflows/:id` | Get workflow with phases and variables |
| PUT | `/api/workflows/:id` | Update workflow |
| DELETE | `/api/workflows/:id` | Delete workflow (custom only) |
| POST | `/api/workflows/:id/clone` | Clone workflow |
| POST | `/api/workflows/:id/phases` | Add phase to workflow |
| PATCH | `/api/workflows/:id/phases/:phaseId` | Update phase (sequence, dependencies, overrides) |
| DELETE | `/api/workflows/:id/phases/:phaseId` | Remove phase from workflow |
| PUT | `/api/workflows/:id/layout` | Save node positions for visual editor |
| POST | `/api/workflows/:id/validate` | Validate workflow structure (check for cycles) |
| POST | `/api/workflows/:id/variables` | Add variable to workflow |
| DELETE | `/api/workflows/:id/variables/:name` | Remove variable from workflow |

**Create workflow body:**
```json
{
  "id": "my-custom-workflow",
  "name": "My Custom Workflow",
  "description": "Custom workflow for specific task type",
  "workflow_type": "task",
  "default_model": "sonnet",
  "default_thinking": false
}
```

| Field | Values | Default |
|-------|--------|---------|
| `id` | Unique identifier (lowercase, hyphens) | Required |
| `name` | Display name | Required |
| `workflow_type` | `task`, `branch`, `standalone` | `task` |
| `default_model` | `sonnet`, `opus`, `haiku` | (inherit) |
| `default_thinking` | `true`, `false` | `false` |

**Clone workflow body:**
```json
{
  "new_id": "my-cloned-workflow",
  "new_name": "My Cloned Workflow"
}
```

### Update Phase

Update a phase within a workflow. Used by the visual editor for connection management.

**PATCH `/api/workflows/:id/phases/:phaseId`**

**Request body:**
```json
{
  "sequence": 3,
  "depends_on": ["spec", "tdd_write"],
  "max_iterations_override": 50,
  "model_override": "opus",
  "thinking_override": true,
  "gate_type_override": "human",
  "condition": "{{HAS_TESTS}}",
  "agent_override": "custom-agent",
  "sub_agents_override": ["reviewer", "tester"],
  "claude_config_override": "{\"hooks\":[\"pre-commit\"],\"allowed_tools\":[\"Bash\"]}"
}
```

All fields are optional. Only provided fields are updated.

| Field | Description |
|-------|-------------|
| `sequence` | Execution order (lower runs first) |
| `depends_on` | Phase template IDs that must complete before this phase |
| `max_iterations_override` | Override max iterations from template |
| `model_override` | Override model (`sonnet`, `opus`, `haiku`) |
| `thinking_override` | Override extended thinking setting |
| `gate_type_override` | Override gate type (`auto`, `human`, `skip`) |
| `condition` | Conditional execution expression |
| `agent_override` | Override executor agent for this phase |
| `sub_agents_override` | Override available sub-agents (array of agent IDs) |
| `claude_config_override` | JSON string overriding claude_config (hooks, skills, MCP servers, tools, env vars). Merged with template config at execution time |

**Response:** Returns the updated `WorkflowPhase` object.

### Save Workflow Layout

Bulk-save node positions for the visual workflow editor. Positions are stored per-phase and restored on reload.

**PUT `/api/workflows/:id/layout`**

**Request body:**
```json
{
  "positions": [
    {"phase_template_id": "spec", "position_x": 100.0, "position_y": 200.0},
    {"phase_template_id": "implement", "position_x": 300.0, "position_y": 200.0}
  ]
}
```

| Field | Description |
|-------|-------------|
| `positions` | Array of phase positions |
| `positions[].phase_template_id` | Phase template ID |
| `positions[].position_x` | X coordinate in canvas |
| `positions[].position_y` | Y coordinate in canvas |

**Response:**
```json
{"success": true}
```

**Notes:**
- Empty `positions` array clears all stored positions (triggers auto-layout on reload)
- Positions for phases not in the workflow are ignored
- Built-in workflows can also store positions (persisted per-user)

### Validate Workflow

Check workflow structure for cycles, invalid references, and other issues.

**POST `/api/workflows/:id/validate`**

No request body required.

**Response:**
```json
{
  "valid": true,
  "issues": []
}
```

**Response with issues:**
```json
{
  "valid": false,
  "issues": [
    {
      "severity": "error",
      "message": "Dependency cycle detected: implement -> review -> implement",
      "phase_ids": ["implement", "review"]
    }
  ]
}
```

| Field | Description |
|-------|-------------|
| `valid` | `true` if workflow has no errors |
| `issues` | Array of validation issues |
| `issues[].severity` | `error` or `warning` |
| `issues[].message` | Human-readable description |
| `issues[].phase_ids` | Affected phases (for highlighting in UI) |

**Validated conditions:**
- No dependency cycles
- All `depends_on` references point to valid phases in the workflow
- No duplicate phase template IDs

### Phase Templates

Reusable phase definitions with prompts and configuration.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/phase-templates` | List all phase templates |
| POST | `/api/phase-templates` | Create phase template |
| GET | `/api/phase-templates/:id` | Get phase template |
| PUT | `/api/phase-templates/:id` | Update phase template |
| DELETE | `/api/phase-templates/:id` | Delete phase template (custom only) |
| GET | `/api/phase-templates/:id/prompt` | Get phase template prompt content |

**Phase template response:**
```json
{
  "id": "implement",
  "name": "Implementation",
  "description": "Write code guided by breakdown",
  "prompt_source": "embedded",
  "max_iterations": 50,
  "gate_type": "auto",
  "produces_artifact": false,
  "is_builtin": true
}
```

| Field | Description |
|-------|-------------|
| `prompt_source` | `embedded` (bundled), `db` (stored in DB), `file` (external file) |
| `gate_type` | `auto` (AI-approved), `human` (requires manual approval) |
| `produces_artifact` | Whether phase generates artifact (spec, tests, etc.) |
| `artifact_type` | Artifact type: `spec`, `tests`, `breakdown`, `docs`, etc. |
| `is_builtin` | Built-in templates cannot be modified |

---

## Workflow Runs

Execution instances of workflows.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/workflow-runs` | List workflow runs (`?status=running`, `?workflow_id=implement`) |
| POST | `/api/workflow-runs` | Trigger new workflow run |
| GET | `/api/workflow-runs/:id` | Get workflow run details |
| POST | `/api/workflow-runs/:id/cancel` | Cancel running workflow |
| GET | `/api/workflow-runs/:id/transcript` | Get workflow run transcript |

**Trigger workflow run body:**
```json
{
  "workflow_id": "implement",
  "prompt": "Add user authentication",
  "instructions": "Use JWT tokens",
  "context_type": "task",
  "task_id": "TASK-001"
}
```

| Field | Description |
|-------|-------------|
| `workflow_id` | Which workflow to execute |
| `prompt` | Task description (becomes `TASK_DESCRIPTION`) |
| `instructions` | Additional guidance (optional) |
| `context_type` | `task`, `branch`, `pr`, `standalone` |
| `task_id` | Link to existing task (optional) |

**Workflow run response:**
```json
{
  "id": "WRUN-001",
  "workflow_id": "implement",
  "status": "running",
  "current_phase": "spec",
  "context_type": "task",
  "task_id": "TASK-001",
  "total_cost_usd": 0.45,
  "total_tokens": 12500,
  "started_at": "2026-01-22T10:30:00Z"
}
```

| Status | Description |
|--------|-------------|
| `pending` | Queued, not yet started |
| `running` | Currently executing phases |
| `completed` | All phases finished successfully |
| `failed` | Execution failed |
| `cancelled` | Cancelled by user |

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
| `activity` | `ActivityUpdate` | Activity state changed (see below) |
| `heartbeat` | `HeartbeatData` | Progress heartbeat during API wait |
| `warning` | `{phase, message}` | Non-fatal warning |
| `finalize` | `FinalizeUpdate` | Finalize phase progress (see below) |
| `task_created` | `{task: Task}` | Task created via CLI/filesystem |
| `task_updated` | `{task: Task}` | Task modified via CLI/filesystem |
| `task_deleted` | `{task_id: string}` | Task deleted via CLI/filesystem |
| `initiative_created` | `{initiative: Initiative}` | Initiative created via CLI/filesystem |
| `initiative_updated` | `{initiative: Initiative}` | Initiative modified via CLI/filesystem |
| `initiative_deleted` | `{initiative_id: string}` | Initiative deleted via CLI/filesystem |
| `session_update` | `SessionUpdate` | Session-level metrics update (see below) |
| `files_changed` | `FilesChangedUpdate` | Files modified during task execution (see below) |
| `decision_required` | `DecisionRequiredData` | Human gate requires approval (see below) |
| `decision_resolved` | `DecisionResolvedData` | Gate decision was resolved (see below) |

### Decision Event Data

Decision events enable real-time gate approval in headless mode. When a task hits a human gate during API execution, a `decision_required` event is broadcast. When resolved via `POST /api/decisions/:id`, a `decision_resolved` event is broadcast.

**decision_required:**
```json
{
  "decision_id": "gate_TASK-001_review",
  "task_id": "TASK-001",
  "task_title": "Add user authentication",
  "phase": "review",
  "gate_type": "human",
  "question": "Please verify the following criteria:",
  "context": "Code review passes\nTests pass",
  "requested_at": "2026-01-10T10:30:00Z"
}
```

| Field | Description |
|-------|-------------|
| `decision_id` | Unique ID for this decision (format: `gate_{task_id}_{phase}`) |
| `task_id` | Associated task ID |
| `task_title` | Task title for display |
| `phase` | Phase awaiting approval |
| `gate_type` | Always `"human"` for these events |
| `question` | Prompt to show the user |
| `context` | Gate criteria (newline-separated) |
| `requested_at` | When the decision was requested (ISO8601) |

**decision_resolved:**
```json
{
  "decision_id": "gate_TASK-001_review",
  "task_id": "TASK-001",
  "phase": "review",
  "approved": true,
  "reason": "LGTM",
  "resolved_by": "api",
  "resolved_at": "2026-01-10T10:35:00Z"
}
```

| Field | Description |
|-------|-------------|
| `decision_id` | The resolved decision ID |
| `task_id` | Associated task ID |
| `phase` | Phase that was approved/rejected |
| `approved` | Whether the decision was approved |
| `reason` | Optional reason provided during resolution |
| `resolved_by` | Resolution source: `"api"` or `"cli"` |
| `resolved_at` | When the decision was resolved (ISO8601) |

**Workflow:**
1. Task hits human gate during API execution
2. Task status changes to `blocked`
3. `decision_required` event is broadcast to WebSocket subscribers
4. UI displays approval prompt
5. User clicks approve/reject, frontend calls `POST /api/decisions/:id`
6. Task status changes to `planned` (approved) or `failed` (rejected)
7. `decision_resolved` event is broadcast
8. User explicitly resumes task via `POST /api/tasks/:id/resume`

### Session Update Event Data

Session update events provide aggregate metrics across all tasks. They use `task_id: "*"` (GlobalTaskID) so all WebSocket subscribers receive them. A `session_update` is automatically sent when:
- A client subscribes to all tasks (`{"type": "subscribe", "task_id": "*"}`)
- Session metrics change (task starts/completes, tokens accumulate, etc.)

**session_update:**
```json
{
  "duration_seconds": 3420,
  "total_tokens": 245000,
  "estimated_cost_usd": 12.50,
  "input_tokens": 180000,
  "output_tokens": 65000,
  "tasks_running": 2,
  "is_paused": false
}
```

| Field | Description |
|-------|-------------|
| `duration_seconds` | Session uptime in seconds |
| `total_tokens` | Aggregate input + output tokens |
| `estimated_cost_usd` | Estimated cost in USD |
| `input_tokens` | Total input tokens |
| `output_tokens` | Total output tokens |
| `tasks_running` | Number of currently executing tasks |
| `is_paused` | Whether the session is globally paused |

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

### Activity Event Data

Activity events provide real-time feedback about executor activity during phase execution.

```json
{
  "phase": "spec",
  "activity": "spec_analyzing"
}
```

| Field | Description |
|-------|-------------|
| `phase` | Current phase name |
| `activity` | Activity state (see below) |

**Activity states:**

| State | Description | CLI Display |
|-------|-------------|-------------|
| `idle` | No activity | - |
| `waiting_api` | Waiting for Claude API response | "Waiting for Claude API..." |
| `streaming` | Actively receiving streaming response | - |
| `running_tool` | Claude is running a tool | "Running tool..." |
| `processing` | Processing response | - |
| `spec_analyzing` | Analyzing codebase during spec phase | "Analyzing codebase..." |
| `spec_writing` | Writing specification document | "Writing specification..." |

**Spec phase progress:**

The `spec_analyzing` and `spec_writing` states are specific to the spec phase and provide granular feedback during long-running specification generation. The spec phase has two distinct stages:

1. **Analyzing** (`spec_analyzing`): Claude reads the codebase, researches patterns, identifies affected files
2. **Writing** (`spec_writing`): Claude writes the specification document

**Web UI handling:**

TaskCard shows the activity state as subtext under the phase name for running tasks. The `ACTIVITY_CONFIG` map in `web/src/lib/types.ts` provides display labels.

### Files Changed Event Data

Files changed events report real-time file modifications during task execution. Emitted every 10 seconds when files change.

```json
{
  "files": [
    {"path": "internal/api/task_server.go", "status": "modified", "additions": 45, "deletions": 12},
    {"path": "internal/api/task_server_test.go", "status": "added", "additions": 120, "deletions": 0}
  ],
  "total_additions": 165,
  "total_deletions": 12,
  "timestamp": "2026-01-10T10:31:00Z"
}
```

| Field | Description |
|-------|-------------|
| `files` | Array of changed files with per-file stats |
| `files[].path` | Relative file path from worktree root |
| `files[].status` | Change type: `added`, `modified`, `deleted`, `renamed` |
| `files[].additions` | Lines added in this file |
| `files[].deletions` | Lines removed from this file |
| `total_additions` | Sum of additions across all files |
| `total_deletions` | Sum of deletions across all files |
| `timestamp` | When this snapshot was taken |

**Behavior:**
- Polls git diff every 10 seconds during task execution
- Only committed/staged changes are reported (not untracked files)
- Compares HEAD against the target branch merge-base
- Deduplicates: only emits when file state actually changes
- Useful for progress indicators showing lines changed during implementation

### Heartbeat Event Data

Heartbeat events indicate continued activity during long-running API calls.

```json
{
  "phase": "spec",
  "iteration": 1,
  "timestamp": "2026-01-10T10:31:00Z"
}
```

Heartbeats are emitted every 30 seconds during API wait states. The CLI displays these as dots to show continued progress.

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

Subscribe to `"*"` to receive database events for all tasks:

```json
{"type": "subscribe", "task_id": "*"}
```

Database events (`task_created`, `task_updated`, `task_deleted`, `initiative_created`, `initiative_updated`, `initiative_deleted`) are only broadcast to global subscribers. These events are triggered when tasks or initiatives are created, modified, or deleted via CLI or API operations.

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
