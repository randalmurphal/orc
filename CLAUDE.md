# Orc - Claude Code Task Orchestrator

## Quick Start

```bash
# Setup (first time)
make setup    # Configure go.mod with local dependencies

# Development
make build    # Build binary to bin/orc
make test     # Run tests
make dev      # Interactive shell in container

# Run
./bin/orc init
./bin/orc new "task description"
./bin/orc run TASK-001
```

## Project Structure

| Path | Purpose |
|------|---------|
| `cmd/orc/` | CLI entry point |
| `internal/cli/` | Cobra commands |
| `internal/api/` | REST API + SSE server |
| `internal/executor/` | flowgraph-based phase execution |
| `internal/task/` | Task model + YAML persistence |
| `internal/plan/` | Phase templates + weight classification |
| `internal/state/` | Execution state tracking |
| `internal/git/` | Git checkpointing, branches |
| `templates/` | Phase templates (plans/, prompts/) |
| `web/` | Svelte 5 frontend (SvelteKit) |

## Dependencies

Uses local sibling repos via `go.mod` replace:
- `../llmkit` - Claude CLI wrapper, templates, model selection
- `../flowgraph` - Graph-based execution with checkpointing

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

## Task Weight → Phases

| Weight | Phases |
|--------|--------|
| trivial | implement |
| small | implement → test |
| medium | implement → test |
| large | spec → implement → test → validate |
| greenfield | research → spec → implement → test → validate |

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

## Completion Detection

Phases complete when Claude outputs:
```xml
<phase_complete>true</phase_complete>
```

Phases block when Claude outputs:
```xml
<phase_blocked>reason: ...</phase_blocked>
```

## File Layout

```
.orc/
├── config.yaml
└── tasks/TASK-001/
    ├── task.yaml       # Definition
    ├── plan.yaml       # Phase sequence
    ├── state.yaml      # Execution state
    └── transcripts/    # Claude conversation logs
```

## Commands

| Command | Purpose |
|---------|---------|
| `orc init` | Initialize .orc/ in current directory |
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

## Web UI

```bash
# Install frontend dependencies (first time)
make web-install

# Development (start both servers)
make serve          # API on :8080
make web-dev        # Frontend on :5173

# Production build
make web-build      # Outputs to web/build/
```

API endpoints:
- `GET /api/tasks` - List tasks
- `POST /api/tasks` - Create task
- `GET /api/tasks/:id` - Get task
- `GET /api/tasks/:id/stream` - SSE transcript stream
- `POST /api/tasks/:id/run` - Start task
- `POST /api/tasks/:id/pause` - Pause task

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

## Container Usage

```bash
# Development shell
make dev

# Run tests in container
make docker-test

# Build production binary
make release-build
```

## Docs Reference

| Topic | Path |
|-------|------|
| Architecture | `docs/architecture/OVERVIEW.md` |
| Phases | `docs/architecture/PHASE_MODEL.md` |
| Gates | `docs/architecture/GATES.md` |
| CLI Spec | `docs/specs/CLI.md` |
| File Formats | `docs/specs/FILE_FORMATS.md` |
