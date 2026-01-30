# Internal Packages

Core Go packages for the orc orchestrator. Each package has a single responsibility.

## Package Overview

| Package | Responsibility | Key Types |
|---------|----------------|-----------|
| `api/` | Connect RPC server, WebSocket | `Server`, `*Server` (service impls) |
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
| `gate/` | Quality gates, approval workflow (auto/human/AI/skip) | `Gate`, `Evaluator`, `Resolver`, `GateAgentResponse`, `PendingDecisionStore` |
| `git/` | Git operations, worktrees (thread-safe) | `Git`, `Checkpoint` |
| `hosting/` | Multi-provider git hosting (GitHub, GitLab) | `Provider`, `PR`, `PRStatusSummary` |
| `initiative/` | Initiative/feature grouping | `Initiative`, `Store`, `Manifest` |
| `jira/` | Jira Cloud import (API client, issue mapping, ADF conversion) | `Client`, `Importer`, `Mapper`, `Issue`, `ImportResult` |
| `llmutil/` | **Shared LLM utilities - schema execution** | `ExecuteWithSchema[T]()` |
| `orchestrator/` | Multi-task parallel coordination | `Orchestrator`, `Scheduler`, `WorkerPool` |
| `plan_session/` | Interactive planning sessions | `Mode`, `Options`, `Spawner` |
| `planner/` | Spec-to-task planning | `Planner`, `SpecLoader`, `ProposedTask` |
| `progress/` | Progress tracking and display | `Tracker` |
| `project/` | Multi-project registry | `Registry`, `Project` |
| `prompt/` | Prompt template management | `Service` |
| `setup/` | Claude-powered interactive setup | `Run`, `Spawner`, `Validator` |
| `spec/` | Interactive spec sessions | `Options`, `Spawner`, `Result` |
| `storage/` | Storage backend abstraction (SQLite) | `Backend`, `DatabaseBackend`, `ExportService` |
| `task/` | Proto helpers, execution state utils, orphan detection | `proto_helpers.go`, `execution_helpers.go`, `CheckOrphaned()` |
| `template/` | Go template rendering | `Engine` |
| `tokenpool/` | OAuth token pool for rate limit failover | `Pool`, `Account` |
| `trigger/` | Lifecycle event trigger evaluation | `Runner`, `TriggerRunner`, `GateRejectionError` |
| `util/` | Common utilities (atomic file writes) | `AtomicWriteFile()` |
| `variable/` | Workflow variable resolution | `Resolver`, `Definition` |
| `workflow/` | Workflow definitions, phase templates | `Workflow`, `PhaseTemplate`, `WorkflowRun` |

## Dependency Graph

```
cmd/orc
    └── cli/
        ├── api/
        │   ├── events/
        │   ├── executor/
        │   │   ├── events/
        │   │   ├── gate/
        │   │   ├── git/
        │   │   ├── hosting/
        │   │   ├── prompt/
        │   │   ├── storage/
        │   │   ├── task/
        │   │   ├── trigger/
        │   │   ├── variable/
        │   │   ├── workflow/
        │   │   └── tokenpool/
        │   ├── hosting/
        │   ├── project/
        │   ├── prompt/
        │   ├── storage/
        │   └── task/
        ├── jira/
        │   └── storage/
        ├── orchestrator/
        │   ├── executor/
        │   ├── initiative/
        │   └── git/
        ├── executor/
        ├── git/
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

**Task Consistency:** Task status and execution state are unified in `orcv1.Task` (the proto domain model from `gen/proto/orc/v1/task.pb.go`). When execution fails, update both `t.Status` and `t.Execution` fields, then save with `backend.SaveTask(t)`. See `executor/CLAUDE.md` for the complete error handling checklist.

### Functional Options

```go
executor := NewExecutor(
    WithGitSvc(gitSvc),
    WithPublisher(publisher),
)
```

### Two-Tier Database Model

Orc uses two database tiers for multi-project support:

| Tier | Type | Scope | Contents |
|------|------|-------|----------|
| `GlobalDB` | `db.GlobalDB` | Shared across all projects | Built-in workflows, agents, project registry |
| `ProjectDB` | `db.ProjectDB` | Per-project | Tasks, initiatives, transcripts, events |

API services resolve the correct `ProjectDB` via `getBackend(projectID)`, which routes through `ProjectCache` (`api/project_cache.go`) -- an LRU cache of open database connections. Server startup seeds the `GlobalDB` with built-in workflows and agents.

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
| `config` | `InitAt()`, `IsInitializedAt()`, `RequireInitAt()` |

## Package Documentation

See package-specific CLAUDE.md files for detailed usage:

| Package | CLAUDE.md |
|---------|-----------|
| `api/` | Connect RPC services, WebSocket |
| `automation/` | Trigger-based automation |
| `bootstrap/` | Instant project initialization |
| `cli/` | CLI commands |
| `db/` | SQLite persistence layer |
| `executor/` | Execution engine (error handling, phase execution) |
| `gate/` | Quality gates (auto/human/AI/skip), resolution, pending decisions |
| `initiative/` | Initiative grouping |
| `orchestrator/` | Multi-task coordination, process group cleanup |
| `plan_session/` | Interactive planning sessions |
| `planner/` | Spec-to-task planning |
| `progress/` | Progress tracking |
| `setup/` | Claude-powered setup |
| `spec/` | Interactive spec sessions |
| `storage/` | Storage backend abstraction |
| `trigger/` | Lifecycle event trigger evaluation |
| `variable/` | Variable resolution system |
| `workflow/` | Workflow definitions |
