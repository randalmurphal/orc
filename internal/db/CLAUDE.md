# Database Package

Database persistence layer with driver abstraction supporting SQLite and PostgreSQL.

## Overview

Two database types with distinct responsibilities:

| Type | Path | Purpose |
|------|------|---------|
| **GlobalDB** | `~/.orc/orc.db` | Cross-project: projects registry, cost_log, budgets, workflows, phase templates, agents |
| **ProjectDB** | `.orc/orc.db` | Per-project: tasks, phases, transcripts, initiatives, events, FTS |

### Data Split

| GlobalDB (`global.go`) | ProjectDB (`project.go`) |
|-------------------------|--------------------------|
| Project registry (id, name, path) | Tasks, phases, gate decisions |
| Cost tracking and budgets | Initiatives, decisions |
| Workflow definitions (shared) | Transcripts, FTS search |
| Phase templates (shared) | Event log, timeline |
| Agent definitions (shared) | Workflow runs (execution records) |
| Future: shared settings | Attachments, comments, branches |

**Key insight**: GlobalDB holds data that spans projects (definitions, registry). ProjectDB holds project-specific execution data. Workflow/agent definitions live in GlobalDB so all projects share them; workflow runs live in ProjectDB since they belong to a specific project.

### Schema Migrations

| Schema | Purpose |
|--------|---------|
| `schema/global_001.sql`-`003.sql` | Cost log, budgets, projects table |
| `schema/global_004.sql` | **Workflow tables for cross-project sharing** (phase_templates, workflows, workflow_phases, workflow_variables, agents, phase_agents) |
| `schema/project_*.sql` | Per-project tables (tasks through project_047.sql) |
| `schema/project_047.sql` | **Branch control columns on `tasks`**: `branch_name`, `pr_draft`, `pr_labels` (JSON), `pr_reviewers` (JSON), `pr_labels_set`, `pr_reviewers_set` |

## File Structure

| File | Contents |
|------|----------|
| `project.go` | Core: TxRunner, TxOps, ProjectDB, OpenProject, RunInTx, Detection |
| `global.go` | GlobalDB: project registry, cost tracking, budgets, workflows, agents |
| `task.go` | Task CRUD, ListOpts, TaskFull, dependencies, scanners, Tx functions |
| `initiative.go` | Initiative, decisions, task refs, dependencies, batch loading |
| `phase.go` | Phase CRUD, Tx variants |
| `agents.go` | Agent CRUD, builtin vs custom agent management |
| `transcript.go` | Transcript CRUD, batch insert, FTS, token aggregation, todos, metrics, agent stats |
| `plan.go` | Plan CRUD |
| `workflow.go` | Workflow, PhaseTemplate, WorkflowRun CRUD, QualityCheck type |
| `phase_output.go` | Phase output CRUD (unified specs + artifacts) |
| `project_command.go` | ProjectCommand CRUD (quality check commands) |
| `event_log.go` | EventLog CRUD, batch insert, time/type filtering for timeline |
| `gate_decision.go` | GateDecision CRUD, Tx variants |
| `attachment.go` | Attachment CRUD |
| `sync_state.go` | SyncState for P2P sync |
| `branch.go` | Branch registry CRUD |
| `subtask.go` | Subtask queue operations |
| `review_comment.go` | Review comment CRUD |
| `task_comment.go` | Task comment CRUD |
| `team.go` | Team members, claims, activity |
| `constitution.go` | Constitution CRUD, validation checks |
| `dashboard.go` | Dashboard SQL aggregates (status counts, cost by date, initiative stats) |

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `DB` | `db.go` | Core wrapper with driver abstraction |
| `GlobalDB` | `global.go` | Global operations (extends DB) |
| `ProjectDB` | `project.go` | Project operations with FTS (extends DB) |
| `TxRunner` | `project.go` | Transaction execution interface |
| `TxOps` | `project.go` | Transaction context (stores context for cancellation) |
| `driver.Driver` | `driver/` | SQLite/PostgreSQL backend interface |

## Usage

```go
// SQLite (default)
gdb, err := db.OpenGlobal()
pdb, err := db.OpenProject("/path/to/project")

// PostgreSQL
gdb, err := db.OpenGlobalWithDialect(dsn, driver.DialectPostgres)
pdb, err := db.OpenProjectWithDialect(dsn, driver.DialectPostgres)
```

## Transaction Support

Multi-table operations use transactions for atomicity:

```go
err := pdb.RunInTx(ctx, func(tx *db.TxOps) error {
    if err := db.SaveTaskTx(tx, task); err != nil {
        return err  // Triggers rollback
    }
    if err := db.AddTaskDependencyTx(tx, taskID, depID); err != nil {
        return err
    }
    return nil  // Commits
})
```

