# Multi-Project & Team Support Design

## Goal

Enable orc to work seamlessly across multiple repos (solo) and with shared team visibility (PostgreSQL), while ensuring GitHub and GitLab work equally well.

## Approach

Two clean modes based on config - no hybrid sync complexity:

- **SQLite mode (default):** Solo developer, multiple repos, works offline
- **PostgreSQL mode:** Team with shared visibility, everyone connects directly, requires network

## Success Criteria

- [ ] PostgreSQL works end-to-end (`orc init`, `orc new`, `orc run`, `orc serve`)
- [ ] Multi-project "My Work" dashboard shows tasks across all registered projects
- [ ] User attribution on tasks (created_by, claimed_by, assigned_to) with proper FK relationships
- [ ] Costs attributed to users, aggregatable by project and team
- [ ] Atomic claim-on-run prevents two users running same task simultaneously
- [ ] Claim history preserved in append-only audit log
- [ ] GitHub and GitLab both complete full task lifecycle
- [ ] `orc init` catches problems early (missing tokens, bad git setup, unknown hosting)

## Non-Goals

| Excluded | Rationale |
|----------|-----------|
| Sync layer / offline for PostgreSQL mode | Complexity; revisit if needed |
| Cross-project task dependencies | Projects are independent |
| Cross-project initiatives | Keep projects isolated |
| Full auth system (Okta, etc.) | Schema is auth-ready with `users` table; add auth layer when needed |
| GitHub GraphQL for auto-merge | Document limitation, workaround exists |
| User identity verification | Trusted team tool; config username is trusted |

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| No sync layer | PostgreSQL-direct is simpler; offline not required |
| `users` table in GlobalDB | Proper FK relationships, auth-ready, shared across projects |
| Atomic claim on `tasks` + append-only history | Best of both: fast atomic checks + full audit trail |
| DSN via env var only | No plaintext passwords in config files |
| Shared schema with `project_id` | Simpler than schema-per-project; unified dashboard needs cross-project queries |
| Heartbeat-based stale detection | Claim age is wrong metric; 10h task with fresh heartbeats is healthy |
| Budgets are soft blocks | Warn and require `--ignore-budget` to proceed, not hard gates |
| Dialect-specific FTS | SQLite FTS5 vs PostgreSQL tsvector can't be abstracted |
| pgxpool for connections | Trivial to add, future-proofs for scale |

## Constraints

| Constraint | Implication |
|------------|-------------|
| PostgreSQL mode requires network | No offline support - by design |
| `user.name` required for PostgreSQL mode | Config validation fails without it |
| FTS implementation differs by dialect | Search queries have explicit dialect branching |

---

## Design Details

### 1. Configuration

```yaml
# ~/.orc/config.yaml (global) or .orc/config.yaml (project)
database:
  dialect: sqlite          # "sqlite" (default) | "postgres"
  dsn_env: ""              # Env var containing PostgreSQL DSN (required for postgres mode)
                           # Example: ORC_DATABASE_DSN="postgres://user:pass@host:5432/orc"

user:
  name: ""                 # Display name, stored on tasks (required for postgres mode)
  email: ""                # Optional, for future auth integration

budgets: {}                # Empty = no limits (default)
# budgets:
#   team_monthly: 2000.00
#   user_monthly: 500.00
#   project_monthly:
#     orc: 800.00
```

**Validation rules:**
- `dialect: postgres` requires `dsn_env` to be set and env var to be non-empty
- `dialect: postgres` requires `user.name` to be set
- Invalid config = clear error at startup

### 2. Schema Design

#### Users Table (GlobalDB)

```sql
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT UNIQUE NOT NULL,
    email TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

- Lives in GlobalDB (shared across all projects)
- Auto-created on first use of a `user.name`
- All user columns in other tables are FKs to `users.id`

#### Task User Columns (ProjectDB)

```sql
ALTER TABLE tasks ADD COLUMN created_by UUID REFERENCES users(id);
ALTER TABLE tasks ADD COLUMN assigned_to UUID REFERENCES users(id);
ALTER TABLE tasks ADD COLUMN claimed_by UUID REFERENCES users(id);
ALTER TABLE tasks ADD COLUMN claimed_at TIMESTAMP WITH TIME ZONE;
```

#### Claim History (ProjectDB)

```sql
CREATE TABLE task_claims (
    id SERIAL PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id),
    user_id UUID NOT NULL REFERENCES users(id),
    claimed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    released_at TIMESTAMP WITH TIME ZONE,
    stolen_from UUID REFERENCES users(id),  -- Non-null if this was a force-steal
    CONSTRAINT fk_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX idx_task_claims_task ON task_claims(task_id);
