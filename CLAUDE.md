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

**Key packages:** `api/` (REST + WebSocket), `cli/` (Cobra), `executor/` (phase engine), `task/` (YAML persistence), `git/` (worktrees), `db/` (SQLite)

## Task Execution Model

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
    └── transcripts/

.claude/                         # Claude Code
├── settings.json, hooks/, skills/
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
└── state.yaml     # Execution state
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

### Known Gotchas
| Issue | Resolution | Source |
|-------|------------|--------|
| PR labels in config don't exist on repo | Orc warns and creates PR without labels (graceful degradation) | TASK-015 |
| `go:embed` fails without static dir | Run `make test` (creates placeholder) or `mkdir -p internal/api/static` | TASK-016 |
| Tests fail with `go.work` | Use `GOWORK=off go test` or `make test` | TASK-016 |
| Raw `InputTokens` appears misleadingly low | Use `EffectiveInputTokens()` which adds cached tokens to get actual context size | TASK-010 |

### Decisions
| Decision | Rationale | Source |
|----------|-----------|--------|
| Sync at completion (default) | Balance safety vs overhead; phase-level sync adds latency for marginal benefit | TASK-019 |

<!-- orc:knowledge:end -->
