# Database Package

Database persistence layer with driver abstraction supporting SQLite and PostgreSQL.

## Overview

| Type | Path | Purpose |
|------|------|---------|
| **GlobalDB** | `~/.orc/orc.db` | Cross-project: projects, cost_log, templates |
| **ProjectDB** | `.orc/orc.db` | Per-project: tasks, phases, transcripts, FTS |

## File Structure

| File | Contents |
|------|----------|
| `project.go` | Core: TxRunner, TxOps, ProjectDB, OpenProject, RunInTx, Detection |
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
| `global.go` | GlobalDB, cost tracking, budgets, templates |
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

Transcripts store Claude Code session messages. `transcript.go:42-667`

### Transcript Schema

| Field | Purpose |
|-------|---------|
| `MessageUUID` | Claude session message ID (unique) |
| `ParentUUID` | Links to parent message (threading) |
| `Type` | `user`, `assistant` |
| `Content` | Full content JSON (preserves structure) |
| `InputTokens`, `OutputTokens` | Per-message usage |
| `CacheCreationTokens`, `CacheReadTokens` | Cache tracking |
| `ToolCalls`, `ToolResults` | JSON for tool interactions |

### Token Aggregation

```go
// Per-task aggregated usage (from assistant messages only)
usage, err := pdb.GetTaskTokenUsage(taskID)     // :313
usage, err := pdb.GetPhaseTokenUsage(taskID, phase) // :339
```

### Todo Snapshots

Progress tracking from Claude's TodoWrite tool calls:

```go
pdb.AddTodoSnapshot(snapshot)           // :386
snapshot, _ := pdb.GetLatestTodos(taskID)  // :405
history, _ := pdb.GetTodoHistory(taskID)   // :438
```

### Metrics Aggregation

```go
summary, _ := pdb.GetMetricsSummary(since)       // :538 - Total cost, tokens, by-model
daily, _ := pdb.GetDailyMetrics(since)           // :597 - Time series for charts
metrics, _ := pdb.GetTaskMetrics(taskID)         // :649 - Per-task breakdown
```

## Full-Text Search

```go
matches, err := pdb.SearchTranscripts(query)
// SQLite: FTS5 MATCH
// PostgreSQL: ILIKE
```

## Event Log System

Persisted executor events for timeline reconstruction and historical queries. `event_log.go`

### EventLog Schema

| Field | Type | Purpose |
|-------|------|---------|
| `TaskID` | string | Task this event belongs to |
| `Phase` | *string | Phase name (nullable for task-level events) |
| `Iteration` | *int | Iteration number (nullable) |
| `EventType` | string | Event type (phase, transcript, activity, tokens, etc.) |
| `Data` | any | JSON-serialized event payload |
| `Source` | string | Event origin: "executor", "api" |
| `CreatedAt` | time.Time | UTC timestamp |
| `DurationMs` | *int64 | Phase duration in ms (for completed phases) |

### Event Persistence

```go
// Save single event
pdb.SaveEvent(&db.EventLog{
    TaskID:    "TASK-001",
    Phase:     &phase,
    EventType: "phase",
    Data:      phaseUpdate,
    Source:    "executor",
    CreatedAt: time.Now(),
})

// Batch save (transactional, used by PersistentPublisher)
pdb.SaveEvents([]*db.EventLog{event1, event2, ...})
```

### Event Queries

```go
// Query with filters
events, err := pdb.QueryEvents(db.QueryEventsOptions{
    TaskID:     "TASK-001",
    Since:      &startTime,
    Until:      &endTime,
    EventTypes: []string{"phase", "transcript"},
    Limit:      100,
    Offset:     0,
})
// Returns: newest first (ORDER BY created_at DESC)
```

### QueryEventsOptions

| Field | Purpose |
|-------|---------|
| `TaskID` | Filter by task |
| `InitiativeID` | Filter by initiative (joins with tasks table) |
| `Since` | Events after this time |
| `Until` | Events before this time |
| `EventTypes` | Filter to specific event types |
| `Limit` | Max results (0 = unlimited) |
| `Offset` | Skip first N results |

### Event Queries with Titles

For timeline display, use `QueryEventsWithTitles` which joins with the tasks table:

