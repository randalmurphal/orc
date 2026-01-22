# API Endpoints

Full endpoint reference for the REST API.

## Handler Files

| File | Endpoints | Description |
|------|-----------|-------------|
| `handlers_tasks.go` | `/api/tasks/*` | Task CRUD |
| `handlers_attachments.go` | `attachments` | File uploads |
| `handlers_tasks_control.go` | `run`, `pause`, `resume` | Task control |
| `handlers_tasks_state.go` | `state`, `plan`, `transcripts`, `stream` | Task state |
| `handlers_finalize.go` | `finalize`, `finalize/status` | Finalize ops |
| `handlers_projects.go` | `/api/projects/*` | Project-scoped tasks |
| `handlers_prompts.go` | `/api/prompts/*` | Prompt templates |
| `handlers_hooks.go` | `/api/hooks/*` | Hook config |
| `handlers_skills.go` | `/api/skills/*` | Skills (SKILL.md) |
| `handlers_settings.go` | `/api/settings/*` | Settings |
| `handlers_tools.go` | `/api/tools/*` | Tool permissions |
| `handlers_agents.go` | `/api/agents/*` | Sub-agents |
| `handlers_scripts.go` | `/api/scripts/*` | Script registry |
| `handlers_claudemd.go` | `/api/claudemd/*` | CLAUDE.md hierarchy |
| `handlers_mcp.go` | `/api/mcp/*` | MCP servers |
| `handlers_templates.go` | `/api/templates/*` | Templates |
| `handlers_config.go` | `/api/config/*` | Orc config |
| `handlers_dashboard.go` | `/api/dashboard/*` | Dashboard stats |
| `handlers_stats.go` | `/api/stats/*` | Activity heatmap data |
| `handlers_metrics.go` | `/api/metrics/*` | JSONL-based analytics |
| `handlers_diff.go` | `/api/tasks/:id/diff/*` | Git diffs |
| `handlers_github.go` | `/api/tasks/:id/github/*` | GitHub PRs |
| `handlers_initiatives.go` | `/api/initiatives/*` | Initiatives |

## Task Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tasks` | List all tasks |
| POST | `/api/tasks` | Create task |
| GET | `/api/tasks/:id` | Get task |
| PUT | `/api/tasks/:id` | Update task |
| DELETE | `/api/tasks/:id` | Delete task |
| POST | `/api/tasks/:id/run` | Run task |
| POST | `/api/tasks/:id/pause` | Pause task |
| POST | `/api/tasks/:id/resume` | Resume task |
| GET | `/api/tasks/:id/state` | Get state |
| GET | `/api/tasks/:id/plan` | Get plan |
| GET | `/api/tasks/:id/transcripts` | Get transcripts |
| GET | `/api/tasks/:id/stream` | Stream transcripts (SSE) |
| POST | `/api/tasks/:id/finalize` | Start finalize |
| GET | `/api/tasks/:id/finalize/status` | Get finalize status |
| POST | `/api/tasks/:id/finalize/cancel` | Cancel finalize |

## Attachment Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tasks/:id/attachments` | List attachments |
| POST | `/api/tasks/:id/attachments` | Upload file |
| GET | `/api/tasks/:id/attachments/:filename` | Download file |
| DELETE | `/api/tasks/:id/attachments/:filename` | Delete file |

## GitHub Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tasks/:id/github/pr` | Get PR info |
| POST | `/api/tasks/:id/github/pr/refresh` | Refresh PR status |

## Diff Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/tasks/:id/diff` | Get task diff |
| GET | `/api/tasks/:id/diff/comments` | Get inline comments |
| POST | `/api/tasks/:id/diff/comments` | Add inline comment |

## Initiative Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/initiatives` | List initiatives |
| POST | `/api/initiatives` | Create initiative |
| GET | `/api/initiatives/:id` | Get initiative |
| PUT | `/api/initiatives/:id` | Update initiative |
| DELETE | `/api/initiatives/:id` | Delete initiative |
| GET | `/api/initiatives/:id/dependency-graph` | Get dependency graph |

## Project Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/projects` | List projects |
| GET | `/api/projects/:id/tasks` | List project tasks |
| POST | `/api/projects/:id/tasks` | Create project task |

## Config Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/config` | Get config |
| PUT | `/api/config` | Update config |
| GET | `/api/settings` | Get settings |
| PUT | `/api/settings` | Update settings |
| PUT | `/api/settings/global` | Update global settings |

## Claude Code Endpoints

All support `?scope=global` for user-level config (`~/.claude/`):

| Method | Path | Description |
|--------|------|-------------|
| GET/PUT | `/api/skills` | Skills |
| GET/PUT | `/api/hooks` | Hooks |
| GET/PUT | `/api/agents` | Agents |
| GET/PUT | `/api/mcp` | MCP servers |
| GET/PUT | `/api/claudemd` | CLAUDE.md |
| GET/PUT | `/api/prompts` | Prompts |
| GET/PUT | `/api/tools` | Tools |

## Dashboard Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/dashboard/stats` | Get dashboard stats |

## Stats Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/stats/activity` | Get activity data for heatmap (`?weeks=16`) |

## Metrics Endpoints

JSONL-based analytics from Claude Code session files.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/metrics/summary?since=7d` | Aggregated metrics summary (cost, tokens, task count, by model) |
| GET | `/api/metrics/daily?since=30d` | Daily aggregated metrics for charts |
| GET | `/api/metrics/by-model?since=7d` | Per-model breakdown |
| GET | `/api/tasks/:id/metrics` | Task-specific metrics by phase |
| GET | `/api/tasks/:id/tokens` | Task token usage (prefers DB, falls back to state) |
| GET | `/api/tasks/:id/todos` | Latest todo snapshot for task |
| GET | `/api/tasks/:id/todos/history` | Todo snapshot history (progress timeline) |

**Query parameters:**
- `since`: Time period filter. Supports: `1h`, `7d`, `30d`, `2w`, `1m`. Defaults to `7d`.

## WebSocket

| Path | Description |
|------|-------------|
| `/api/ws` | WebSocket for real-time events |
