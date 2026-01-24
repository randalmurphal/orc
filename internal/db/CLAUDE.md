# Database Package

SQLite/PostgreSQL persistence with driver abstraction. `GlobalDB` (`~/.orc/orc.db`) for cross-project data, `ProjectDB` (`.orc/orc.db`) for per-project data with FTS.

## Architecture

**Core types**: `DB` (driver abstraction), `GlobalDB` (cost, projects), `ProjectDB` (tasks, FTS), `TxRunner` (transactions)

**CRUD files**: `task.go`, `initiative.go`, `phase.go`, `transcript.go`, `plan.go`, `workflow.go`, `event_log.go`, `gate_decision.go`, `attachment.go`, `constitution.go`

**Key patterns**: Transaction support via `RunInTx`, batch loading for N+1 avoidance, FTS via `transcript.go`

## Usage Patterns

**Open**: `OpenGlobal()`, `OpenProject(path)`, `OpenGlobalWithDialect(dsn, dialect)` for PostgreSQL

**Transactions**: `RunInTx(ctx, func(tx *TxOps) error {...})` - auto rollback on error

**Batch loading**: `GetAllTaskDependencies()`, `GetAllInitiativeDecisions()` - avoid N+1

## Key Subsystems

**Transcripts** (`transcript.go`): Claude session messages, token aggregation, FTS search, todo snapshots

**Events** (`event_log.go`): Timeline reconstruction, `QueryEvents()` with filters (task, time, type), `QueryEventsWithTitles()` for UI

**Cost tracking** (`global.go`): `RecordCostExtended()` with model/cache tokens, `GetCostByModel()`, budget management

**Workflows** (`workflow.go`): `PhaseTemplate`, `Workflow`, `WorkflowRun` CRUD. See `internal/workflow/CLAUDE.md` for built-in workflows.

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
