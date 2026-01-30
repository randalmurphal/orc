# Multi-Project Support Design

**Date:** 2025-01-29
**Status:** Approved
**Summary:** Proper multi-project support with single-server multi-tenancy, URL-based project context, and global shared resources.

## Problem

Current orc "multi-project support" is broken:

- Server binds to one project at startup based on cwd
- UI project switcher is decorative (changes localStorage, not API target)
- No CLI flag to target different projects from anywhere
- Must run multiple servers or `cd` between projects

## Solution

Single-server multi-tenant architecture where:

- One `orc serve` serves ALL registered projects
- Project context is explicit in every request (URL path)
- UI project switching actually works (navigation, not mode toggle)
- CLI supports `--project` flag with cwd fallback

## Architecture

### Server Model

```
┌─────────────────────────────────────────────────────────┐
│                     orc serve                           │
├─────────────────────────────────────────────────────────┤
│  Global DB (~/.orc/orc.db)                              │
│  ├── workflows                                          │
│  ├── phase_templates                                    │
│  ├── agents                                             │
│  └── cost_tracking                                      │
├─────────────────────────────────────────────────────────┤
│  Project DB Cache (LRU, max 10)                         │
│  ├── abc123 → /home/user/repos/orc/.orc/orc.db          │
│  ├── def456 → /home/user/repos/llmkit/.orc/orc.db       │
│  └── ...                                                │
└─────────────────────────────────────────────────────────┘
```

**Key points:**

- No "current project" at server level
- Databases opened lazily on first request
- LRU eviction when cache full (prevents memory bloat)
- Global DB always open (workflows, phases, agents shared)

### Resource Scoping

| Resource | Scope | Storage |
|----------|-------|---------|
| Tasks | Per-project | `.orc/orc.db` (project) |
| Initiatives | Per-project | `.orc/orc.db` (project) |
| Transcripts | Per-project | `.orc/orc.db` (project) |
| Execution state | Per-project | `.orc/orc.db` (project) |
| Project config | Per-project | `.orc/config.yaml` |
| Workflows | Global (shared) | `~/.orc/orc.db` (global) |
| Phase templates | Global (shared) | `~/.orc/orc.db` (global) |
| Agents | Global (shared) | `~/.orc/orc.db` (global) |
| Project registry | Global | `~/.orc/projects.yaml` |
| Cost tracking | Global | `~/.orc/orc.db` (global) |

## API Structure

### Project-Scoped Routes

All project-specific operations include project ID in path:

```
POST   /api/projects/:id/tasks              # Create task
GET    /api/projects/:id/tasks              # List tasks
GET    /api/projects/:id/tasks/:taskId      # Get task
PUT    /api/projects/:id/tasks/:taskId      # Update task
DELETE /api/projects/:id/tasks/:taskId      # Delete task
POST   /api/projects/:id/tasks/:taskId/run  # Run task

GET    /api/projects/:id/initiatives        # List initiatives
POST   /api/projects/:id/initiatives        # Create initiative
GET    /api/projects/:id/initiatives/:initId
PUT    /api/projects/:id/initiatives/:initId
DELETE /api/projects/:id/initiatives/:initId

GET    /api/projects/:id/transcripts/:transcriptId
GET    /api/projects/:id/config
PUT    /api/projects/:id/config
```

### Global Routes

No project context required:

```
GET    /api/projects                        # List all projects
POST   /api/projects/register               # Register project by path
DELETE /api/projects/:id                    # Unregister project

GET    /api/workflows                       # List workflows
POST   /api/workflows                       # Create workflow
GET    /api/workflows/:id
PUT    /api/workflows/:id
DELETE /api/workflows/:id

GET    /api/phases                          # List phase templates
POST   /api/phases
GET    /api/phases/:id
PUT    /api/phases/:id
DELETE /api/phases/:id

GET    /api/agents                          # List agents
POST   /api/agents
GET    /api/agents/:id
PUT    /api/agents/:id
DELETE /api/agents/:id

GET    /api/settings                        # Global orc settings
PUT    /api/settings
```

### WebSocket

Single WebSocket endpoint with project subscription:

```
WS     /api/ws
```

**Subscription messages:**

```json
{"type": "subscribe", "project_ids": ["abc123"]}
{"type": "subscribe", "project_ids": ["*"]}
```

**Events include project context:**

```json
{"type": "task_updated", "project_id": "abc123", "task_id": "TASK-001", "data": {...}}
```

UI sends new subscription when navigating between projects.

## CLI Changes

### Project Resolution Order

First match wins:

1. `--project` flag (explicit override)
2. `ORC_PROJECT` env var (for scripting/CI)
3. cwd detection (find `.orc/` in current or parent dirs)
4. Error: "not in a project, use --project or cd to one"

**No global default fallback** - explicit or detected, never assumed.

### Flag Accepts Multiple Formats