`TxOps` propagates the context for cancellation/timeout support.

## Batch Loading

Avoid N+1 queries with batch methods:

```go
// Instead of per-task queries:
allDeps, _ := db.GetAllTaskDependencies()  // 1 query
deps := allDeps[task.ID]                    // Map lookup
```

**Batch functions:**

| Function | Returns | Used By |
|----------|---------|---------|
| `GetAllTaskDependencies()` | `map[string][]string` | Task loading |
| `GetAllPhasesGrouped()` | `map[string][]Phase` | `LoadAllStates()` |
| `GetAllGateDecisionsGrouped()` | `map[string][]GateDecision` | `LoadAllStates()` |
| `GetAllInitiativeDecisions()` | `map[string][]InitiativeDecision` | Initiative loading |
| `GetAllInitiativeTaskRefs()` | `map[string][]string` | Initiative loading |
| `GetAllInitiativeDependencies()` | `map[string][]string` | Initiative loading |
| `GetAllInitiativeDependents()` | `map[string][]string` | Initiative loading |
| `GetInitiativeTitlesBatch(ids)` | `map[string]string` | Dashboard (no N+1) |

## Dashboard Aggregates

SQL-level aggregation for dashboard stats, avoiding full task load. `dashboard.go`

| Function | Returns | Purpose |
|----------|---------|---------|
| `GetDashboardStatusCounts()` | `DashboardStatusCounts` | Task counts by status via `GROUP BY` |
| `GetDashboardCostByDate(since)` | `[]DashboardCostByDate` | Daily cost aggregation |
| `GetDashboardInitiativeStats(limit)` | `[]DashboardInitiativeStat` | Per-initiative task stats |
| `GetInitiativeTitlesBatch(ids)` | `map[string]string` | Batch title lookup (single query) |

**Indexes** (`schema/project_043.sql`): `idx_tasks_completed_at`, `idx_tasks_updated_at` — accelerate time-filtered dashboard queries.

## Transcript System

Transcripts store Claude Code session messages. `transcript.go`

| Category | Key Functions |
|----------|---------------|
| Token aggregation | `GetTaskTokenUsage()`, `GetPhaseTokenUsage()` |
| Todo snapshots | `AddTodoSnapshot()`, `GetLatestTodos()`, `GetTodoHistory()` |
| Metrics | `GetMetricsSummary()`, `GetDailyMetrics()`, `GetTaskMetrics()` |
| FTS | `SearchTranscripts()` (SQLite: FTS5 MATCH, PostgreSQL: ILIKE) |

## Event Log System

Persisted executor events for timeline reconstruction. `event_log.go`

| Operation | Function |
|-----------|----------|
| Save | `SaveEvent()`, `SaveEvents()` (batch, transactional) |
| Query | `QueryEvents(opts)`, `QueryEventsWithTitles(opts)`, `CountEvents(opts)` |

`QueryEventsOptions` filters: `TaskID`, `InitiativeID`, `Since`, `Until`, `EventTypes`, `Limit`, `Offset`. Returns newest first.

## Cost Tracking (GlobalDB)

| Operation | Function |
|-----------|----------|
| Record | `RecordCostExtended(CostEntry)` (model, cache, duration tracking) |
| Query by model | `GetCostByModel(projectID, since)` |
| Time series | `GetCostTimeseries(projectID, since, granularity)` |
| Budget | `SetBudget()`, `GetBudget()`, `GetBudgetStatus()` |
| Model detect | `DetectModel(modelID)` returns "opus", "sonnet", "haiku", "unknown" |

## Workflow System

Workflow definitions and agents live in **GlobalDB** (shared across projects). Workflow runs live in **ProjectDB** (per-project execution records). `workflow.go`

| Type | DB | Purpose |
|------|-----|---------|
| `PhaseTemplate` | Global | Reusable phase definitions with prompt config |
| `Workflow` | Global | Composed execution plans from phase templates |
| `WorkflowPhase` | Global | Phase sequence within a workflow |
| `WorkflowVariable` | Global | Custom variable definitions |
| `WorkflowRun` | Project | Execution instance tracking |
| `WorkflowRunPhase` | Project | Phase execution within a run |

Standard CRUD: `Save/Get/List/Delete` for each type. See `internal/workflow/CLAUDE.md` for built-in workflows and seeding.

## Testing

```bash
go test ./internal/db/... -v
```

Tests use `t.TempDir()` for isolated databases.

## Reference

See [SCHEMA.md](SCHEMA.md) for full schema documentation including:
- Table definitions
- Transaction-aware functions
- Dialect-specific queries
