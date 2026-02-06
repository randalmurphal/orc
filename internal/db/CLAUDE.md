# Database Package

Database persistence layer with driver abstraction supporting SQLite and PostgreSQL.

## Overview

Two database types with distinct responsibilities:

| Type | Path | Purpose |
|------|------|---------|
| **GlobalDB** | `~/.orc/orc.db` | Cross-project: projects registry, cost_log, budgets, workflows, phase templates, agents, hook_scripts, skills |
| **ProjectDB** | `~/.orc/projects/<id>/orc.db` | Per-project: tasks, phases, transcripts, initiatives, events, FTS |

### Data Split

| GlobalDB (`global.go`, `user.go`) | ProjectDB (`project.go`) |
|-----------------------------------|--------------------------|
| Project registry (id, name, path) | Tasks, phases, gate decisions |
| Users (global registry) | Initiatives, decisions |
| Cost tracking and budgets | Transcripts, FTS search |
| Workflow definitions (shared) | Event log, timeline |
| Phase templates (shared) | Workflow runs (execution records) |
| Agent definitions (shared) | Attachments, comments, branches |
| Hook scripts (shared) | |
| Skills (shared) | |

**Key insight**: GlobalDB holds data that spans projects (definitions, registry). ProjectDB holds project-specific execution data. Workflow/agent definitions live in GlobalDB so all projects share them; workflow runs live in ProjectDB since they belong to a specific project. The executor reads definitions (workflows, phases, phase templates) from GlobalDB and writes execution records (runs, phase results) to ProjectDB.

### Schema Migrations (SQLite)

SQLite migrations in `schema/`. PostgreSQL equivalents in `schema/postgres/` (embedded via `db.go`, see Driver Package section).

| Schema | Purpose |
|--------|---------|
| `schema/global_001.sql`-`003.sql` | Cost log, budgets, projects table |
| `schema/global_004.sql` | **Workflow tables for cross-project sharing** (phase_templates, workflows, workflow_phases, workflow_variables, agents, phase_agents) |
| `schema/project_*.sql` | Per-project tables (tasks through project_047.sql) |
| `schema/project_047.sql` | **Branch control columns on `tasks`**: `branch_name`, `pr_draft`, `pr_labels` (JSON), `pr_reviewers` (JSON), `pr_labels_set`, `pr_reviewers_set` |
| `schema/global_005.sql` | **Extended gate config**: gate_input_config, gate_output_config, gate_mode, gate_agent_id on phase_templates; before_triggers on workflow_phases; triggers on workflows |
| `schema/global_006.sql` | **Hook scripts and skills tables**: `hook_scripts` (id, name, description, content, event_type, is_builtin), `skills` (id, name, description, content, supporting_files, is_builtin) |
| `schema/project_048.sql` | **Mirrors global_005** for project DB |
| `schema/project_052.sql` | **VIEW-based agent filtering** (orc:disable_fk migration) |
| `schema/project_053.sql` | **Feedback table**: real-time user feedback to agents (type, timing, file/line for inline comments) |
| `schema/project_055.sql` | **Sequences table**: atomic ID generation for workflow runs, tasks, initiatives, auto-tasks |
| `schema/global_010.sql` | **Users table**: `users` (id, name, email, created_at); `user_id` column on `cost_log` |
| `schema/project_057.sql` | **User attribution columns**: `assigned_to` on tasks, `created_by`/`owned_by` on initiatives, `executed_by` on phases, `started_by` on workflow_runs |
| `schema/project_058.sql` | **Atomic user claims**: `claimed_by`/`claimed_at` columns on `tasks`, `task_claim_history` table (append-only with `stolen_from` tracking) |

### FK-Disabling Migrations

SQLite's `PRAGMA foreign_keys` cannot be changed inside a transaction. For migrations that restructure tables with FK references (e.g., renaming a table that's referenced by FKs), add the marker comment at the **start** of the migration file:

