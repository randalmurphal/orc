# Internal Packages

Core Go packages for the orc orchestrator. Each package has a single responsibility.

## Package Overview

| Package | Responsibility | Key Types |
|---------|----------------|-----------|
| `api/` | HTTP server, REST endpoints, WebSocket | `Server`, handlers |
| `automation/` | Trigger-based task automation | `Trigger`, `Service`, `Evaluator` |
| `bootstrap/` | Instant project initialization (<500ms) | `Run`, `Options`, `Result` |
| `cli/` | Command-line interface (Cobra) | Commands |
| `claude/` | Re-exports llmkit/claudeconfig types | `Settings`, `Skill` |
| `config/` | Configuration loading, hierarchy, env vars | `Config`, `TrackedConfig`, `ConfigSource` |
| `db/` | SQLite persistence (global + project) | `GlobalDB`, `ProjectDB`, `Transcript` |
| `detect/` | Project type, framework, frontend detection | `Detection`, `Detect()` |
| `diff/` | Git diff computation and caching for web UI | `Service`, `DiffResult`, `FileDiff`, `Cache` |
| `enhance/` | Task enhancement via AI | `Enhancer` |
| `errors/` | Custom error types | `OrcError` |
| `events/` | Event publishing for real-time updates | `Publisher`, `Event` |
| `executor/` | Phase execution engine | `WorkflowExecutor`, `Result` |
| `gate/` | Quality gates, approval workflow | `Gate`, `Evaluator`, `PendingDecisionStore` |
| `git/` | Git operations, worktrees (thread-safe) | `Git`, `Checkpoint` |
| `github/` | GitHub API client, PR operations | `Client`, `PR`, `PRStatusSummary` |
| `initiative/` | Initiative/feature grouping | `Initiative`, `Store`, `Manifest` |
| `llmutil/` | **Shared LLM utilities - schema execution** | `ExecuteWithSchema[T]()` |
| `orchestrator/` | Multi-task parallel coordination | `Orchestrator`, `Scheduler`, `WorkerPool` |
| `plan_session/` | Interactive planning sessions | `Mode`, `Options`, `Spawner` |
| `planner/` | Spec-to-task planning | `Planner`, `SpecLoader`, `ProposedTask` |
| `progress/` | Progress tracking and display | `Tracker` |
| `project/` | Multi-project registry | `Registry`, `Project` |
| `prompt/` | Prompt template management | `Service` |
| `setup/` | Claude-powered interactive setup | `Run`, `Spawner`, `Validator` |
| `spec/` | Interactive spec sessions | `Options`, `Spawner`, `Result` |
| `state/` | Execution state persistence | `State`, `CommitTaskState`, `CommitPhaseTransition` |
| `storage/` | Storage backend abstraction (SQLite) | `Backend`, `DatabaseBackend`, `ExportService` |
| `task/` | Task model, attachments, orphan detection | `Task`, `CheckOrphaned()`, `Attachment` |
| `template/` | Go template rendering | `Engine` |
| `tokenpool/` | OAuth token pool for rate limit failover | `Pool`, `Account` |
| `util/` | Common utilities (atomic file writes) | `AtomicWriteFile()` |
| `variable/` | Workflow variable resolution | `Resolver`, `Definition` |
| `wizard/` | Interactive CLI wizard (deprecated) | `Wizard` |
| `workflow/` | Workflow definitions, phase templates | `Workflow`, `PhaseTemplate`, `WorkflowRun` |

## Dependency Graph

```
cmd/orc
    └── cli/
        ├── api/
        │   ├── events/
        │   ├── executor/
        │   │   ├── events/
        │   │   ├── git/
        │   │   ├── github/
        │   │   ├── prompt/
        │   │   ├── state/
        │   │   ├── storage/
        │   │   ├── task/
        │   │   ├── variable/
        │   │   ├── workflow/
        │   │   └── tokenpool/
        │   ├── github/
        │   ├── project/
        │   ├── prompt/
        │   ├── storage/
        │   └── task/
        ├── orchestrator/
        │   ├── executor/
        │   ├── initiative/
        │   └── git/
        ├── executor/
        ├── git/
        ├── state/
        ├── storage/
        ├── workflow/
        └── task/
```

## Key Patterns

### Error Handling

**Philosophy:** Fail loud. Silent failures are bugs.

```go
return fmt.Errorf("load task %s: %w", id, err)
```

**Task/State Consistency:** When execution fails, BOTH task and state must update. See `executor/CLAUDE.md` for the complete error handling checklist.

### Functional Options

```go
executor := NewExecutor(
    WithGitSvc(gitSvc),
    WithPublisher(publisher),
)
```

### Interface-Based Design

```go
type Publisher interface {
    Publish(event Event)
}
```

## Testing

```bash
make test           # Handles prerequisites, runs with race detector
make test-short     # Without race detector (faster)
```

### Test Isolation

**NEVER use `os.Chdir()` in tests** - it's process-wide and not goroutine-safe.

Use explicit path parameters with `t.TempDir()`:

```go
func TestSomething(t *testing.T) {
    tmpDir := t.TempDir()
    err := config.InitAt(tmpDir, false)
    task, err := task.LoadFrom(tmpDir, "TASK-001")
}
```

**Path-aware function variants:**

| Package | Functions |
|---------|-----------|
| `task` | `LoadFrom()`, `LoadAllFrom()`, `TaskDirIn()`, `ExistsIn()`, `DeleteIn()`, `NextIDIn()` |
| `state` | `LoadFrom()`, `LoadAllStatesFrom()` |
| `config` | `InitAt()`, `IsInitializedAt()`, `RequireInitAt()` |

## Package Documentation

See package-specific CLAUDE.md files for detailed usage:

| Package | CLAUDE.md |
|---------|-----------|
| `api/` | API server and handlers |
| `automation/` | Trigger-based automation |
| `bootstrap/` | Instant project initialization |
| `cli/` | CLI commands |
| `db/` | SQLite persistence layer |
| `executor/` | Execution engine (error handling, phase execution) |
| `initiative/` | Initiative grouping |
| `orchestrator/` | Multi-task coordination, process group cleanup |
| `plan_session/` | Interactive planning sessions |
| `planner/` | Spec-to-task planning |
| `progress/` | Progress tracking |
| `setup/` | Claude-powered setup |
| `spec/` | Interactive spec sessions |
| `storage/` | Storage backend abstraction |
| `variable/` | Variable resolution system |
| `workflow/` | Workflow definitions |