CREATE INDEX idx_task_claims_user ON task_claims(user_id);
```

- Append-only audit log
- `stolen_from` records who was displaced when `--force` used
- Never deleted, only new rows inserted

#### Cost Attribution (GlobalDB)

```sql
ALTER TABLE cost_log ADD COLUMN user_id UUID REFERENCES users(id);
CREATE INDEX idx_cost_log_user ON cost_log(user_id);
CREATE INDEX idx_cost_log_project_user ON cost_log(project_id, user_id);
```

#### Other Tables

| Table | New Columns |
|-------|-------------|
| `initiatives` | `created_by UUID`, `owned_by UUID` |
| `phases` | `executed_by UUID` |
| `workflow_runs` | `started_by UUID` |

All reference `users(id)` in GlobalDB.

### 3. PostgreSQL Schema Strategy

**Shared schema with `project_id` columns** (not schema-per-project).

Rationale:
- Unified dashboard needs cross-project queries
- Team trusts each other (same credentials anyway)
- Simpler than managing schemas per project

```sql
-- All project-scoped tables have project_id
CREATE TABLE tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    ...
);

CREATE INDEX idx_tasks_project ON tasks(project_id);

-- Unified dashboard query is trivial
SELECT * FROM tasks
WHERE status IN ('running', 'blocked', 'ready')
ORDER BY project_id, updated_at;
```

### 4. Claim-on-Run

**Atomic claim** using single UPDATE:

```sql
UPDATE tasks
SET claimed_by = $1,
    claimed_at = NOW()
WHERE id = $2
  AND (claimed_by IS NULL OR claimed_by = $1)
RETURNING id;
```

- Returns row = claim succeeded
- Returns nothing = someone else has it
- No check-then-insert race condition

**On successful claim**, also insert history:

```sql
INSERT INTO task_claims (task_id, user_id, claimed_at, stolen_from)
VALUES ($1, $2, NOW(), NULL);
```

**Force steal** (when `--force` used):

```sql
-- First get current claimer
SELECT claimed_by FROM tasks WHERE id = $1;

-- Then force update
UPDATE tasks SET claimed_by = $2, claimed_at = NOW() WHERE id = $1;

-- Record in history with stolen_from
INSERT INTO task_claims (task_id, user_id, claimed_at, stolen_from)
VALUES ($1, $2, NOW(), $3);  -- $3 = previous claimer
```

**CLI behavior:**

```bash
$ orc run TASK-001
Error: task TASK-001 is claimed by alice (last heartbeat 2 minutes ago)
Use --force to steal the claim

$ orc run TASK-001 --force
Warning: Stealing claim from alice
Starting TASK-001...
```

### 5. Stale Claim Detection

Based on **heartbeat freshness**, not claim age.

```go
const StaleHeartbeatThreshold = 10 * time.Minute  // Configurable

