# Internal Packages

Core Go packages for the orc orchestrator. Each package has a single responsibility.

## Package Overview

| Package | Responsibility | Key Types |
|---------|----------------|-----------|
| `api/` | HTTP server, REST endpoints, WebSocket | `Server`, handlers |
| `bootstrap/` | Instant project initialization (<500ms) | `Run`, `Options`, `Result` |
| `cli/` | Command-line interface (Cobra) | Commands |
| `claude/` | Re-exports llmkit/claudeconfig types | `Settings`, `Skill` |
| `config/` | Configuration loading, hierarchy, env vars | `Config`, `TrackedConfig`, `ConfigSource` |
| `db/` | SQLite persistence (global + project) | `GlobalDB`, `ProjectDB`, `Transcript` |
| `detect/` | Project type, framework, and frontend detection | `Detection`, `Detect()` |
| `diff/` | Git diff computation and caching for web UI | `Service`, `DiffResult`, `FileDiff`, `Cache` |
| `enhance/` | Task enhancement via AI | `Enhancer` |
| `errors/` | Custom error types | `OrcError` |
| `events/` | Event publishing for real-time updates | `Publisher`, `Event` |
| `executor/` | Phase execution engine | `Executor`, `Result` |
| `gate/` | Quality gates and approval workflow | `Gate`, `Evaluator` |
| `git/` | Git operations, worktrees | `Git`, `Checkpoint` |
| `github/` | GitHub API client, PR operations, status detection | `Client`, `PR`, `PRStatusSummary` |
| `plan/` | Plan generation, regeneration on weight change | `Plan`, `Phase`, `RegeneratePlan` |
| `progress/` | Progress tracking and display | `Tracker` |
| `project/` | Multi-project registry | `Registry`, `Project` |
| `prompt/` | Prompt template management | `Service` |
| `state/` | Execution state persistence, auto-commit helpers | `State`, `CommitTaskState`, `CommitPhaseTransition` |
| `storage/` | Storage backend abstraction layer | `Backend`, `HybridBackend`, `ExportService` |
| `task/` | Task model, YAML persistence, attachments, testing requirements | `Task`, `TestingRequirements`, `Store`, `Attachment` |
| `setup/` | Claude-powered interactive setup | `Run`, `Spawner`, `Validator` |
| `template/` | Go template rendering | `Engine` |
| `tokenpool/` | OAuth token pool for rate limit failover | `Pool`, `Account` |
| `watcher/` | File system watcher for real-time updates | `Watcher`, `Debouncer` |
| `wizard/` | Interactive CLI wizard (deprecated) | `Wizard` |

## Dependency Graph

```
cmd/orc
    └── cli/
        ├── api/
        │   ├── events/
        │   ├── executor/
        │   │   ├── events/
        │   │   ├── git/
        │   │   ├── github/     # PR creation
        │   │   ├── plan/
        │   │   ├── prompt/
        │   │   ├── state/
        │   │   ├── storage/
        │   │   ├── task/
        │   │   └── tokenpool/
        │   ├── github/         # PR status polling
        │   ├── project/
        │   ├── prompt/
        │   ├── storage/
        │   ├── task/
        │   └── watcher/        # File system monitoring
        │       ├── events/
        │       └── task/
        ├── executor/
        ├── git/
        ├── plan/
        ├── state/
        ├── storage/
        └── task/
```

## Key Patterns

### Error Handling
All errors wrap context for traceability:
```go
return fmt.Errorf("load task %s: %w", id, err)
```

### Functional Options
Constructors use functional options pattern:
```go
executor := NewExecutor(
    WithGitSvc(gitSvc),
    WithPublisher(publisher),
    WithLogger(logger),
)
```

### Interface-Based Design
Core components use interfaces for testability:
```go
type Publisher interface {
    Publish(event Event)
}
```

## Testing

Each package has comprehensive tests. Use `make test` to run all tests with proper setup:
```bash
make test           # Handles prerequisites, runs with race detector
make test-short     # Without race detector (faster)
```

Or run directly (requires prerequisites):
```bash
go test ./internal/... -v
```

### Test Prerequisites

The API package uses `go:embed` for static frontend files. Tests require a placeholder:
```bash
mkdir -p internal/api/static
echo "# Placeholder for go:embed" > internal/api/static/.gitkeep
```

When using `go.work` for local dependency development, use `GOWORK=off` for test isolation:
```bash
GOWORK=off go test -v ./...
```

The Makefile handles both automatically.

### Test Isolation Pattern

**NEVER use `os.Chdir()` in tests** - it's process-wide and not goroutine-safe.

Instead, use explicit path parameters with `t.TempDir()`:

```go
func TestSomething(t *testing.T) {
    tmpDir := t.TempDir()

    // Initialize in temp directory
    err := config.InitAt(tmpDir, false)

    // Load from temp directory
    task, err := task.LoadFrom(tmpDir, "TASK-001")

    // Save to temp directory
    err = task.SaveTo(filepath.Join(tmpDir, ".orc", "tasks", task.ID))
}
```

**Path-aware function variants:**

| Package | Functions |
|---------|-----------|
| `task` | `LoadFrom(projectDir, id)`, `LoadAllFrom(tasksDir)`, `TaskDirIn(projectDir, id)`, `ExistsIn(projectDir, id)`, `DeleteIn(projectDir, id)`, `NextIDIn(tasksDir)`, `ListAttachments(taskDir)`, `SaveAttachment(taskDir, filename, reader)`, `GetAttachment(taskDir, filename)`, `DeleteAttachment(taskDir, filename)` |
| `state` | `LoadFrom(projectDir, taskID)`, `LoadAllStatesFrom(projectDir)` |
| `plan` | `LoadFrom(projectDir, taskID)`, `RegeneratePlanForTask(projectDir, task)` |
| `config` | `InitAt(basePath, force)`, `IsInitializedAt(basePath)`, `RequireInitAt(basePath)` |
| `template` | `SaveTo(baseDir)`, `ListFrom(projectTemplatesDir)` |

The API server uses `WorkDir` in its config to specify the project directory.

## Package Details

See package-specific CLAUDE.md files:
- `api/CLAUDE.md` - API server and handlers
- `bootstrap/CLAUDE.md` - Instant project initialization
- `cli/CLAUDE.md` - CLI commands
- `db/CLAUDE.md` - SQLite persistence layer
- `executor/CLAUDE.md` - Execution engine modules
- `setup/CLAUDE.md` - Claude-powered setup
- `watcher/CLAUDE.md` - File system watcher for live updates
