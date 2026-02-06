# Internal Packages

Core Go packages for the orc orchestrator. Each package has a single responsibility.

## Package Overview

| Package | Responsibility | Key Types |
|---------|----------------|-----------|
| `api/` | Connect RPC server, WebSocket | `Server`, `*Server` (service impls) |
| `automation/` | Trigger-based task automation | `Trigger`, `Service`, `Evaluator` |
| `brief/` | Auto-generated project briefs from task history | `Generator`, `Brief`, `Cache` |
| `bootstrap/` | Instant project initialization (<500ms) | `Run`, `Options`, `Result` |
| `cli/` | Command-line interface (Cobra) | Commands |
| `claude/` | Re-exports llmkit/claudeconfig types | `Settings`, `Skill` |
| `config/` | Configuration loading, hierarchy, env vars | `Config`, `TrackedConfig`, `ConfigSource` |
| `db/` | Database persistence (SQLite + PostgreSQL) | `GlobalDB`, `ProjectDB`, `Transcript` |
| `db/driver/` | Database driver abstraction | `Driver`, `Dialect`, `Tx`, `SchemaFS` |
| `detect/` | Project type, framework, frontend detection | `Detection`, `Detect()` |
| `diff/` | Git diff computation and caching for web UI | `Service`, `DiffResult`, `FileDiff`, `Cache` |
| `enhance/` | Task enhancement via AI | `Enhancer` |
| `errors/` | Custom error types | `OrcError` |
| `events/` | Event publishing for real-time updates | `Publisher`, `Event` |
| `executor/` | Phase execution engine | `WorkflowExecutor`, `Result` |
| `gate/` | Quality gates, approval workflow (auto/human/AI/skip) | `Gate`, `Evaluator`, `Resolver`, `GateAgentResponse`, `PendingDecisionStore` |
| `git/` | Git operations, worktrees (thread-safe) | `Git`, `Checkpoint` |
| `hosting/` | Multi-provider git hosting (GitHub, GitLab), PR lifecycle (create/find/update/merge) | `Provider`, `PR`, `PRStatusSummary`, `ErrNoPRFound` |
| `initiative/` | Initiative/feature grouping, acceptance criteria | `Initiative`, `Criterion`, `CoverageReport`, `Store`, `Manifest` |
| `jira/` | Jira Cloud import (API client, issue mapping, ADF conversion) | `Client`, `Importer`, `Mapper`, `Issue`, `ImportResult` |
| `knowledge/` | Knowledge layer: Docker infra, stores, embeddings, retrieval | `Service`, `Components`, `QueryComponents`, `TaskContext` |
| `knowledge/retrieve/` | Multi-signal search pipeline with presets and scoring | `Pipeline`, `Stage`, `Scorer`, `PresetDeps` |
| `knowledge/infra/` | Docker container lifecycle management | `Manager`, `DockerClient`, `Config` |
| `knowledge/store/` | Graph (Neo4j), vector (Qdrant), cache (Redis) stores | `GraphStore`, `VectorStore`, `CacheStore` |
| `knowledge/embed/` | Text embedding providers (Voyage AI, local sidecar) | `Embedder`, `VoyageEmbedder`, `SidecarEmbedder` |
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
        │   │   ├── brief/
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
        ├── knowledge/
        │   ├── retrieve/
        │   ├── infra/
        │   ├── store/
        │   └── embed/
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

### Construction Helpers

When multiple packages need the same object built from config, create ONE helper and use it everywhere. Never let callers build the object inline — config fields get missed, defaults diverge, and bugs happen silently.

| Object | Helper | Location |
|--------|--------|----------|
| `git.Git` (CLI) | `NewGitOpsFromConfig()` | `cli/git_helpers.go` |

`git.DefaultConfig()` intentionally has an empty `WorktreeDir` — callers MUST set it explicitly via the helper or `config.ResolveWorktreeDir()`.

### Functional Options

```go
executor := NewExecutor(
    WithGitSvc(gitSvc),
    WithPublisher(publisher),
)
```

### Two-Tier Database Model

Orc uses two database tiers for multi-project support, with driver abstraction (`db/driver/`) supporting both SQLite and PostgreSQL:

