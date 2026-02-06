-- JSONL-based transcript system
-- Replaces the simple transcript table with a richer structure that stores
-- per-message data from Claude Code's JSONL files.
--
-- This migration:
-- 1. Drops old transcripts and rebuilds with new schema
-- 2. Adds todo_snapshots for progress tracking
-- 3. Adds usage_metrics for analytics

-- Drop old transcripts table
DROP TABLE IF EXISTS transcripts;

-- New transcripts table with per-message data from JSONL
CREATE TABLE IF NOT EXISTS transcripts (
    id SERIAL PRIMARY KEY,
    task_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    session_id TEXT NOT NULL,            -- Claude session UUID
    message_uuid TEXT NOT NULL UNIQUE,   -- Individual message UUID
    parent_uuid TEXT,                    -- Links to parent message (for threading)
    type TEXT NOT NULL,                  -- 'user', 'assistant', 'queue-operation'
    role TEXT,                           -- from message.role
    content TEXT NOT NULL,               -- Full content JSON (preserves structure)
    model TEXT,                          -- Model used (from assistant messages)

    -- Per-message token tracking (from message.usage)
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,

    -- Tool information
    tool_calls TEXT,                     -- JSON array of tool_use blocks from content
    tool_results TEXT,                   -- JSON of toolUseResult metadata (durations, etc.)

    timestamp INTEGER NOT NULL,          -- Unix timestamp (ms)
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_transcripts_task ON transcripts(task_id);
CREATE INDEX IF NOT EXISTS idx_transcripts_session ON transcripts(session_id);
CREATE INDEX IF NOT EXISTS idx_transcripts_timestamp ON transcripts(task_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_transcripts_phase ON transcripts(task_id, phase);

-- Aggregation view for token totals per task/phase
CREATE VIEW IF NOT EXISTS task_token_usage AS
SELECT
    task_id,
    phase,
    SUM(input_tokens) as total_input,
    SUM(output_tokens) as total_output,
    SUM(cache_creation_tokens) as total_cache_creation,
    SUM(cache_read_tokens) as total_cache_read,
    COUNT(*) as message_count
FROM transcripts
WHERE type = 'assistant'
GROUP BY task_id, phase;

-- Todo snapshots for progress tracking during execution
-- Stores TodoWrite tool call results to show agent progress
CREATE TABLE IF NOT EXISTS todo_snapshots (
    id SERIAL PRIMARY KEY,
    task_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    message_uuid TEXT,                   -- Links to transcript that triggered this
    items TEXT NOT NULL,                 -- JSON array of TodoItem
    timestamp INTEGER NOT NULL,          -- Unix timestamp (ms)
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_todo_snapshots_task ON todo_snapshots(task_id);
CREATE INDEX IF NOT EXISTS idx_todo_snapshots_timestamp ON todo_snapshots(task_id, timestamp DESC);

-- Usage metrics for analytics (denormalized for fast queries)
-- Aggregated per-phase data for cost/token analysis by model, project, time
CREATE TABLE IF NOT EXISTS usage_metrics (
    id SERIAL PRIMARY KEY,
    task_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    model TEXT NOT NULL,
    project_path TEXT NOT NULL,          -- Normalized project path (for multi-project analytics)

    -- Token counts
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,

    -- Cost (calculated from tokens + model pricing)
    cost_usd REAL DEFAULT 0,

    -- Timing
    duration_ms INTEGER DEFAULT 0,       -- Phase duration
    timestamp INTEGER NOT NULL,          -- Unix timestamp (ms) when recorded

    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_metrics_model ON usage_metrics(model);
CREATE INDEX IF NOT EXISTS idx_metrics_project ON usage_metrics(project_path);
CREATE INDEX IF NOT EXISTS idx_metrics_timestamp ON usage_metrics(timestamp);
CREATE INDEX IF NOT EXISTS idx_metrics_task ON usage_metrics(task_id);

-- Pre-aggregated views for dashboard queries
CREATE VIEW IF NOT EXISTS metrics_by_model AS
SELECT
    model,
    (to_timestamp(timestamp / 1000.0) AT TIME ZONE 'UTC')::date as date,
    SUM(input_tokens) as total_input,
    SUM(output_tokens) as total_output,
    SUM(cost_usd) as total_cost,
    COUNT(DISTINCT task_id) as task_count
FROM usage_metrics
GROUP BY model, date;

CREATE VIEW IF NOT EXISTS metrics_by_project AS
SELECT
    project_path,
    (to_timestamp(timestamp / 1000.0) AT TIME ZONE 'UTC')::date as date,
    SUM(input_tokens) as total_input,
    SUM(output_tokens) as total_output,
    SUM(cost_usd) as total_cost,
    COUNT(DISTINCT task_id) as task_count
FROM usage_metrics
GROUP BY project_path, date;

CREATE VIEW IF NOT EXISTS metrics_daily AS
SELECT
    (to_timestamp(timestamp / 1000.0) AT TIME ZONE 'UTC')::date as date,
    SUM(input_tokens) as total_input,
    SUM(output_tokens) as total_output,
    SUM(cost_usd) as total_cost,
    COUNT(DISTINCT task_id) as task_count,
    COUNT(DISTINCT model) as models_used
FROM usage_metrics
GROUP BY date;