```sql
-- orc:disable_fk
-- Migration description...
ALTER TABLE parent RENAME TO parent_storage;
```

The migration runner will:
1. Disable FK constraints (`PRAGMA foreign_keys = OFF`)
2. Run the migration in a transaction
3. Re-enable FK constraints
4. Verify no FK violations were introduced (`PRAGMA foreign_key_check`)

**When to use**: Renaming tables that are FK targets, recreating tables to remove FK constraints, or any DDL that would fail with FK enforcement enabled.

## Driver Package (`driver/`)

Database driver abstraction supporting SQLite and PostgreSQL. All database operations go through the `Driver` interface.

| File | Purpose |
|------|---------|
| `driver.go` | `Driver` interface, `Tx` interface, `SchemaFS`, `ParseDialect()`, `New()` factory |
| `sqlite.go` | SQLite driver: `?` placeholders, `datetime('now')`, `strftime()` date helpers, FK-disable migration support |
| `postgresql.go` | PostgreSQL driver: `$N` placeholders, `NOW()`, `TO_CHAR()`/`date_trunc()` helpers, `_migrations` version tracking |
| `migrations.go` | Shared migration runner for both dialects |

**Dialect helpers** (used by CRUD code for portable queries):

| Method | SQLite | PostgreSQL |
|--------|--------|------------|
| `Placeholder(n)` | `?` | `$1`, `$2`, ... |
| `Now()` | `datetime('now')` | `NOW()` |
| `DateFormat(col, fmt)` | `strftime(fmt, col)` | `TO_CHAR(col, fmt)` |
| `DateTrunc(unit, col)` | strftime workaround | `date_trunc(unit, col)` |

Supported `DateFormat` formats: `day`, `week`, `month`, `rfc3339`. Supported `DateTrunc` units: `day`, `week`, `month`, `year`. Adding new formats requires updating both driver implementations + tests.

### PostgreSQL Migration Files

PostgreSQL migrations live in `schema/postgres/` (embedded via `//go:embed`). Each mirrors the corresponding SQLite migration with dialect-appropriate syntax:

| PostgreSQL | Replaces | Key Differences |
|------------|----------|-----------------|
| `SERIAL PRIMARY KEY` | `INTEGER PRIMARY KEY AUTOINCREMENT` | Auto-increment |
| `TIMESTAMP WITH TIME ZONE DEFAULT NOW()` | `TEXT DEFAULT (datetime('now'))` | Timestamps |
| `$1, $2` | `?` | Placeholders |
| `tsvector` + GIN indexes | FTS5 | Full-text search |
| `JSONB` | JSON text columns | Structured data |

**Current PostgreSQL migration coverage**: `global_001.sql`-`global_010.sql`, `project_001.sql`-`project_056.sql`. Remaining: project migrations `project_057.sql`-`project_058.sql` still need porting.

**Table ordering**: PostgreSQL validates FK references at DDL time, so migration files may reorder tables compared to SQLite versions. The SQLite migration runner disables FK enforcement during migrations, making order irrelevant there.

**Integration tests** (`postgres_integration_test.go`) require `ORC_TEST_POSTGRES_DSN` env var.

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
| `workflow.go` | Workflow, PhaseTemplate, WorkflowRun CRUD, QualityCheck type, LoopConfig (parse, max_loops resolution) |
| `gate_config.go` | Gate config JSON parse/marshal: GateInputConfig, GateOutputConfig, BeforePhaseTrigger, WorkflowTrigger |
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
| `feedback.go` | User feedback to agents CRUD |
| `team.go` | Team members, activity |
| `task_claim.go` | User claim operations: `ClaimTaskByUser`, `ForceClaimTaskByUser`, `ReleaseUserClaim`, `GetUserClaimHistory` (append-only with steal tracking) |
| `hook_scripts.go` | Hook script CRUD (GlobalDB): Save/Get/List/Delete, upsert pattern, built-in protection |
| `skills.go` | Skill CRUD (GlobalDB): Save/Get/List/Delete, upsert pattern, built-in protection, JSON supporting_files |
| `constitution.go` | Constitution CRUD, validation checks |
| `dashboard.go` | Dashboard SQL aggregates (status counts, cost by date, initiative stats) |
| `sequence.go` | Atomic ID generation: `NextSequence()`, `GetSequence()`, `SetSequence()` |
| `user.go` | User CRUD (GlobalDB): GetOrCreateUser, GetUser, GetUserByName, ListUsers |

