-- Global database migration 002: Add model tracking and enhanced schema to cost_log
-- Adds model field, cache tokens, and creates aggregation/budget tables
-- Note: Transaction safety is handled by the migration infrastructure in sqlite.go/postgres.go

-- Add missing columns to cost_log
-- SQLite only allows ADD COLUMN with defaults, so we use safe defaults
ALTER TABLE cost_log ADD COLUMN model TEXT DEFAULT '';
ALTER TABLE cost_log ADD COLUMN iteration INTEGER DEFAULT 0;
ALTER TABLE cost_log ADD COLUMN cache_creation_tokens INTEGER DEFAULT 0;
ALTER TABLE cost_log ADD COLUMN cache_read_tokens INTEGER DEFAULT 0;
ALTER TABLE cost_log ADD COLUMN total_tokens INTEGER DEFAULT 0;
ALTER TABLE cost_log ADD COLUMN initiative_id TEXT DEFAULT '';

-- Better indexes for analytics queries
CREATE INDEX IF NOT EXISTS idx_cost_model ON cost_log(model);
CREATE INDEX IF NOT EXISTS idx_cost_model_timestamp ON cost_log(model, timestamp);
CREATE INDEX IF NOT EXISTS idx_cost_initiative ON cost_log(initiative_id);
CREATE INDEX IF NOT EXISTS idx_cost_project_timestamp ON cost_log(project_id, timestamp);

-- Cost aggregates table for efficient time-series queries
CREATE TABLE IF NOT EXISTS cost_aggregates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT NOT NULL,
    model TEXT DEFAULT '',
    phase TEXT DEFAULT '',
    date TEXT NOT NULL,
    total_cost_usd REAL DEFAULT 0,
    total_input_tokens INTEGER DEFAULT 0,
    total_output_tokens INTEGER DEFAULT 0,
    total_cache_tokens INTEGER DEFAULT 0,
    turn_count INTEGER DEFAULT 0,
    task_count INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(project_id, model, phase, date)
);

CREATE INDEX IF NOT EXISTS idx_cost_agg_project_date ON cost_aggregates(project_id, date);
CREATE INDEX IF NOT EXISTS idx_cost_agg_model_date ON cost_aggregates(model, date);

-- Budget tracking
CREATE TABLE IF NOT EXISTS cost_budgets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id TEXT UNIQUE NOT NULL,
    monthly_limit_usd REAL,
    alert_threshold_percent INTEGER DEFAULT 80,
    current_month TEXT DEFAULT '',
    current_month_spent REAL DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
