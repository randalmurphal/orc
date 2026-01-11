# Orc - Claude Code Task Orchestrator

## Quick Start

```bash
# Install (users)
curl -fsSL https://raw.githubusercontent.com/randalmurphal/orc/main/install.sh | sh

# Or: go install github.com/randalmurphal/orc/cmd/orc@latest

# Development (contributors)
make setup    # Creates go.work, installs bun deps
make build    # Build binary to bin/orc
make test     # Run tests
make dev-full # API (:8080) + frontend (:5173)

# Run
./bin/orc init
./bin/orc new "task description"
./bin/orc run TASK-001
```

## Project Structure

| Path | Purpose |
|------|---------|
| `cmd/orc/` | CLI entry point |
| `internal/` | Core packages (see `internal/CLAUDE.md`) |
| `templates/` | Phase templates (see `templates/CLAUDE.md`) |
| `web/` | Svelte 5 frontend (see `web/CLAUDE.md`) |
| `docs/` | Architecture docs, specs, ADRs (see `docs/CLAUDE.md`) |

### Key Internal Packages

| Package | Purpose | Key Files |
|---------|---------|-----------|
| `api/` | REST API + WebSocket | `server.go` + 16 `handlers_*.go` files |
| `cli/` | Cobra commands | `root.go` + 18 `cmd_*.go` files |
| `executor/` | Phase execution | 12 modules: publish, template, retry, worktree, phase, etc. |
| `events/` | Real-time event publishing | EventPublisher, event types |
| `task/` | Task model + YAML persistence | Task struct, CRUD operations |
| `plan/` | Phase templates + weight classification | Plan generation, phase sequences |
| `state/` | Execution state tracking | Checkpointing, iteration tracking |
| `prompt/` | Prompt template management | Template loading, variable substitution |
| `git/` | Git operations, worktrees | Branch management, checkpoints |
| `project/` | Multi-project registry | Project discovery, registry management |
| `tokenpool/` | OAuth token pool for rate limit failover | Account rotation, state persistence |
| `db/` | SQLite persistence (global + project) | FTS search, cost tracking, migrations |
| `bootstrap/` | Instant project initialization | <500ms init, no prompts |
| `setup/` | Claude-powered interactive setup | Prompt generation, validation |

## Dependencies

Published Go modules (v1.1.0):
- `github.com/randalmurphal/llmkit` - Claude CLI wrapper, templates, model selection
- `github.com/randalmurphal/flowgraph` - Graph-based execution with checkpointing
- `github.com/randalmurphal/devflow` - Git operations, worktree management

For local development, `make setup` creates `go.work` to use sibling directories.

## Automation Profiles

| Profile | Behavior |
|---------|----------|
| `auto` | Fully automated, no human intervention (default) |
| `fast` | Minimal gates, speed over safety |
| `safe` | AI reviews, human only for merge |
| `strict` | Human gates on spec/review/merge |

```bash
orc run TASK-001 --profile safe
orc config profile strict  # Set default
```

## Config Hierarchy

Configuration loads from multiple sources (later overrides earlier):

| Priority | Source | Location |
|----------|--------|----------|
| 1 | Defaults | Built-in |
| 2 | System | `/etc/orc/config.yaml` |
| 3 | User | `~/.orc/config.yaml` |
| 4 | Project | `.orc/config.yaml` |
| 5 | Environment | `ORC_*` variables |

**Environment Variable Overrides:**
```bash
ORC_PROFILE=strict          # profile
ORC_MODEL=claude-sonnet     # model
ORC_MAX_ITERATIONS=50       # max_iterations
ORC_TIMEOUT=5m              # timeout
ORC_RETRY_ENABLED=false     # retry.enabled
ORC_GATES_DEFAULT=human     # gates.default_type
ORC_WORKTREE_ENABLED=false  # worktree.enabled
```

**View config sources:**
```bash
orc config show --source
# profile = strict (from env ORC_PROFILE)
# model = claude-sonnet (from project)
# retry.enabled = true (from user)
```

## Task Weight → Phases

| Weight | Phases |
|--------|--------|
| trivial | implement |
| small | implement → test |
| medium | implement → test → docs |
| large | spec → implement → test → docs → validate |
| greenfield | research → spec → implement → test → docs → validate |

All phases use **auto gates by default**. Config/profile can override.

## Cross-Phase Retry

If tests fail, orc automatically retries from implementation:
- `test` → retry from `implement`
- `validate` → retry from `implement`

The retry phase receives **{{RETRY_CONTEXT}}** with:
- What phase failed and why
- Output from the failed phase
- Which retry attempt this is