func IsClaimStale(task *Task) bool {
    if task.ClaimedBy == "" {
        return false
    }
    return time.Since(task.LastHeartbeat) > StaleHeartbeatThreshold
}
```

- 10-hour task with fresh heartbeats = healthy, not stealable without `--force`
- 30-minute claim with no heartbeat for 15min = stale, warning shown

**`orc status` output:**

```
TASK-001  Running   alice   (stale - no heartbeat for 23 minutes)
TASK-002  Running   bob     (healthy)
```

### 6. Release Command

New command: `orc release TASK-001`

```go
func releaseTask(taskID string, userID uuid.UUID) error {
    // Verify current user owns the claim
    result, err := db.Exec(`
        UPDATE tasks
        SET claimed_by = NULL, claimed_at = NULL
        WHERE id = $1 AND claimed_by = $2
        RETURNING id
    `, taskID, userID)

    if rowsAffected == 0 {
        return fmt.Errorf("task not claimed by you")
    }

    // Record release in history
    db.Exec(`
        UPDATE task_claims
        SET released_at = NOW()
        WHERE task_id = $1 AND user_id = $2 AND released_at IS NULL
    `, taskID, userID)

    return nil
}
```

### 7. Budget Enforcement

**Soft block** - warns and requires flag to proceed.

```go
func (e *Executor) checkBudget(ctx context.Context) error {
    status, err := e.globalDB.GetBudgetStatus(e.projectID, e.userID)
    if err != nil || status == nil {
        return nil  // No budget configured = no limit
    }

    if status.OverLimit {
        return &BudgetExceededError{
            Spent: status.CurrentSpent,
            Limit: status.Limit,
        }
    }

    if status.AtWarningThreshold {  // 80%
        e.logger.Warn("approaching budget limit",
            "spent", status.CurrentSpent,
            "limit", status.Limit,
            "percent", status.PercentUsed)
    }

    return nil
}

// In Run()
if err := e.checkBudget(ctx); err != nil {
    if errors.Is(err, &BudgetExceededError{}) && !opts.IgnoreBudget {
        return fmt.Errorf("%w\nUse --ignore-budget to proceed anyway", err)
    }
}
```

### 8. Full-Text Search

**Dialect-specific implementations:**

```go
func (p *ProjectDB) SearchTranscripts(ctx context.Context, query string) ([]Transcript, error) {
    var sqlQuery string

    if p.Dialect() == driver.DialectSQLite {
        sqlQuery = `
            SELECT t.* FROM transcripts t
            JOIN transcript_fts fts ON t.id = fts.rowid
            WHERE fts.content MATCH ?
        `
    } else {
        sqlQuery = `
            SELECT * FROM transcripts
            WHERE to_tsvector('english', content) @@ plainto_tsquery('english', $1)
        `
    }

    return p.queryTranscripts(ctx, sqlQuery, query)
}
```

**PostgreSQL migration:**

```sql
-- Create GIN index for fast full-text search
CREATE INDEX idx_transcripts_fts ON transcripts
USING GIN(to_tsvector('english', content));
```

### 9. Connection Pooling

Use `pgxpool` for PostgreSQL connections:

```go
import "github.com/jackc/pgx/v5/pgxpool"

func NewPostgresDriver() *PostgresDriver {
    return &PostgresDriver{}
}

func (d *PostgresDriver) Open(dsn string) error {
    config, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return fmt.Errorf("parse dsn: %w", err)
    }

    // Sensible defaults for CLI tool
    config.MaxConns = 10
    config.MinConns = 2
    config.MaxConnLifetime = time.Hour

    pool, err := pgxpool.NewWithConfig(context.Background(), config)
    if err != nil {
        return fmt.Errorf("create pool: %w", err)
    }

    d.pool = pool
    return nil
}
```

### 10. Unified Dashboard

**New landing page** showing all projects:

```
┌─────────────────────────────────────────────────────────────────┐
│  orc                                        [randy] [settings]  │
├─────────────────────────────────────────────────────────────────┤
│  MY WORK                                     Filter: [All ▾]    │
│                                                                 │
│  ┌─ orc ──────────────────────────────────────────────────────┐ │
│  │  ● TASK-042 Running    Add unified dashboard    2m         │ │
│  │  ○ TASK-041 Blocked    Fix GitLab auto-merge    (bob)      │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌─ llmkit ───────────────────────────────────────────────────┐ │
│  │  ○ TASK-018 Ready      Add streaming support               │ │
│  └────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

**API addition:**

```protobuf
rpc GetAllProjectsStatus(GetAllProjectsStatusRequest) returns (GetAllProjectsStatusResponse);

message GetAllProjectsStatusRequest {
  string user_id = 1;  // Optional: filter to user's tasks
}

message GetAllProjectsStatusResponse {
  repeated ProjectStatus projects = 1;
}

message ProjectStatus {
  string project_id = 1;
  string project_name = 2;
  string project_path = 3;
  repeated TaskSummary active_tasks = 4;
  int32 total_tasks = 5;
  int32 completed_today = 6;
}

message TaskSummary {
  string id = 1;
  string title = 2;
  string status = 3;
  string claimed_by_name = 4;
  google.protobuf.Timestamp claimed_at = 5;
  bool is_stale = 6;
}
```

