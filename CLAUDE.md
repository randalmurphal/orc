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
| `internal/api/` | REST API + WebSocket server |
| `internal/executor/` | flowgraph-based phase execution |
| `internal/events/` | Event publisher for real-time updates |
| `internal/task/` | Task model + YAML persistence |
| `internal/plan/` | Phase templates + weight classification |
| `internal/state/` | Execution state tracking |
| `internal/prompt/` | Prompt management service |
| `internal/hooks/` | Claude Code hooks management |
| `internal/skills/` | Claude Code skills management |
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
├── prompts/           # Project prompt overrides
│   └── implement.md
└── tasks/TASK-001/
    ├── task.yaml       # Definition
    ├── plan.yaml       # Phase sequence
    ├── state.yaml      # Execution state
    └── transcripts/    # Claude conversation logs

.claude/
├── hooks/             # Claude Code hooks
│   └── my-hook.json
└── skills/            # Claude Code skills
    └── my-skill.yaml
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

# E2E tests
make e2e            # Run Playwright tests
```

## API Endpoints

### Tasks
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
| GET | `/api/hooks` | List hooks |
| GET | `/api/hooks/types` | Get hook types |
| POST | `/api/hooks` | Create hook |
| GET | `/api/hooks/:name` | Get hook |
| PUT | `/api/hooks/:name` | Update hook |
| DELETE | `/api/hooks/:name` | Delete hook |

### Skills
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/skills` | List skills |
| POST | `/api/skills` | Create skill |
| GET | `/api/skills/:name` | Get skill |
| PUT | `/api/skills/:name` | Update skill |
| DELETE | `/api/skills/:name` | Delete skill |

### Config & Real-time
| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/config` | Get configuration |
| PUT | `/api/config` | Update configuration |
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
cd web && npm test

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