Configurable via:
```yaml
retry:
  enabled: true
  max_retries: 3
  retry_map:
    test: implement
    validate: implement
```

## Review Configuration

Multi-round code review runs after implementation (when enabled).

```yaml
review:
  enabled: true           # Enable review phase (default: true)
  rounds: 2               # Number of review rounds (default: 2)
  require_pass: true      # Must pass review to continue
```

**Review Process:**
1. **Round 1 (Exploratory)**: Identifies gaps, issues, architectural concerns
2. **Round 2 (Validation)**: Verifies all Round 1 issues were addressed

Review outputs use XML format:
```xml
<review_decision>
  <status>pass|fail|needs_user_input</status>
  <gaps_addressed>true|false</gaps_addressed>
  <summary>Review summary...</summary>
</review_decision>
```

## QA Configuration

QA session runs after review passes (when enabled).

```yaml
qa:
  enabled: true           # Run QA phase (default: true)
  skip_for_weights:       # Skip QA for these weights
    - trivial
  require_e2e: false      # Require e2e tests to pass
  generate_docs: true     # Auto-generate feature docs
```

**QA outputs:**
```xml
<qa_result>
  <status>pass|fail|needs_attention</status>
  <summary>QA summary...</summary>
  <tests_run>
    <total>10</total>
    <passed>9</passed>
    <failed>1</failed>
  </tests_run>
</qa_result>
```

## Sub-task Queue

Agents can propose sub-tasks during implementation for later review.

```yaml
subtasks:
  allow_creation: true    # Agents can propose sub-tasks
  auto_approve: false     # Require human approval
  max_pending: 10         # Max queued per task
```

Sub-tasks are managed through the API and web UI. Agents propose via XML:
```xml
<subtask_proposal>
  <title>Refactor auth module</title>
  <description>Split auth into smaller modules</description>
  <parent_task>TASK-001</parent_task>
</subtask_proposal>
```

## Completion Detection

Phases complete when Claude outputs:
```xml
<phase_complete>true</phase_complete>
```

Phases block when Claude outputs:
```xml
<phase_blocked>reason: ...</phase_blocked>
```

## Multi-Project Support

Orc supports multiple projects through a global registry.

**Global registry:** `~/.orc/projects.yaml`
```yaml
projects:
  - id: abc123
    name: orc
    path: /home/randy/repos/orc
    created_at: 2025-01-10T14:00:00Z
```

**Commands:**
- `orc init` - Initializes project and registers in global registry
- `orc serve` - Serves all registered projects

**UI:** Project dropdown in header to switch between projects.

## Organization Model

Every user is part of an organization (even solo developers are an "org of 1"). Features are opt-in with sensible defaults for solo development.

```yaml
team:
  name: ""                    # Organization name (auto-detected from username)
  activity_logging: true      # Log all actions as history (default: on)
  task_claiming: false        # Task assignment features (default: off for solo)
  visibility: all             # all | assigned | owned
  mode: local                 # local | shared_db | sync_server (future)
  server_url: ""              # For sync_server mode
```

**Environment Variables:**
```bash
ORC_TEAM_NAME=myteam              # team.name
ORC_TEAM_ACTIVITY_LOG=true        # team.activity_logging
ORC_TEAM_TASK_CLAIMING=true       # team.task_claiming
ORC_TEAM_VISIBILITY=all           # team.visibility
ORC_TEAM_MODE=shared_db           # team.mode
```

**Modes:**
- `local` - Single user, local database (default)
- `shared_db` - Multiple users, shared PostgreSQL database
- `sync_server` - Future: distributed sync server

**Features by Mode:**

| Feature | local | shared_db |
|---------|-------|-----------|
| Activity logging | ✓ | ✓ |
| Task claiming | ✓ | ✓ |
| Real-time sync | - | ✓ |
| Team members | 1 | Many |

## Worktree Isolation

Tasks run in isolated git worktrees for parallel execution.

```yaml
worktree:
  enabled: true                    # Enable worktree isolation (default: true)
  dir: ".orc/worktrees"           # Worktree directory
  cleanup_on_complete: true        # Remove on success (default: true)
  cleanup_on_fail: false           # Keep on failure for debugging
```

**Layout:**
```
.orc/worktrees/
├── orc-task-001/    # Isolated worktree for TASK-001
└── orc-task-002/    # Another task running in parallel
```

## Token Pool

Automatic OAuth token rotation when rate limits are hit.

```yaml
pool:
  enabled: true
  config_path: ~/.orc/token-pool/pool.yaml
```