### 11. Cost Attribution

Costs roll up: user → project → team.

```
Team Total: $847.32 this month
├── orc: $412.18
│   ├── randy: $287.41
│   └── alice: $98.22
└── llmkit: $312.50
    └── randy: $312.50
```

**Aggregation queries:**

```sql
-- Team total
SELECT SUM(cost_usd) FROM cost_log
WHERE timestamp >= date_trunc('month', NOW());

-- By project
SELECT project_id, SUM(cost_usd) FROM cost_log
WHERE timestamp >= date_trunc('month', NOW())
GROUP BY project_id;

-- By user within project
SELECT u.name, SUM(c.cost_usd) FROM cost_log c
JOIN users u ON c.user_id = u.id
WHERE c.project_id = $1 AND c.timestamp >= date_trunc('month', NOW())
GROUP BY u.name;
```

### 12. GitHub/GitLab Parity

**Current gaps:**

| Feature | GitHub | GitLab | Action |
|---------|--------|--------|--------|
| Auto-merge | ❌ (needs GraphQL) | ✅ | Document limitation |
| Hosting detection | Lazy (at PR time) | Lazy | Move to init |

**Changes:**

1. **Document auto-merge gap** in config comments and CLI
2. **Move hosting detection to `orc init`**
3. **Validate token** exists during init

**Init verification output:**

```
✓ Git repository detected
✓ Remote 'origin' found: github.com/randalmurphal/orc
✓ Hosting provider: GitHub
✓ GITHUB_TOKEN found
✓ ANTHROPIC_API_KEY found
```

**Auto-merge warning:**

```bash
$ orc run TASK-001
Warning: auto_merge is enabled but not supported on GitHub (requires GraphQL)
Consider using merge_on_ci_pass instead
```

---

## Migration Path

### SQLite Migrations to Port

~65 SQLite migrations need PostgreSQL equivalents. Key differences:

| SQLite | PostgreSQL |
|--------|------------|
| `AUTOINCREMENT` | `SERIAL` or `GENERATED ALWAYS AS IDENTITY` |
| `datetime('now')` | `NOW()` |
| `strftime('%Y-%m-%d', x)` | `TO_CHAR(x, 'YYYY-MM-DD')` |
| `INTEGER` for bool | `BOOLEAN` |
| FTS5 virtual tables | `tsvector` + GIN indexes |
| `?` placeholders | `$1, $2, ...` placeholders |

### SQLite-isms in Go Code (Completed - TASK-790)

All hardcoded SQLite date functions replaced with driver helpers. `transcript.go` FTS remains (addressed separately in TASK-791).

| File | Pattern Removed | Replaced With |
|------|----------------|---------------|
| `global.go` | `strftime()` | `driver.DateFormat()` |
| `branch.go` | `strftime()` | `driver.DateFormat()` + `driver.Now()` |
| `phase_output.go` | `datetime('now')` | `driver.Now()` |
| `project.go` | `datetime('now')` | `driver.Now()` |
| `subtask.go` | `datetime('now')` | `driver.Now()` |

Regression tests in `sqlite_isms_test.go` prevent re-introduction of hardcoded patterns.

---

## Testing Strategy

| Area | Test Approach |
|------|---------------|
| PostgreSQL backend | Integration tests with testcontainers |
| Claim atomicity | Concurrent goroutines attempting same claim |
| Claim history | Verify append-only, stolen_from populated |
| Budget enforcement | Unit tests for threshold logic |
| FTS both dialects | Search tests run on both SQLite and PostgreSQL |
| Dashboard API | Integration tests with multiple projects |
| Init validation | Tests for various git/token states |

---

## Future Considerations (Not in Scope)

- **Okta/OIDC auth:** `users` table ready, add auth middleware when needed
- **Real-time dashboard:** WebSocket push instead of polling
- **Cross-project views:** If projects need to relate, revisit architecture
- **User roles/permissions:** Add `user_roles` table when needed
