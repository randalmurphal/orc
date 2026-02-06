# PostgreSQL & Multi-Project Team Support (INIT-041)

## What This Initiative Does

This initiative transforms orc from a single-user SQLite tool into a dual-mode system that can also run on PostgreSQL for team environments. The core idea is simple: **one config switch** flips orc from solo mode to team mode, and everything else follows from that.

| Mode | Database | Use Case | Network |
|------|----------|----------|---------|
| **SQLite** (default) | Local files at `~/.orc/` | Solo developer, multiple repos | Offline-capable |
| **PostgreSQL** | Shared server via DSN | Team with shared visibility | Requires network |

There's no hybrid sync, no conflict resolution layer, no eventual consistency. You pick a mode and it works cleanly in that mode. SQLite users see zero changes to their workflow.

---

## The Problem It Solves

Orc currently works great for one person on one machine. But when a team of developers all use orc on the same codebase, they have no way to:

1. **See each other's work** - No shared dashboard, no way to know Alice is already running TASK-042
2. **Prevent collisions** - Two people can `orc run TASK-001` simultaneously, causing conflicts
3. **Track costs per person** - API spend is unattributed, impossible to budget
4. **Get a unified view** - Managing tasks across multiple repos means switching between project databases

This initiative adds all four capabilities while preserving the solo developer experience exactly as-is.

---

## Architecture

### Dual Database Modes

```
SQLite Mode (default)                    PostgreSQL Mode
========================                 ========================
~/.orc/orc.db        (GlobalDB)         postgres://host/orc
~/.orc/projects/                           (single shared DB)
  ├── proj-a/orc.db  (ProjectDB)           ├── global tables
  └── proj-b/orc.db  (ProjectDB)           ├── project tables
                                            │   (project_id column)
                                            └── cross-project queries
```

In SQLite mode, each project gets its own database file. In PostgreSQL mode, all projects share one database, distinguished by a `project_id` column. This makes cross-project queries (like "show me all my running tasks") trivial in PostgreSQL but requires iterating databases in SQLite.

### Driver Abstraction

The existing `internal/db/driver/` package already abstracts the database layer:

```go
type Driver interface {
    Open(dsn string) error
    Exec(ctx, query, args...) (sql.Result, error)
    Query(ctx, query, args...) (*sql.Rows, error)
    Dialect() Dialect           // "sqlite" or "postgres"
    Placeholder(index int) string  // ? vs $1
    Now() string                // datetime('now') vs NOW()
}
```

Both `SQLiteDriver` and `PostgresDriver` implement this interface. Application code calls `driver.Now()` instead of hardcoding SQL functions, and `driver.Placeholder()` instead of hardcoding parameter syntax.

### Configuration

```yaml
# ~/.orc/config.yaml
database:
  dialect: sqlite          # "sqlite" (default) | "postgres"
  dsn_env: ""              # Env var name containing PostgreSQL DSN
                           # (never store passwords in config files)

user:
  name: "randy"            # Required for postgres mode
  email: "randy@dev.com"   # Optional
```

PostgreSQL mode has two hard requirements: `dsn_env` must point to a valid env var, and `user.name` must be set. Without these, orc fails at startup with a clear error.

---

## Feature Breakdown

### 1. User Identity System (TASK-782 - Completed)

The foundation everything else depends on. A `users` table in GlobalDB with UUID primary keys:

```sql
CREATE TABLE users (
    id TEXT PRIMARY KEY,     -- UUID as text (SQLite compat)
    name TEXT UNIQUE NOT NULL,
    email TEXT,
    created_at TEXT
);
```

Every table that needs user attribution now has columns referencing `users.id`:

| Table | New Columns | Purpose |
|-------|-------------|---------|
| `tasks` | `created_by`, `assigned_to` | Who created/owns the task |
| `initiatives` | `created_by`, `owned_by` | Initiative ownership |
| `phases` | `executed_by` | Who ran the phase |
| `workflow_runs` | `started_by` | Who kicked off the run |
| `cost_log` | `user_id` | Cost attribution per person |

Users are auto-created on first use via `GetOrCreateUser(name)` - set your name in config and it handles the rest. Idempotent, race-condition safe.

### 2. Database Dialect Configuration (TASK-783 - In Progress)

Adds the config schema and validation logic for switching between SQLite and PostgreSQL. Validates that PostgreSQL mode has all required settings before any database operations begin.

### 3. Atomic Claim-on-Run System (TASK-784 - In Progress)

The core concurrency feature. When you `orc run TASK-001`, it atomically claims the task:

```sql
UPDATE tasks
SET claimed_by = ?, claimed_at = datetime('now')
WHERE id = ?
  AND (claimed_by IS NULL OR claimed_by = ?)
RETURNING id;
```

If someone else already has it, the UPDATE returns zero rows and you get a clear error:

```
Error: task TASK-001 is claimed by alice (last heartbeat 2 minutes ago)
Use --force to steal the claim
```

Force-stealing is tracked in an append-only `task_claims` history table, recording who had the claim before and when it was stolen. This audit trail is never deleted.

This task also **deletes** the old `internal/db/team.go` - the previous claim system had a check-then-insert race condition that the new atomic approach eliminates.

### 4. Release Command (TASK-785)

New CLI command: `orc release TASK-001`. Explicitly gives up a claim so another team member can run the task. Only works if you're the current claimer. Records the release in claim history.

### 5. Heartbeat-Based Stale Detection (TASK-786)

Replaces age-based stale detection with heartbeat freshness. A 10-hour task with active heartbeats is healthy; a 30-minute claim with no heartbeat for 15 minutes is stale. This distinction matters because long-running tasks are normal.

```
TASK-001  Running   alice   (stale - no heartbeat for 23 minutes)
TASK-002  Running   bob     (healthy - heartbeat 30s ago)
```

Stale claims show warnings in `orc status` but don't auto-release. The developer can use `--force` to take over.

### 6. Budget Enforcement (TASK-787)

Wires the existing `GetBudgetStatus()` into the executor as a soft block. Budgets are configured per team, per user, or per project:

```yaml
budgets:
  team_monthly: 2000.00
  user_monthly: 500.00
  project_monthly:
    orc: 800.00
```

Over budget? Clear error with an escape hatch:

```
Error: Budget exceeded ($2,143.00 / $2,000.00 monthly limit)
Use --ignore-budget to proceed anyway
```

At 80% usage, a warning is logged but execution continues. No budget configured means no limits.

### 7. PostgreSQL Migration Port (TASK-788, 795, 796, 797)

The largest mechanical effort: porting all ~65 SQLite migrations to PostgreSQL syntax. Split across 4 tasks by migration range for parallelism.

Key syntax translations:

| SQLite | PostgreSQL |
|--------|------------|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `SERIAL PRIMARY KEY` |
| `TEXT` for timestamps | `TIMESTAMP WITH TIME ZONE` |
| `datetime('now')` | `NOW()` |
| `strftime('%Y-%m-%d', x)` | `TO_CHAR(x, 'YYYY-MM-DD')` |
| `INTEGER` for booleans | `BOOLEAN` |
| `?` placeholders | `$1, $2, ...` |
| FTS5 virtual tables | `tsvector` + GIN indexes |

Currently 7 PostgreSQL migration files exist (`global_001.sql` plus `project_001.sql` through `project_006.sql`). The remaining ~58 need to be written.

### 8. Connection Pooling (TASK-789)

Replaces `lib/pq` with `pgxpool` (from the `jackc/pgx` library) for connection pooling. The current PostgreSQL driver uses raw `sql.Open` with no pool management. With `pgxpool`:

- 10 max connections, 2 min idle
- 1-hour max connection lifetime
- Proper pool health checks

This is a straightforward swap since the driver already abstracts the connection layer.

### 9. SQLite-ism Fixes (TASK-790)

Several Go files have SQLite-specific SQL hardcoded (like `strftime()` and `datetime('now')`). This task adds dialect-aware helpers to the driver interface:

```go
DateFormat(column, format string) string  // strftime vs TO_CHAR
DateTrunc(unit, column string) string     // sqlite workaround vs date_trunc
```

Files affected: `global.go`, `branch.go`, `phase_output.go`, `project.go`, `subtask.go`.

### 10. PostgreSQL Full-Text Search (TASK-791)

SQLite uses FTS5 virtual tables with `MATCH` operator. PostgreSQL uses `tsvector`/`tsquery` with GIN indexes. These are fundamentally different approaches that can't be abstracted, so the search code branches on dialect:

```go
if p.Dialect() == driver.DialectSQLite {
    // SELECT ... WHERE content MATCH ?
} else {
    // SELECT ... WHERE to_tsvector('english', content) @@ plainto_tsquery('english', $1)
}
```

PostgreSQL gets a GIN index for performance: `CREATE INDEX ... USING GIN(to_tsvector('english', content))`.

### 11. Cross-Project Dashboard API (TASK-792)

New `GetAllProjectsStatus` API endpoint that aggregates active tasks across all registered projects. In PostgreSQL mode this is a single efficient query. In SQLite mode it iterates through the project database cache.

Returns per-project summaries: active tasks, claim status, stale indicators, completion counts.