**Pool configuration (`~/.orc/token-pool/pool.yaml`):**
```yaml
version: 1
strategy: round-robin    # round-robin | failover | lowest-utilization (future)
switch_on_rate_limit: true
accounts:
  - id: personal
    name: "Personal Max"
    access_token: "sk-ant-oat01-..."
    refresh_token: "sk-ant-ort01-..."
    enabled: true
```

**Features:**
- Round-robin account selection
- Automatic switching on rate limit errors
- Session continuity via `CLAUDE_CODE_OAUTH_TOKEN` override
- State persistence (exhausted flags survive restarts)

**Setup:**
```bash
orc pool init              # Initialize ~/.orc/token-pool/
orc pool add personal      # Authenticate via claude login
orc pool add work          # Add another account
orc pool list              # View accounts
```

## Completion Actions

After all phases complete, orc can auto-merge or create a PR.

```yaml
completion:
  action: pr              # pr | merge | none (default: pr)
  target_branch: main     # Branch to merge into
  delete_branch: true     # Delete task branch after merge

  pr:
    title: "[orc] {{TASK_TITLE}}"
    body_template: templates/pr-body.md
    labels: [automated]
    reviewers: []
    draft: false
    auto_merge: true      # Enable auto-merge when approved
```

## File Layout

```
~/.orc/
├── orc.db               # Global SQLite (projects, cost logs, templates)
├── config.yaml          # User-level config (applies to all projects)
├── projects.yaml        # Global project registry
└── token-pool/          # OAuth token pool
    ├── pool.yaml        # Pool configuration + tokens
    └── state.yaml       # Runtime state (current account, exhausted flags)

~/.claude/
├── settings.json        # Global Claude Code settings
└── CLAUDE.md            # Global instructions

~/CLAUDE.md              # User-level instructions

.orc/
├── orc.db               # Project SQLite (tasks, phases, transcripts FTS)
├── config.yaml
├── prompts/             # Project prompt overrides
│   └── implement.md
├── worktrees/           # Isolated worktrees for tasks
└── tasks/TASK-001/
    ├── task.yaml        # Definition
    ├── plan.yaml        # Phase sequence
    ├── state.yaml       # Execution state
    └── transcripts/     # Claude conversation logs

.claude/
├── settings.json        # Project settings (hooks, env, plugins)
├── plugins/             # Claude Code plugins (slash commands)
│   └── orc/
│       ├── .claude-plugin/
│       │   └── plugin.json
│       └── commands/    # /orc:init, /orc:continue, etc.
├── skills/              # Claude Code skills (SKILL.md format)
│   └── my-skill/
│       └── SKILL.md     # YAML frontmatter + markdown body
└── CLAUDE.md            # Project instructions
```

## Claude Code Integration

Orc installs a Claude Code plugin providing slash commands:

| Command | Purpose |
|---------|---------|
| `/orc:init` | Initialize project or create new spec |
| `/orc:continue` | Resume current task from checkpoint |
| `/orc:status` | Show current task progress |
| `/orc:review` | Start multi-round code review |
| `/orc:qa` | Run QA session (tests, docs) |
| `/orc:propose` | Queue sub-task for later review |

The plugin is installed automatically by `orc init` to `.claude/plugins/orc/`.

## Commands

| Command | Purpose |
|---------|---------|
| `orc go` | Main entry point - interactive guidance or quick execution |
| `orc go --headless` | Automated execution, no user interaction |
| `orc go "description"` | Quick mode - create and execute single task |
| `orc go --stream` | Stream Claude transcript to stdout |
| `orc init` | Initialize .orc/ in current directory (instant, <500ms) |
| `orc setup` | Claude-powered interactive project setup |
| `orc new "title"` | Create task, classify weight, generate plan |
| `orc run TASK-ID` | Execute task phases (auto by default) |
| `orc run TASK-ID -p safe` | Execute with specific profile |
| `orc serve` | Start API server for web UI |
| `orc config` | Show/set configuration |
| `orc config profile X` | Set automation profile |
| `orc pause TASK-ID` | Pause execution, save state |
| `orc resume TASK-ID` | Continue from checkpoint |
| `orc rewind TASK-ID --to X` | Reset to before phase X |
| `orc status` | Show running tasks |
| `orc delete TASK-ID` | Delete task and its files |
| `orc diff TASK-ID` | Show git diff for task changes |
| `orc pool init` | Initialize token pool directory |
| `orc pool add <name>` | Add OAuth account to pool |
| `orc pool list` | List accounts in pool |
| `orc pool status` | Show account exhaustion status |
| `orc pool switch <id>` | Manually switch account |
| `orc pool remove <id>` | Remove account from pool |
| `orc pool reset` | Clear exhausted flags |
| `orc export TASK-ID` | Export task to YAML (with --transcripts, --state) |
| `orc import <file>` | Import task from YAML (with --force) |
| `orc initiative new <title>` | Create initiative to group related tasks |
| `orc initiative list` | List all initiatives |
| `orc initiative show <id>` | Show initiative details |
| `orc initiative add-task <init-id> <task-id>` | Link task to initiative |
| `orc initiative decide <init-id> <decision>` | Record decision |
| `orc initiative run <id>` | Run all initiative tasks in order |

