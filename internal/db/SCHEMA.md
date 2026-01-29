# Database Schema

Detailed schema documentation for global and project databases.

## Schema Files

Embedded via `//go:embed schema/*.sql`:

| File | Purpose |
|------|---------|
| `schema/global_001.sql` | Projects, cost_log (basic), templates tables |
| `schema/global_002.sql` | cost_log extensions (model, cache tokens), cost_aggregates, cost_budgets |
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
| `schema/project_011.sql` | Initiative dependencies (blocked_by relationships) |
| `schema/project_012.sql` | Pure SQL storage: plans, specs, gate_decisions, attachments, sync_state |
| `schema/project_025.sql` | Constitution tables for project principles and spec validation |

## Global Tables

| Table | Columns | Purpose |
|-------|---------|---------|
| `projects` | id, name, path, language, created_at | Registered projects |
| `cost_log` | project_id, task_id, phase, model, iteration, cost_usd, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, total_tokens, initiative_id, timestamp | Token usage with model tracking (no FK) |
| `cost_aggregates` | project_id, model, phase, date, total_cost_usd, total_input_tokens, total_output_tokens, total_cache_tokens, turn_count, task_count | Pre-computed time-series for dashboards |
| `cost_budgets` | project_id, monthly_limit_usd, alert_threshold_percent, current_month, current_month_spent | Monthly budget tracking |
| `templates` | id, name, phases (JSON), created_at | Shared task templates |

### cost_log Extended Columns (global_002.sql)

| Column | Type | Default | Description |
|--------|------|---------|-------------|
| `model` | TEXT | `''` | Claude model (opus, sonnet, haiku) |
| `iteration` | INTEGER | `0` | Iteration within phase |
| `cache_creation_tokens` | INTEGER | `0` | Tokens written to prompt cache |
| `cache_read_tokens` | INTEGER | `0` | Tokens served from cache |
| `total_tokens` | INTEGER | `0` | Effective total tokens |
| `initiative_id` | TEXT | `''` | Links to initiative |

### cost_log Indexes

| Index | Columns | Purpose |
|-------|---------|---------|
| `idx_cost_model` | model | Model-based queries |
| `idx_cost_model_timestamp` | model, timestamp | Time-range by model |
| `idx_cost_initiative` | initiative_id | Initiative cost analysis |
| `idx_cost_project_timestamp` | project_id, timestamp | Project timeline |

## Project Tables

### Core Task Tables

| Table | Key Columns | Purpose |
|-------|-------------|---------|
| `tasks` | id, title, description, weight, status, queue, priority, category, initiative_id, pr_number, session_id, total_tokens | Task records |
| `phases` | task_id, phase, status, iterations, input_tokens, output_tokens, cached_tokens, commit_sha, skip_reason | Phase state |
| `plans` | task_id, version, weight, phases (JSON) | Phase plans |
| `specs` | task_id, content, source, updated_at | Task specifications |
| `transcripts` | task_id, phase, timestamp, content | Claude logs |

### Dependency Tables

| Table | Columns | Purpose |
|-------|---------|---------|
| `task_dependencies` | task_id, blocked_by_id | Task blocked_by relationships |
| `initiative_dependencies` | initiative_id, blocked_by_id | Initiative blocked_by relationships |

### Initiative Tables

| Table | Columns | Purpose |
|-------|---------|---------|
| `initiatives` | id, title, vision, status, owner, created_at | Initiative groupings |
| `initiative_tasks` | initiative_id, task_id, sequence | Task-to-initiative mappings |
| `initiative_decisions` | initiative_id, decision, rationale, decided_at | Decisions within initiatives |

### Auxiliary Tables

| Table | Purpose |
|-------|---------|
| `detection` | Project detection results (language, frameworks) |
| `gate_decisions` | Gate approval records |
| `task_attachments` | Task file attachments (BLOB) |
| `subtasks` | Subtask queue (parent, title, status) |
| `review_comments` | Inline review comments |
| `team_members` | Organization members |
| `task_claims` | Task assignments |
| `activity_log` | Audit trail |
| `task_comments` | Task comments/notes |
| `sync_state` | P2P sync tracking |

### FTS Tables (SQLite only)

| Table | Purpose |
|-------|---------|
| `specs_fts` | FTS5 virtual table for spec search |
| `transcripts_fts` | FTS5 virtual table with triggers |

## Dialect-Specific Queries

| Method | SQLite | PostgreSQL |
|--------|--------|------------|
| `StoreDetection` | `INSERT OR REPLACE` | `INSERT ... ON CONFLICT DO UPDATE` |
| `AddTaskDependency` | `INSERT OR IGNORE` | `INSERT ... ON CONFLICT DO NOTHING` |
| `SearchTranscripts` | FTS5 MATCH | ILIKE |
| Timestamps | `datetime('now')` | `NOW()` |
| Placeholders | `?` | `$1, $2, ...` |

## Transaction-Aware Functions

Functions accepting `*TxOps` for multi-table atomic operations:

| Function | Purpose |
|----------|---------|
| `SaveTaskTx` | Save task |
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

## Batch Loading Functions

For N+1 query avoidance:

| Function | Returns |
|----------|---------|
| `GetAllTaskDependencies()` | `map[string][]string` |
| `GetAllInitiativeDecisions()` | `map[string][]InitiativeDecision` |
| `GetAllInitiativeTaskRefs()` | `map[string][]InitiativeTaskRef` (JOINed with tasks) |
| `GetAllInitiativeDependencies()` | `map[string][]string` |
| `GetAllInitiativeDependents()` | `map[string][]string` |