## Key Types

| Type | File | Purpose |
|------|------|---------|
| `DB` | `db.go` | Core wrapper with driver abstraction |
| `GlobalDB` | `global.go` | Global operations (extends DB) |
| `ProjectDB` | `project.go` | Project operations with FTS (extends DB) |
| `TxRunner` | `project.go` | Transaction execution interface |
| `TxOps` | `project.go` | Transaction context (stores context for cancellation) |
| `driver.Driver` | `driver/` | SQLite/PostgreSQL backend interface |
| `LoopConfig` | `workflow.go:114` | Phase loop config: `LoopToPhase`, `Condition` (JSON), `EffectiveMaxLoops()` |
| `SeqWorkflowRun`, `SeqTask`, `SeqInitiative`, `SeqAutoTask` | `sequence.go:12-17` | Sequence name constants for atomic ID generation |
| `User` | `user.go:13` | User identity: ID (UUID), Name (unique), Email, CreatedAt |
| `UserClaimHistoryEntry` | `task_claim.go:12` | Claim audit trail: TaskID, UserID, ClaimedAt, ReleasedAt, StolenFrom |

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

## Atomic Sequence Generation

Prevents race conditions in ID generation across parallel processes. `sequence.go`

| Function | Purpose |
|----------|---------|
| `NextSequence(ctx, name)` | Atomically increment and return next value (cross-process safe) |
| `GetSequence(name)` | Read current value without incrementing |
| `SetSequence(name, value)` | Set sequence to specific value (catch-up after import) |

**ID generation functions** (use sequences internally):

| Function | Location | Generates |
|----------|----------|-----------|
| `GetNextWorkflowRunID()` | `workflow.go:395` | `RUN-XXX` |
| `GetNextTaskID()` | `task.go:24` | `TASK-XXX` |
| `GetNextInitiativeID()` | `initiative.go:43` | `INIT-XXX` |

**Why sequences**: The old `MAX(id)+1` pattern had TOCTOU race conditions when parallel Claude processes called ID generation simultaneously. Database-level UPDATE locks provide true atomicity.

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
| FTS | `SearchTranscripts()` (SQLite: FTS5 MATCH, PostgreSQL: tsvector/tsquery) |

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
| Cost report | `GetCostReport(CostReportFilter)` — aggregated costs with group-by (user/project/model) |
| Budget | `SetBudget()`, `GetBudget()`, `GetBudgetStatus()` |
| Model detect | `DetectModel(modelID)` returns "opus", "sonnet", "haiku", "unknown" |

## User Management (GlobalDB)

Users are stored globally and referenced by ID in project tables. `user.go`

| Function | Purpose |
|----------|---------|
| `GetOrCreateUser(name)` | Idempotent user lookup/create (returns ID) |
| `GetOrCreateUserWithEmail(name, email)` | Same, with email on create |
| `GetUser(id)` | Get by ID (returns nil, nil if not found) |
| `GetUserByName(name)` | Get by unique name |
| `ListUsers()` | All users, newest first |

**User attribution columns** (ProjectDB, reference `users.id`):

| Table | Columns | Purpose |
|-------|---------|---------|
| `tasks` | `created_by`, `assigned_to` | Task creation and assignment |
| `initiatives` | `created_by`, `owned_by` | Initiative ownership |
| `phases` | `executed_by` | Who ran the phase |
| `workflow_runs` | `started_by` | Who started the run |
| `cost_log` (GlobalDB) | `user_id` | Cost attribution |

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