| Tier | Type | Location | Contents |
|------|------|----------|----------|
| `GlobalDB` | `db.GlobalDB` | `~/.orc/orc.db` (SQLite) or shared PG | Built-in workflows, agents, project registry |
| `ProjectDB` | `db.ProjectDB` | `~/.orc/projects/<id>/orc.db` (SQLite) or shared PG | Tasks, initiatives, transcripts, events |

All runtime state lives in `~/.orc/`, keeping project `.orc/` directories config-only (git-tracked). API services resolve the correct `ProjectDB` via `getBackend(projectID)`, which routes through `ProjectCache` (`api/project_cache.go`) -- an LRU cache of open database connections. Server startup seeds the `GlobalDB` with built-in workflows and agents. Dialect configured via `database.dialect` in config (`internal/config/config_types.go`).

### Initiative Acceptance Criteria

Initiatives support structured acceptance criteria (`initiative/criterion.go`) that track whether an initiative's goals are met. Criteria are stored in `initiative_criteria` table (migration `project_059.sql`) and managed through `Initiative` domain methods.

**Status lifecycle:**

| Status | Meaning | Transition |
|--------|---------|------------|
| `uncovered` | No tasks mapped | Initial state |
| `covered` | At least one task mapped | Automatic on `MapCriterionToTask()` |
| `satisfied` | Verified as met | Manual via `VerifyCriterion()` |
| `regressed` | Previously satisfied, now broken | Manual via `VerifyCriterion()` |

**Key operations:** `AddCriterion()` (auto-generates `AC-NNN` IDs), `MapCriterionToTask()`, `VerifyCriterion()`, `GetCoverageReport()`. Criterion sequence is not persisted; it is reconstructed from existing IDs via `RecomputeCriterionSeq()` on load.

**CLI:** `orc initiative criteria INIT-001 [add|map|verify|coverage]`
**API:** `AddCriterion`, `RemoveCriterion`, `MapCriterionToTask`, `VerifyCriterion`, `GetCoverageReport` RPCs on `InitiativeService`.

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

**NEVER let tests touch the real `~/.orc/` directory.** Any test that calls `bootstrap.Run()`, `project.RegisterProject()`, `db.OpenProject()`, or anything that resolves `GlobalPath()` MUST isolate HOME:

```go
func TestSomething(t *testing.T) {
    tmpDir := t.TempDir()
    homeDir := filepath.Join(tmpDir, "home")
    projectDir := filepath.Join(tmpDir, "project")
    os.MkdirAll(homeDir, 0755)
    os.MkdirAll(projectDir, 0755)
    t.Setenv("HOME", homeDir) // Isolate from real ~/.orc

    // Now homeDir/.orc/ is the global dir, projectDir/.orc/ is the project dir
    // These MUST be different directories to avoid collision
    result, err := bootstrap.Run(bootstrap.Options{WorkDir: projectDir})
}
```

**Why separate dirs?** `~/.orc/` (global state) and `<project>/.orc/` (config) must not overlap. If `HOME == projectDir`, both resolve to the same `.orc/` directory, causing subtle bugs.

**Prefer `t.TempDir()` over `os.MkdirTemp()`** — `t.TempDir()` is automatically cleaned up by the test framework. `os.MkdirTemp()` requires manual `defer os.RemoveAll()`.

**Prefer in-memory databases** when testing storage logic that doesn't need file I/O:

```go
backend := storage.NewTestBackend(t)  // In-memory, auto-cleanup
globalDB := storage.NewTestGlobalDB(t)
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
| `brief/` | Auto-generated project briefs |
| `bootstrap/` | Instant project initialization |
| `cli/` | CLI commands |
| `db/` | Database persistence (SQLite + PostgreSQL) |
| `events/` | Real-time event publishing (WebSocket + DB persistence) |
| `executor/` | Execution engine (error handling, phase execution) |
| `gate/` | Quality gates (auto/human/AI/skip), resolution, pending decisions |
| `initiative/` | Initiative grouping |
| `knowledge/` | Knowledge layer (infra, stores, embeddings) |
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
