-- Global database schema: PostgreSQL version
-- Stores projects registry, cost tracking, and templates

-- Projects registry
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT UNIQUE NOT NULL,
    language TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Cost tracking across all projects
-- No foreign key on project_id to allow orphan cost entries
CREATE TABLE IF NOT EXISTS cost_log (
    id SERIAL PRIMARY KEY,
    project_id TEXT,
    task_id TEXT,
    phase TEXT,
    cost_usd DECIMAL(10, 6),
    input_tokens INTEGER,
    output_tokens INTEGER,
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cost_log_project ON cost_log(project_id);
CREATE INDEX IF NOT EXISTS idx_cost_log_timestamp ON cost_log(timestamp);

-- User-defined templates
CREATE TABLE IF NOT EXISTS templates (
    name TEXT PRIMARY KEY,
    weight TEXT,
    phases JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
