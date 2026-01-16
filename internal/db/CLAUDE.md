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

```go
gdb.RecordCost(projectID, taskID, phase, costUSD, inputTokens, outputTokens)
summary, err := gdb.GetCostSummary(projectID, since)
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