## Web UI

```bash
# Install frontend dependencies (first time)
make web-install

# Development (start both servers)
make serve          # API on :8080
make web-dev        # Frontend on :5173

# Production build
make web-build      # Outputs to web/build/

# E2E tests
make e2e            # Run Playwright tests
```

### Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `⌘ K` | Open command palette |
| `⌘ N` | Create new task |
| `⌘ B` | Toggle sidebar |
| `⌘ P` | Switch project |
| `/` | Focus search |
| `?` | Show keyboard shortcuts help |
| `Esc` | Close overlay |

**Navigation Sequences:**
| Sequence | Action |
|----------|--------|
| `g d` | Go to dashboard |
| `g t` | Go to tasks |
| `g s` | Go to settings |
| `g p` | Go to prompts |
| `g h` | Go to hooks |
| `g k` | Go to skills |

**Task List (on Tasks page):**
| Shortcut | Action |
|----------|--------|
| `j` | Select next task |
| `k` | Select previous task |
| `Enter` | Open selected task |
| `r` | Run selected task |
| `p` | Pause selected task |
| `d` | Delete selected task |

### Toast Notifications

The UI shows toast notifications for:
- Task state changes (completed, failed, blocked)
- Phase completions
- User actions (run, pause, delete)

Toasts appear in the top-right corner and auto-dismiss after a configurable duration.

### Dashboard

The dashboard (`/dashboard`) displays:
- **Quick Stats**: Running, Blocked, Today's tasks, Token usage
- **Connection Status**: Live/Connecting/Offline WebSocket indicator
- **Active Tasks**: Currently running or paused tasks
- **Recent Activity**: Recently completed or failed tasks

## API Endpoints

### Projects
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects` | List registered projects |
| GET | `/api/projects/:id` | Get project details |
| GET | `/api/projects/:id/tasks` | List tasks for project |
| POST | `/api/projects/:id/tasks` | Create task in project |
| GET | `/api/projects/:id/tasks/:taskId` | Get specific task |
| DELETE | `/api/projects/:id/tasks/:taskId` | Delete task |
| POST | `/api/projects/:id/tasks/:taskId/run` | Start task execution |
| POST | `/api/projects/:id/tasks/:taskId/pause` | Pause running task |
| POST | `/api/projects/:id/tasks/:taskId/resume` | Resume paused task |
| POST | `/api/projects/:id/tasks/:taskId/rewind` | Rewind to phase (body: {phase}) |
| GET | `/api/projects/:id/tasks/:taskId/state` | Get task state |
| GET | `/api/projects/:id/tasks/:taskId/plan` | Get task plan |
| GET | `/api/projects/:id/tasks/:taskId/transcripts` | Get task transcripts |

### Tasks (Global - CWD-based)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/tasks` | List tasks (supports `?page=N&limit=N`) |
| POST | `/api/tasks` | Create task |
| GET | `/api/tasks/:id` | Get task |
| DELETE | `/api/tasks/:id` | Delete task |
| GET | `/api/tasks/:id/state` | Get execution state |
| GET | `/api/tasks/:id/plan` | Get task plan |
| GET | `/api/tasks/:id/transcripts` | Get transcripts |
| POST | `/api/tasks/:id/run` | Start task |
| POST | `/api/tasks/:id/pause` | Pause task |
| POST | `/api/tasks/:id/resume` | Resume task |

### Project Tasks (Multi-project support)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/projects/:id/tasks/:taskId` | Get task in project |
| DELETE | `/api/projects/:id/tasks/:taskId` | Delete task in project |
| GET | `/api/projects/:id/tasks/:taskId/state` | Get execution state |
| GET | `/api/projects/:id/tasks/:taskId/plan` | Get task plan |
| GET | `/api/projects/:id/tasks/:taskId/transcripts` | Get transcripts |
| POST | `/api/projects/:id/tasks/:taskId/run` | Start task |
| POST | `/api/projects/:id/tasks/:taskId/pause` | Pause running task |
| POST | `/api/projects/:id/tasks/:taskId/resume` | Resume paused task |
| POST | `/api/projects/:id/tasks/:taskId/rewind` | Rewind to phase (body: `{"phase": "implement"}`) |

