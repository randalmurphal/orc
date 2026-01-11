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
- `tasks` - Task records (id, title, weight, status, cost)
- `phases` - Phase execution state (iterations, tokens)
- `transcripts` - Claude conversation logs
- `transcripts_fts` - FTS5 virtual table with triggers (SQLite only)
- `initiatives` - Initiative groupings (id, title, status, vision, owner)
- `initiative_decisions` - Decisions within initiatives
- `initiative_tasks` - Task-to-initiative mappings with sequence
- `task_dependencies` - Task dependency relationships
- `subtasks` - Subtask queue (parent_task, title, status, proposed_by)
- `review_comments` - Inline review comments (file_path, line_number, severity, status)
- `team_members` - Organization members (email, display_name, initials, role)
- `task_claims` - Task assignments/claims (task_id, member_id, claimed_at)
- `activity_log` - Audit trail (action, task_id, member_id, details)

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
