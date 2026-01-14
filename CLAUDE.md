# Orc - Claude Code Task Orchestrator

AI-powered task orchestration with phased execution, git worktree isolation, and multi-round review.

## Quick Start

```bash
# Install
go install github.com/randalmurphal/orc/cmd/orc@latest

# Development
make setup && make build    # Build to bin/orc
make test                   # Run tests
make dev-full               # API (:8080) + frontend (:5173)

# Run
./bin/orc init && ./bin/orc new "task description" && ./bin/orc run TASK-001
```

## Claude Code Plugin

The orc plugin for Claude Code lives in a separate lightweight repo to avoid cloning the full codebase:

**Repo:** [randalmurphal/orc-claude-plugin](https://github.com/randalmurphal/orc-claude-plugin)

Install in Claude Code (run once):
```
/plugin marketplace add randalmurphal/orc-claude-plugin
/plugin install orc@orc
```

Commands: `/orc:init`, `/orc:status`, `/orc:continue`, `/orc:review`, `/orc:qa`, `/orc:propose`

## Project Structure

| Path | Purpose | Details |
|------|---------|---------|
| `cmd/orc/` | CLI entry point | - |
| `internal/` | Core packages | See `internal/CLAUDE.md` |
| `templates/` | Phase prompts | See `templates/CLAUDE.md` |
| `web/` | Svelte 5 frontend | See `web/CLAUDE.md` |
| `docs/` | Architecture, specs, ADRs | See `docs/CLAUDE.md` |

**Key packages:** `api/` (REST + WebSocket), `cli/` (Cobra), `executor/` (phase engine), `task/` (YAML persistence), `git/` (worktrees), `db/` (SQLite), `watcher/` (live refresh)

## Task Execution Model

### Task Organization

Tasks support queue and priority organization to prevent cognitive overload:

| Property | Values | Purpose |
|----------|--------|---------|
| Queue | `active`, `backlog` | Separates current work from "someday" items |
| Priority | `critical`, `high`, `normal`, `low` | Urgency within a queue |

**Queues:**
- **Active** (default): Current work shown on the board
- **Backlog**: Deferred tasks, collapsed by default in each column

**Priorities:**
- **Critical**: Urgent, needs immediate attention (pulsing indicator)
- **High**: Important, should be done soon
- **Normal**: Default priority
- **Low**: Can wait

Tasks are sorted by priority within each column. Higher priority tasks appear first.

### Weight Classification

| Weight | Phases | Use Case |
|--------|--------|----------|
| trivial | implement | One-liner fix |
| small | implement → test | Bug fix, small feature |
| medium | implement → test → docs | Feature with tests |
| large | spec → implement → test → docs → validate | Complex feature |
| greenfield | research → spec → implement → test → docs → validate | New system |

### Completion Detection

Phases complete when Claude outputs:
```xml
<phase_complete>true</phase_complete>
```

Phases block when:
```xml
<phase_blocked>reason: ...</phase_blocked>
```

### Cross-Phase Retry

Failed phases trigger automatic retry from earlier phase:
- `test` fails → retry from `implement`
- `validate` fails → retry from `implement`

Retry phase receives `{{RETRY_CONTEXT}}` with failure details.

### Artifact Detection

When running a task, orc detects existing artifacts and offers to skip phases:

```
📄 spec.md already exists. Skip spec phase? [Y/n]:
```

| Phase | Auto-Skippable | Artifacts Detected |
|-------|----------------|-------------------|
| spec | Yes | `spec.md` with valid content |
| research | Yes | `artifacts/research.md` or in spec |
| docs | Yes | `artifacts/docs.md` |
| implement | No | Never (too complex) |
| test | No | Must re-run against current code |
| validate | No | Must verify current state |

Use `--auto-skip` to skip automatically without prompting. Skip reasons are recorded in `state.yaml`.

## Configuration

**Hierarchy** (later overrides earlier): Defaults → `/etc/orc/` → `~/.orc/` → `.orc/` → `ORC_*` env

### Automation Profiles

| Profile | Behavior |
|---------|----------|
| `auto` | Fully automated (default) |
| `fast` | Minimal gates, speed over safety |
| `safe` | AI reviews, human for merge |
| `strict` | Human gates on spec/review/merge |

```bash
orc run TASK-001 --profile safe
orc config profile strict
```

### Key Config Options

| Option | Purpose | Docs |
|--------|---------|------|
| `storage.mode` | hybrid/files/database | `docs/specs/DATABASE_ABSTRACTION.md` |
| `worktree.enabled` | Git worktree isolation | `docs/architecture/GIT_INTEGRATION.md` |
| `pool.enabled` | OAuth token rotation | - |
| `team.mode` | local/shared_db | `docs/specs/TEAM_ARCHITECTURE.md` |
| `completion.action` | pr/merge/none | - |
| `completion.sync.strategy` | Branch sync timing | `docs/architecture/GIT_INTEGRATION.md` |
| `artifact_skip.enabled` | Detect existing artifacts | `docs/architecture/PHASE_MODEL.md` |
| `artifact_skip.auto_skip` | Skip without prompting | `docs/architecture/PHASE_MODEL.md` |

**All config:** `orc config docs` or `docs/specs/CONFIG_HIERARCHY.md`

## File Layout

```
~/.orc/                          # Global
├── orc.db, config.yaml, projects.yaml, token-pool/

.orc/                            # Project
├── orc.db, config.yaml
├── prompts/                     # Phase prompt overrides
├── worktrees/                   # Isolated worktrees
└── tasks/TASK-001/
    ├── task.yaml, plan.yaml, state.yaml, spec.md
    ├── transcripts/
    ├── attachments/             # Task attachments (images, files)
    └── test-results/            # Playwright test results
        ├── report.json, index.html
        ├── screenshots/
        └── traces/

.claude/                         # Claude Code
├── settings.json, hooks/, skills/

.mcp.json                        # MCP server configuration (auto-generated for UI tasks)
```

## Commands

| Command | Purpose |
|---------|---------|
| `orc go` | Main entry (interactive/headless/quick) |
| `orc init` | Initialize project (<500ms) |
| `orc new "title"` | Create task, classify weight |
| `orc run TASK-ID` | Execute phases |
| `orc plan TASK-ID` | Interactive spec creation |
| `orc status` | Show running/blocked/paused |
| `orc log TASK-ID --follow` | Stream transcript |
| `orc knowledge status` | Knowledge queue stats |

**Full CLI:** `internal/cli/CLAUDE.md` | **Pool:** `orc pool --help` | **Initiatives:** `orc initiative --help`

## Key Patterns

**Error handling:** Always wrap with context
```go
return fmt.Errorf("load task %s: %w", id, err)
```

**Phase execution:** flowgraph with Ralph-style loop
```go
graph := flowgraph.NewGraph[PhaseState]()
graph.AddConditionalEdge("check", routerFunc)
```

**Git commits:** After every phase
```
[orc] TASK-001: implement - completed
```

## Dependencies

Go modules: `llmkit` (Claude wrapper), `flowgraph` (execution), `devflow` (git ops)

For local dev: `make setup` creates `go.work` for sibling directories.

## Web UI

```bash
make serve      # API :8080
make web-dev    # Frontend :5173
```

**Live refresh:** Task board auto-updates when tasks are created/modified/deleted via CLI or filesystem. File watcher monitors `.orc/tasks/` and broadcasts events over WebSocket.

**Project selection:** The server can run from any directory. Project selection persists in URL (`?project=xxx`) and localStorage, surviving page refresh. Use `Cmd+P` to switch projects.

**Keyboard shortcuts:** `Cmd+K` (palette), `Cmd+N` (new task), `g t` (tasks), `j/k` (navigate)

See `web/CLAUDE.md` for component architecture.

## Documentation Reference

| Topic | Location |
|-------|----------|
| API Endpoints | `docs/API_REFERENCE.md` |
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phase Model | `docs/architecture/PHASE_MODEL.md` |
| Executor | `docs/architecture/EXECUTOR.md` |
| Config | `docs/specs/CONFIG_HIERARCHY.md` |
| CLI Spec | `docs/specs/CLI.md` |
| File Formats | `docs/specs/FILE_FORMATS.md` |
| Troubleshooting | `docs/guides/TROUBLESHOOTING.md` |

## Testing

```bash
make test       # Backend (Go)
make web-test   # Frontend (Vitest)
make e2e        # E2E (Playwright)
```

## UI Testing with Playwright MCP

Tasks involving UI changes automatically get Playwright MCP tools for E2E testing.

### Auto-Detection

When a task's title or description contains UI-related keywords (`button`, `form`, `page`, `modal`, `component`, etc.), orc:
1. Sets `requires_ui_testing: true` in task.yaml
2. Configures Playwright MCP server in `.mcp.json`
3. Creates screenshot directory at `.orc/tasks/{id}/test-results/screenshots/`

### Playwright MCP Tools

| Tool | Purpose |
|------|---------|
| `browser_navigate` | Load pages/routes |
| `browser_snapshot` | Capture accessibility tree (preferred for state verification) |
| `browser_click` | Click elements by ref from snapshot |
| `browser_type` | Type text into inputs |
| `browser_fill_form` | Fill multiple form fields |
| `browser_take_screenshot` | Visual verification |
| `browser_console_messages` | Check for JavaScript errors |
| `browser_network_requests` | Verify API calls |

### Test Results

Results are stored in `.orc/tasks/{id}/test-results/`:
- `report.json` - Structured test report
- `screenshots/` - Test screenshots
- `traces/` - Playwright traces
- `index.html` - HTML report (if generated)

See `docs/specs/FILE_FORMATS.md` for full format specification.

<!-- orc:begin -->
## Orc Orchestration

This project uses [orc](https://github.com/randalmurphal/orc) for task orchestration.

### Slash Commands

| Command | Purpose |
|---------|---------|
| `/orc:init` | Initialize project or create spec |
| `/orc:continue` | Resume current task |
| `/orc:status` | Show progress and next steps |
| `/orc:review` | Multi-round code review |
| `/orc:qa` | E2E tests and documentation |
| `/orc:propose` | Create sub-task for later |

### Task Files

Task specifications and state are stored in `.orc/tasks/`:

```
.orc/tasks/TASK-001/
├── task.yaml      # Task metadata
├── spec.md        # Task specification
├── plan.yaml      # Phase sequence
├── state.yaml     # Execution state
└── attachments/   # Images, files (for screenshots, etc.)
```

### CLI Commands

```bash
orc status           # View active tasks
orc run TASK-001     # Execute task
orc pause TASK-001   # Pause execution
orc resume TASK-001  # Continue task
```

See `.orc/` for configuration and task details.

<!-- orc:end -->

<!-- orc:knowledge:begin -->
## Project Knowledge

Patterns, gotchas, and decisions learned during development.

### Patterns Learned
| Pattern | Description | Source |
|---------|-------------|--------|
| Branch sync before completion | Task branches rebase onto target before PR to catch conflicts early | TASK-019 |
| Executor PID tracking | Track executor PID + heartbeat in state.yaml to detect orphaned tasks (running but executor dead) | TASK-046 |
| Atomic status+phase updates | Set `current_phase` atomically with `status=running` to avoid UI timing issues (task shows in wrong column) | TASK-057 |
| Plan regeneration on weight change | When task weight changes, plan.yaml auto-regenerates with new phases; completed/skipped phases preserved if they exist in both plans | TASK-003 |
| Artifact detection for phase skip | Before running phases, check if artifacts exist (spec.md, research.md, docs.md) and offer to skip; use `--auto-skip` for non-interactive mode | TASK-004 |
| Project selection persistence | URL param (`?project=xxx`) takes precedence over localStorage; enables shareable links and browser back/forward navigation | TASK-009 |
| Running task visual indicator | Running tasks show pulsing border + gradient background; placed in column matching `current_phase` from state.yaml | TASK-006 |

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
| PR labels in config don't exist on repo | Orc warns and creates PR without labels (graceful degradation) | TASK-015 |
| `go:embed` fails without static dir | Run `make test` (creates placeholder) or `mkdir -p internal/api/static` | TASK-016 |
| Tests fail with `go.work` | Use `GOWORK=off go test` or `make test` | TASK-016 |
| Raw `InputTokens` appears misleadingly low | Use `EffectiveInputTokens()` which adds cached tokens to get actual context size | TASK-010 |
| Task stuck in "running" after crash | Use `orc resume TASK-XXX` (auto-detects orphaned state) or `--force` to override | TASK-046 |
| Spurious "Task deleted" toast notifications | Fixed: Watcher now verifies deletions with debounce to filter false positives from git ops/atomic saves | TASK-053 |
| Setup errors (worktree creation) failed silently | Fixed: Errors now always display even in quiet mode, task status set to failed | TASK-044 |
| Web UI shows "No project selected" | Select a project via `Cmd+P` - server can run from any directory | TASK-005 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| Sync at completion (default) | Balance safety vs overhead; phase-level sync adds latency for marginal benefit | TASK-019 |

<!-- orc:knowledge:end -->