```bash
orc new "task" --project abc123              # By ID
orc new "task" --project orc                 # By name (if unique)
orc new "task" --project /home/user/repos/orc  # By path
```

### Commands by Scope

**Project-scoped (need `--project` or cwd):**

| Command | Notes |
|---------|-------|
| `orc new` | Creates task in project |
| `orc run` | Executes task |
| `orc status` | Shows project status |
| `orc show` | Task details |
| `orc resume` | Resume task |
| `orc approve` | Approve gate |
| `orc initiative *` | Initiative operations |
| `orc config` | Project config |

**Global (no project context):**

| Command | Notes |
|---------|-------|
| `orc serve` | Serves ALL projects |
| `orc projects` | List registered projects |
| `orc projects add <path>` | Register project |
| `orc projects remove <id>` | Unregister project |
| `orc workflow *` | Global workflows |

**Special:**

| Command | Notes |
|---------|-------|
| `orc init` | Creates + registers project in cwd |

## Frontend Changes

### Route Structure

```
/                                    → Project picker (landing)
/projects/:id                        → Project dashboard
/projects/:id/tasks                  → Task list
/projects/:id/tasks/:taskId          → Task detail
/projects/:id/initiatives            → Initiatives
/projects/:id/initiatives/:initId    → Initiative detail
/projects/:id/settings               → Project config

/workflows                           → Global workflow list/editor
/workflows/:workflowId               → Workflow detail
/phases                              → Global phase templates
/agents                              → Global agents
/settings                            → Global orc settings
```

### Component Changes

| Component | Change |
|-----------|--------|
| Router | Restructure to nested project routes |
| Project context | URL-driven (extract from route params) |
| API client | Project-scoped calls include `:id` in path |
| Landing page | New: project picker with stats, "Add project" |
| Project switcher | Navigate to `/projects/:newId/tasks` |
| Top bar | Show current project, quick-switch dropdown |
| Workflow editor | Global scope, no project in URL |
| Phase editor | Global scope |
| Agent editor | Global scope |
| WebSocket hook | Resubscribe on project route change |

### Data Fetching Pattern

```tsx
// Project-scoped
function useProjectTasks() {
  const { projectId } = useParams();
  return useQuery(['tasks', projectId], () =>
    api.get(`/projects/${projectId}/tasks`)
  );
}

// Global
function useWorkflows() {
  return useQuery(['workflows'], () =>
    api.get('/workflows')
  );
}
```

### Landing Page Behavior

When navigating to `/`:

1. Show list of all registered projects
2. Each project shows: name, path, active task count, last activity
3. "Add Project" button opens form to register by path
4. Clicking project navigates to `/projects/:id/tasks`
5. No auto-redirect to "default" project

## Server Implementation

### Project DB Cache

```go
type ProjectDBCache struct {
    cache   *lru.Cache[string, *db.ProjectDB]
    mu      sync.RWMutex
    maxSize int
}

func (c *ProjectDBCache) Get(projectID string) (*db.ProjectDB, error) {
    c.mu.RLock()
    if db, ok := c.cache.Get(projectID); ok {
        c.mu.RUnlock()
        return db, nil
    }
    c.mu.RUnlock()

    c.mu.Lock()
    defer c.mu.Unlock()

    // Double-check after acquiring write lock
    if db, ok := c.cache.Get(projectID); ok {
        return db, nil
    }

    // Load project path from registry
    proj, err := project.Get(projectID)
    if err != nil {
        return nil, err
    }

    // Open database
    db, err := db.OpenProjectDB(filepath.Join(proj.Path, ".orc", "orc.db"))
    if err != nil {
        return nil, err
    }

    c.cache.Add(projectID, db)
    return db, nil
}
```

### Request Handling

```go
func (s *Server) handleProjectRequest(w http.ResponseWriter, r *http.Request) {
    projectID := chi.URLParam(r, "projectId")

    db, err := s.projectCache.Get(projectID)
    if err != nil {
        http.Error(w, "project not found", http.StatusNotFound)
        return
    }

    // Handle request with project db
}
```

## Migration

**Breaking change** - intentional, clean break:

1. Update all API routes (no backward-compat shim)
2. Update frontend routes and API calls
3. Update server to multi-tenant model
4. Update CLI to add `--project` flag
5. Remove all single-project binding code

**Data migration:** None required. Project databases stay in place. Registry already exists. Manual registration of existing projects if needed.

## Non-Goals

- Cross-project task search (can add later)
- Unified activity feed across projects (can add later)
- Cross-project dependencies (tasks depending on tasks in other projects)

## Implementation Order

1. **Server:** Project DB cache, extract project ID middleware
2. **API:** Migrate routes to `/api/projects/:id/` structure
3. **Global resources:** Move workflows/phases/agents to global DB
4. **CLI:** Add `--project` flag infrastructure
5. **Frontend:** Router restructure, project picker, API client updates
6. **WebSocket:** Add project_id to events, subscription filtering
7. **Cleanup:** Remove single-project code, old routes, dead code
