# Database Package

Database persistence layer with driver abstraction supporting SQLite and PostgreSQL.

## Overview

| Type | Path | Purpose |
|------|------|---------|
| **GlobalDB** | `~/.orc/orc.db` | Cross-project: projects, cost_log, templates |
| **ProjectDB** | `.orc/orc.db` | Per-project: tasks, phases, transcripts, FTS |

## Key Types

| Type | Purpose |
|------|---------|
| `DB` | Core wrapper with driver abstraction |
| `GlobalDB` | Global operations (extends DB) |
| `ProjectDB` | Project operations with FTS (extends DB) |
| `TxRunner` | Transaction execution interface |
| `TxOps` | Transaction context (stores context for cancellation) |
| `driver.Driver` | SQLite/PostgreSQL backend interface |

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

**Batch functions:** `GetAllTaskDependencies()`, `GetAllInitiativeDecisions()`, `GetAllInitiativeTaskRefs()`, `GetAllInitiativeDependencies()`, `GetAllInitiativeDependents()`

## Full-Text Search

```go
matches, err := pdb.SearchTranscripts(query)
// SQLite: FTS5 MATCH
// PostgreSQL: ILIKE
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
// Record cost with full model and cache tracking
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