### Initiatives
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/initiatives` | List initiatives (query: ?status=active, ?shared=true) |
| POST | `/api/initiatives` | Create initiative |
| GET | `/api/initiatives/:id` | Get initiative |
| PUT | `/api/initiatives/:id` | Update initiative |
| DELETE | `/api/initiatives/:id` | Delete initiative |
| GET | `/api/initiatives/:id/tasks` | List initiative tasks |
| POST | `/api/initiatives/:id/tasks` | Add task to initiative |
| POST | `/api/initiatives/:id/decisions` | Add decision |
| GET | `/api/initiatives/:id/ready` | Get tasks ready to run |

### Prompts
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/prompts` | List prompts |
| GET | `/api/prompts/variables` | Get template variables |
| GET | `/api/prompts/:phase` | Get prompt for phase |
| GET | `/api/prompts/:phase/default` | Get default prompt |
| PUT | `/api/prompts/:phase` | Save prompt override |
| DELETE | `/api/prompts/:phase` | Delete prompt override |

### Hooks (settings.json format)
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/hooks` | List all hooks (map of event → hooks) |
| GET | `/api/hooks/types` | Get valid hook event types |
| POST | `/api/hooks` | Create hook (event + matcher + command) |
| GET | `/api/hooks/:event` | Get hooks for event type |
| PUT | `/api/hooks/:event` | Update hooks for event |
| DELETE | `/api/hooks/:event` | Delete all hooks for event |

### Skills (SKILL.md format)
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
| GET | `/api/cost/summary` | Get cost summary (query: period=day\|week\|month\|all, since=RFC3339) |

### GitHub PR Integration
| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/api/tasks/:id/github/pr` | Create PR for task branch |
| GET | `/api/tasks/:id/github/pr` | Get PR details, comments, and checks |
| POST | `/api/tasks/:id/github/pr/merge` | Merge the PR (body: `{"method": "squash", "delete_branch": true}`) |
| POST | `/api/tasks/:id/github/pr/comments/sync` | Sync local review comments to PR |
| POST | `/api/tasks/:id/github/pr/comments/:commentId/autofix` | Queue auto-fix for a review comment |
| GET | `/api/tasks/:id/github/pr/checks` | Get CI check run status |

### Config & Real-time
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/config` | Get orc configuration |
| PUT | `/api/config` | Update orc configuration |
| GET | `/api/ws` | WebSocket for real-time updates |
| GET | `/api/tasks/:id/stream` | SSE transcript stream (legacy) |

## WebSocket Protocol

Connect to `/api/ws` for real-time updates.

### Client → Server Messages
```json
{"type": "subscribe", "task_id": "TASK-001"}
{"type": "unsubscribe"}
{"type": "command", "task_id": "TASK-001", "action": "pause"}
{"type": "ping"}
```

### Server → Client Messages
```json
{"type": "subscribed", "task_id": "TASK-001"}
{"type": "event", "event_type": "state", "data": {...}}
{"type": "event", "event_type": "transcript", "data": {...}}
{"type": "event", "event_type": "phase", "data": {...}}
{"type": "pong"}
```

## Key Patterns

**Error handling**: Always wrap with context
```go
return fmt.Errorf("load task %s: %w", id, err)
```

**Phase execution**: flowgraph with Ralph-style loop
```go
graph := flowgraph.NewGraph[PhaseState]()
graph.SetEntry("prompt")
graph.AddConditionalEdge("check", routerFunc)
```

**Git commits**: After every phase completion
```
[orc] TASK-001: implement - completed
```

**Event publishing**: Real-time updates during execution
```go
publisher.Publish(events.Event{
    Type:   events.EventTranscript,
    TaskID: taskID,
    Data:   transcriptLine,
})
```

## Container Usage

```bash
# Development shell
make dev

# Run tests in container
make docker-test

# Build production binary
make release-build
```

## Testing

```bash
# Backend tests
make test

# Frontend unit tests
make web-test

# E2E tests with Playwright
make e2e
```

## Docs Reference

| Topic | Path |
|-------|------|
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phases | `docs/architecture/PHASE_MODEL.md` |
| Gates | `docs/architecture/GATES.md` |
| CLI Spec | `docs/specs/CLI.md` |
| File Formats | `docs/specs/FILE_FORMATS.md` |
