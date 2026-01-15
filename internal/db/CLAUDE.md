# Database Package

Database persistence layer with driver abstraction supporting SQLite and PostgreSQL.

## Overview

This package provides two database types:
- **GlobalDB** (`~/.orc/orc.db`) - Cross-project data: project registry, cost logs, templates
- **ProjectDB** (`.orc/orc.db`) - Per-project data: tasks, phases, transcripts with FTS

## Key Types

| Type | Purpose |
|------|---------|
| `DB` | Core database wrapper with driver abstraction |
| `GlobalDB` | Global operations (extends DB) |
| `ProjectDB` | Project operations with FTS (extends DB) |
| `TxRunner` | Interface for transaction execution |
| `TxOps` | Transaction context for multi-table operations |
| `driver.Driver` | Interface for SQLite/PostgreSQL backends |
| `driver.Dialect` | Enum: `DialectSQLite`, `DialectPostgres` |
| `Transcript` | Transcript record with task/phase/content |
| `TranscriptMatch` | FTS search result with snippets |
| `CostSummary` | Aggregated cost statistics |

## Driver Package

The `driver/` subpackage provides database abstraction:

| Type | Purpose |
|------|---------|
| `Driver` | Interface for database operations |
| `SQLiteDriver` | SQLite implementation using modernc.org/sqlite |
| `PostgresDriver` | PostgreSQL implementation using lib/pq |
| `Tx` | Transaction interface |

### Driver Interface

```go
type Driver interface {
    Open(dsn string) error
    Close() error
    Exec(ctx, query, args...) (sql.Result, error)
    Query(ctx, query, args...) (*sql.Rows, error)
    QueryRow(ctx, query, args...) *sql.Row
    BeginTx(ctx, opts) (Tx, error)
    Migrate(ctx, schemaFS, schemaType) error
    Dialect() Dialect
    Placeholder(index int) string  // ? for SQLite, $1 for Postgres
    Now() string                   // datetime('now') or NOW()
    DB() *sql.DB
}
```

### Placeholder Differences

```go
// SQLite uses ? for all placeholders
drv.Placeholder(1) // "?"
drv.Placeholder(2) // "?"

// PostgreSQL uses $1, $2, etc.
drv.Placeholder(1) // "$1"
drv.Placeholder(2) // "$2"
```

## Schema Files

Embedded via `//go:embed schema/*.sql`:

| File | Purpose |
|------|---------|
| `schema/global_001.sql` | Projects, cost_log, templates tables |
| `schema/project_001.sql` | Detection, tasks, phases, transcripts, FTS |
| `schema/project_002.sql` | Initiatives, decisions, task dependencies |
| `schema/project_003.sql` | Subtasks queue table |
| `schema/project_004.sql` | Review comments with severity/status |
| `schema/project_005.sql` | Team members, task claims, activity log |
| `schema/project_006.sql` | Knowledge queue (patterns, gotchas, decisions) |
| `schema/project_007.sql` | Knowledge validation tracking (staleness) |
| `schema/project_008.sql` | Task comments (human/agent/system notes) |
| `schema/project_009.sql` | Task queue (backlog/active) and priority fields |
| `schema/project_010.sql` | Task category field (feature/bug/refactor/chore/docs/test) |
| `schema/project_011.sql` | Initiative dependencies (blocked_by relationships between initiatives) |
| `schema/project_012.sql` | Pure SQL storage: plans, specs, gate_decisions, attachments, sync_state tables; expanded task/phase columns for full state storage |

## Usage

### Default (SQLite)

```go
// Global DB (cross-project) - SQLite
gdb, err := db.OpenGlobal()
defer gdb.Close()

// Project DB (per-project) - SQLite
pdb, err := db.OpenProject("/path/to/project")
defer pdb.Close()
```

### With Dialect

```go
import "github.com/randalmurphal/orc/internal/db/driver"

// PostgreSQL global DB
gdb, err := db.OpenGlobalWithDialect(
    "postgres://user:pass@host/dbname",
    driver.DialectPostgres,
)

// PostgreSQL project DB
pdb, err := db.OpenProjectWithDialect(
    "postgres://user:pass@host/project_db",
    driver.DialectPostgres,
)

// Access dialect for conditional logic
if pdb.Dialect() == driver.DialectSQLite {
    // SQLite-specific code
}
```

## Key Features

### Transaction Support

Multi-table operations use transactions for atomicity:

