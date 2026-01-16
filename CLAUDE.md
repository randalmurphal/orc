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

## Project Structure

| Path | Purpose | Details |
|------|---------|---------|
| `cmd/orc/` | CLI entry point | - |
| `internal/` | Core packages | See `internal/CLAUDE.md` |
| `templates/` | Phase prompts | See `templates/CLAUDE.md` |
| `web/` | React 19 frontend | See `web/CLAUDE.md` |
| `docs/` | Architecture, specs, ADRs | See `docs/CLAUDE.md` |

**Key packages:** `api/` (REST + WebSocket), `cli/` (Cobra), `executor/` (phase engine), `task/` (task model), `storage/` (database backend), `git/` (worktrees), `db/` (SQLite)

## Task Model

### Organization

| Property | Values | Purpose |
|----------|--------|---------|
| Queue | `active`, `backlog` | Current work vs "someday" |
| Priority | `critical`, `high`, `normal`, `low` | Urgency |
| Category | `feature`, `bug`, `refactor`, `chore`, `docs`, `test` | Type of work |
| Initiative | Initiative ID | Groups related tasks |

### Weight Classification

| Weight | Phases | Use Case |
|--------|--------|----------|
| trivial | implement | One-liner fix |
| small | implement, test | Bug fix, small feature |
| medium | implement, test, docs | Feature with tests |
| large | spec, implement, test, docs, validate, finalize | Complex feature |
| greenfield | research, spec, implement, test, docs, validate, finalize | New system |

### Dependencies

Tasks support `blocked_by` (must complete first) and `related_to` (informational). CLI: `orc new "Part 2" --blocked-by TASK-001`. Initiatives also support `blocked_by` for ordering.

### Completion Detection

Phases complete with `<phase_complete>true</phase_complete>`. Failed phases trigger retry from earlier phase with `{{RETRY_CONTEXT}}`.

## Configuration

**Hierarchy:** Defaults -> `/etc/orc/` -> `~/.orc/` -> `.orc/` -> `ORC_*` env

### Automation Profiles

| Profile | Behavior | PR Approval |
|---------|----------|-------------|
| `auto` | Fully automated (default) | AI auto-approves |
| `fast` | Speed over safety | AI auto-approves |
| `safe` | AI reviews, human merge | Human required |
| `strict` | Human gates throughout | Human required |

See `docs/specs/CONFIG_HIERARCHY.md` for all options.

## File Layout

```
~/.orc/                          # Global config, database, token pool
.orc/                            # Project database, config, prompts, worktrees
.claude/                         # Claude Code settings, hooks, skills
```

Task data stored in SQLite (`orc.db`). Use `orc show TASK-001 --format yaml` for export.

## Commands

| Command | Purpose |
|---------|---------|
| `orc go` | Main entry (interactive/headless/quick) |
| `orc init` | Initialize project |
| `orc new "title"` | Create task (`-c bug`, `-a file`) |
| `orc run TASK-ID` | Execute phases |
| `orc status` | Show running/blocked/ready/paused |
| `orc deps [TASK-ID]` | Show dependencies (`--tree`, `--graph`) |

**Full CLI:** `internal/cli/CLAUDE.md` | **Initiatives:** `orc initiative --help`

## Key Patterns

**Error handling:** Always wrap with context
```go
return fmt.Errorf("load task %s: %w", id, err)
```

**Git commits:** After every phase: `[orc] TASK-001: implement - completed`

## Dependencies

Go modules: `llmkit` (Claude wrapper), `flowgraph` (execution), `devflow` (git ops). For local dev: `make setup` creates `go.work`.

## Web UI

Start: `make build && orc serve` (production) or `make dev-full` (hot reload).

Features: Live task board, WebSocket updates, initiative filtering, keyboard shortcuts (`Shift+Alt` modifier), settings editor.

See `web/CLAUDE.md` for component library and architecture.

## Testing

```bash
make test       # Backend (Go)
make web-test   # Frontend (Vitest)
make e2e        # E2E (Playwright)
```

**E2E tests run in isolated sandbox** (`/tmp`), not production. Import from `./fixtures` for automatic sandbox selection.

## Documentation Reference

| Topic | Location |
|-------|----------|
| API Endpoints | `docs/API_REFERENCE.md` |
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phase Model | `docs/architecture/PHASE_MODEL.md` |
| Config | `docs/specs/CONFIG_HIERARCHY.md` |
| Web Components | `web/CLAUDE.md` |

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

Task specifications and state stored in `.orc/tasks/`:
```
.orc/tasks/TASK-001/
├── task.yaml, spec.md, plan.yaml, state.yaml, attachments/
```

### CLI Commands

```bash
orc status           # View active tasks
orc run TASK-001     # Execute task
orc pause TASK-001   # Pause execution
orc resume TASK-001  # Continue task
```
<!-- orc:end -->

## Project Knowledge

See [docs/knowledge/PROJECT_KNOWLEDGE.md](docs/knowledge/PROJECT_KNOWLEDGE.md) for patterns, gotchas, and decisions learned during development.

<!-- orc:knowledge:target:docs/knowledge/PROJECT_KNOWLEDGE.md -->