### 12. "My Work" Dashboard UI (TASK-793)

New landing page replacing the current project picker gate. Shows all your tasks across all projects in one view:

```
MY WORK                                     Filter: [All]

┌─ orc ──────────────────────────────────────────────────┐
│  ● TASK-042 Running    Add unified dashboard    2m     │
│  ○ TASK-041 Blocked    Fix GitLab auto-merge    (bob)  │
└────────────────────────────────────────────────────────┘

┌─ llmkit ───────────────────────────────────────────────┐
│  ○ TASK-018 Ready      Add streaming support           │
└────────────────────────────────────────────────────────┘
```

Click any task to jump into its project context. The project picker becomes a filter rather than a mandatory gate.

### 13. Init Validation (TASK-794 - Failed, needs retry)

Moves hosting provider detection (GitHub/GitLab) from PR-time to `orc init` time. Validates tokens exist and work early, so you don't discover missing `GITHUB_TOKEN` halfway through a task run.

### 14. Cost Viewing (TASK-798)

CLI, API, and UI for viewing cost data with user attribution. Costs roll up from user to project to team:

```
Team Total: $847.32 this month
├── orc: $412.18
│   ├── randy: $287.41
│   └── alice: $98.22
└── llmkit: $312.50
    └── randy: $312.50
```

---

## Current Progress

| Task | Description | Status |
|------|-------------|--------|
| TASK-782 | Users table and user attribution schema | Completed |
| TASK-783 | Database dialect configuration | In Progress |
| TASK-784 | Atomic claim-on-run with history | In Progress |
| TASK-785 | Release command | Planned |
| TASK-786 | Heartbeat-based stale detection | Planned |
| TASK-787 | Budget enforcement in executor | Completed |
| TASK-788 | Port global SQLite migrations to PostgreSQL | Planned |
| TASK-789 | pgxpool connection pooling | Planned |
| TASK-790 | Fix SQLite-isms in Go code | Completed |
| TASK-791 | PostgreSQL full-text search | Planned |
| TASK-792 | Cross-project status API endpoint | Planned |
| TASK-793 | "My Work" dashboard UI | Planned |
| TASK-794 | Hosting detection and token validation | Failed (needs retry) |
| TASK-795 | Port project migrations 001-020 | Completed |
| TASK-796 | Port project migrations 021-040 | Planned |
| TASK-797 | Port project migrations 041-056 | Planned |
| TASK-798 | Cost viewing CLI, API, and UI | Planned |

**4 completed, 2 in progress, 10 planned, 1 failed.**

---

## Dependency Graph

```
TASK-782 (users table) ─────────┬──→ TASK-784 (atomic claims) ──┬──→ TASK-785 (release cmd)
         │                      │                                ├──→ TASK-786 (heartbeat stale)
         │                      │                                ├──→ TASK-788 (PG migrations) ──┬──→ TASK-789 (pgxpool)
         │                      │                                │                               ├──→ TASK-795 (port 001-020)
         │                      │                                │                               ├──→ TASK-796 (port 021-040)
         │                      │                                │                               └──→ TASK-797 (port 041-056)
         │                      │                                └──→ TASK-792 (dashboard API) ──→ TASK-793 (My Work UI)
         │                      └──→ TASK-787 (budget enforcement)
         └──→ TASK-783 (dialect config) ─────────────────────────────→ TASK-789 (pgxpool)

TASK-788 ──→ TASK-790 (fix SQLite-isms)
TASK-788 ──→ TASK-791 (PG full-text search)
TASK-786 ──→ TASK-798 (cost viewing)

Independent: TASK-794 (init validation)
```

The critical path runs through TASK-782 (done) → TASK-784 (claims) → TASK-788 (PG migrations) → TASK-795/796/797 (migration ports).

---

## What Doesn't Change

This initiative is explicitly scoped. Things NOT included:

| Excluded | Why |
|----------|-----|
| Sync layer / offline for PostgreSQL | Unnecessary complexity; pick a mode |
| Cross-project task dependencies | Projects are independent units |
| Cross-project initiatives | Keep projects isolated |
| Full auth system (Okta, OIDC) | Schema is auth-ready; add when needed |
| GitHub GraphQL for auto-merge | Document the limitation, workaround exists |
| User identity verification | Trusted team tool; config username is trusted |
| User roles/permissions | Future work, `users` table is ready for it |

---

## Design Document

Full technical details including SQL schemas, claim algorithms, budget logic, FTS implementation, and API protobuf definitions are in [`docs/designs/MULTI_PROJECT_TEAM_SUPPORT.md`](docs/designs/MULTI_PROJECT_TEAM_SUPPORT.md).