```go
// TxRunner interface for transaction execution
type TxRunner interface {
    RunInTx(ctx context.Context, fn func(tx *TxOps) error) error
}

// ProjectDB implements TxRunner
err := pdb.RunInTx(ctx, func(tx *db.TxOps) error {
    if err := db.SaveTaskTx(tx, task); err != nil {
        return err  // Triggers rollback
    }
    if err := db.ClearTaskDependenciesTx(tx, taskID); err != nil {
        return err  // Triggers rollback
    }
    for _, depID := range blockedBy {
        if err := db.AddTaskDependencyTx(tx, taskID, depID); err != nil {
            return err  // Triggers rollback
        }
    }
    return nil  // Commits transaction
})
```

**Transaction-aware functions (TxOps):**
| Function | Purpose |
|----------|---------|
| `SaveTaskTx` | Save task within transaction |
| `ClearTaskDependenciesTx` | Clear task dependencies |
| `AddTaskDependencyTx` | Add task dependency |
| `ClearPhasesTx` | Clear task phases |
| `SavePhaseTx` | Save phase state |
| `AddGateDecisionTx` | Add gate decision |
| `SaveInitiativeTx` | Save initiative |
| `AddInitiativeDecisionTx` | Add initiative decision |
| `ClearInitiativeTasksTx` | Clear initiative task links |
| `AddTaskToInitiativeTx` | Add task to initiative |
| `ClearInitiativeDependenciesTx` | Clear initiative dependencies |
| `AddInitiativeDependencyTx` | Add initiative dependency |

### Automatic Migrations

```go
func (d *DB) Migrate(schemaType string) error
// Runs all schema/*.sql files matching pattern
// Safe to call multiple times (IF NOT EXISTS)
// PostgreSQL driver converts SQLite syntax automatically
```

### Full-Text Search

```go
func (p *ProjectDB) SearchTranscripts(query string) ([]TranscriptMatch, error)
// SQLite: Uses FTS5 with MATCH
// PostgreSQL: Uses ILIKE (basic search)
// Returns snippets with matches
```

### Cost Tracking

```go
func (g *GlobalDB) RecordCost(projectID, taskID, phase string, costUSD float64, inputTokens, outputTokens int) error
func (g *GlobalDB) GetCostSummary(projectID string, since time.Time) (*CostSummary, error)
```

## Schema Design

### Global Tables
- `projects` - Registered projects (id, name, path, language, created_at)
- `cost_log` - Token usage per phase (no FK for orphan entries)
- `templates` - Shared task templates (JSON phases)

### Project Tables
- `detection` - Project detection results (language, frameworks, build tools)
- `tasks` - Task records (id, title, weight, status, queue, priority, category, PR info, session tracking, tokens)
- `phases` - Phase execution state (iterations, tokens, commit_sha, skip_reason)
- `plans` - Phase plans (version, weight, phases JSON)
- `specs` - Task specifications (content, source)
- `specs_fts` - FTS5 virtual table for spec search (SQLite only)
- `transcripts` - Claude conversation logs
- `transcripts_fts` - FTS5 virtual table with triggers (SQLite only)
- `initiatives` - Initiative groupings (id, title, status, vision, owner)
- `initiative_decisions` - Decisions within initiatives
- `initiative_tasks` - Task-to-initiative mappings with sequence
- `initiative_dependencies` - Initiative blocked_by relationships
- `task_dependencies` - Task dependency relationships
- `gate_decisions` - Gate approval records (phase, gate_type, approved, reason)
- `task_attachments` - Task file attachments (BLOB storage)
- `subtasks` - Subtask queue (parent_task, title, status, proposed_by)
- `review_comments` - Inline review comments (file_path, line_number, severity, status)
- `team_members` - Organization members (email, display_name, initials, role)
- `task_claims` - Task assignments/claims (task_id, member_id, claimed_at)
- `activity_log` - Audit trail (action, task_id, member_id, details)
- `knowledge_queue` - Knowledge approval queue (patterns, gotchas, decisions)
- `task_comments` - Task comments/notes (author, author_type, content, phase)
- `sync_state` - P2P sync tracking (site_id, sync_version, mode)

## Dialect-Specific Queries

Some methods use dialect-aware queries:

| Method | SQLite | PostgreSQL |
|--------|--------|------------|
| `StoreDetection` | `INSERT OR REPLACE` | `INSERT ... ON CONFLICT DO UPDATE` |
| `AddTaskDependency` | `INSERT OR IGNORE` | `INSERT ... ON CONFLICT DO NOTHING` |
| `SearchTranscripts` | FTS5 MATCH | ILIKE |
| Timestamps | `datetime('now')` | `NOW()` |

## Testing

```bash
go test ./internal/db/... -v
```

Tests use `t.TempDir()` for isolated databases. PostgreSQL tests require a running PostgreSQL instance.
