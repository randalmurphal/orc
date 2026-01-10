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
| `internal/executor/` | flowgraph-based phase execution |
| `internal/task/` | Task model + YAML persistence |
| `internal/plan/` | Phase templates + weight classification |
| `internal/state/` | Execution state tracking |
| `internal/git/` | Git checkpointing, branches |
| `templates/` | Phase templates (plans/, prompts/) |

## Dependencies

Uses local sibling repos via `go.mod` replace:
- `../llmkit` - Claude CLI wrapper, templates, model selection
- `../flowgraph` - Graph-based execution with checkpointing

## Task Weight → Phases

| Weight | Phases |
|--------|--------|
| trivial | implement |
| small | implement → test |
| medium | spec → implement → review → test |
| large | research → spec → design → implement → review → test → validate |
| greenfield | research → spec → design → implement → review → test → validate |

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
| `orc run TASK-ID` | Execute task phases |
| `orc pause TASK-ID` | Pause execution, save state |
| `orc resume TASK-ID` | Continue from checkpoint |
| `orc rewind TASK-ID --phase X` | Reset to before phase X |
| `orc approve TASK-ID` | Pass human gate |
| `orc status` | Show running tasks |

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