```go
events, err := pdb.QueryEventsWithTitles(db.QueryEventsOptions{
    InitiativeID: "INIT-001",  // Filter by initiative
    Limit:        100,
})
// Returns: []EventLogWithTitle (includes TaskTitle field)

// Get total count for pagination
count, err := pdb.CountEvents(opts)
```

## Cost Tracking

### Basic (Deprecated)

```go
// Deprecated: Use RecordCostExtended for model and cache token tracking
gdb.RecordCost(projectID, taskID, phase, costUSD, inputTokens, outputTokens)
summary, err := gdb.GetCostSummary(projectID, since)
```

### Extended (Recommended)

```go
// Record cost with full model, cache, and duration tracking
entry := db.CostEntry{
    ProjectID:           projectID,
    TaskID:              taskID,
    Phase:               phase,
    Model:               db.DetectModel(modelID),  // opus, sonnet, haiku, unknown
    Iteration:           1,
    CostUSD:             0.015,
    InputTokens:         1000,
    OutputTokens:        500,
    CacheCreationTokens: 200,
    CacheReadTokens:     8000,
    TotalTokens:         9700,
    InitiativeID:        "INIT-001",
    DurationMs:          45678,  // Phase execution time in milliseconds
}
gdb.RecordCostExtended(entry)

// Query by model
costs, err := gdb.GetCostByModel(projectID, since)
// Returns: map[string]float64{"opus": 12.50, "sonnet": 3.20, ...}

// Time-series for charting (day, week, month granularity)
timeseries, err := gdb.GetCostTimeseries(projectID, since, "day")
// Returns: []CostAggregate with date buckets

// Pre-computed aggregates
gdb.UpdateCostAggregate(agg)  // Upsert
aggregates, err := gdb.GetCostAggregates(projectID, "2026-01-01", "2026-01-31")
```

### Budget Management

```go
// Set monthly budget
budget := db.CostBudget{
    ProjectID:             projectID,
    MonthlyLimitUSD:       100.00,
    AlertThresholdPercent: 80,
    CurrentMonth:          "2026-01",
}
gdb.SetBudget(budget)

// Get budget (nil if none configured)
budget, err := gdb.GetBudget(projectID)

// Get status with computed fields
status, err := gdb.GetBudgetStatus(projectID)
// Returns: *BudgetStatus{PercentUsed, OverBudget, AtAlertThreshold, ...}
```

### Model Detection

```go
model := db.DetectModel("claude-opus-4-5-20251101")  // "opus"
model := db.DetectModel("claude-sonnet-4-20250514")  // "sonnet"
model := db.DetectModel("claude-3-5-haiku-20241022") // "haiku"
model := db.DetectModel("unknown-model")             // "unknown"
```

## Workflow System

Workflow operations in `workflow.go` manage the configurable workflow system:

### Types

| Type | Purpose |
|------|---------|
| `PhaseTemplate` | Reusable phase definitions with prompt config |
| `Workflow` | Composed execution plans from phase templates |
| `WorkflowPhase` | Phase sequence within a workflow |
| `WorkflowVariable` | Custom variable definitions |
| `WorkflowRun` | Execution instance tracking |
| `WorkflowRunPhase` | Phase execution within a run |

### Operations

```go
// Phase templates
pdb.SavePhaseTemplate(pt)
pdb.GetPhaseTemplate(id)
pdb.ListPhaseTemplates()
pdb.DeletePhaseTemplate(id)

// Workflows
pdb.SaveWorkflow(w)
pdb.GetWorkflow(id)
pdb.ListWorkflows()
pdb.DeleteWorkflow(id)
pdb.GetWorkflowPhases(workflowID)
pdb.SaveWorkflowPhase(wp)
pdb.GetWorkflowVariables(workflowID)
pdb.SaveWorkflowVariable(wv)

// Workflow runs
pdb.SaveWorkflowRun(wr)
pdb.GetWorkflowRun(id)
pdb.ListWorkflowRuns(opts)
pdb.GetNextWorkflowRunID()
pdb.GetWorkflowRunPhases(runID)
pdb.SaveWorkflowRunPhase(wrp)
```

See `internal/workflow/CLAUDE.md` for built-in workflows and seeding.

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
