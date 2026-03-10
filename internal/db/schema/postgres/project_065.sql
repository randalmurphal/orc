-- Migration 065: Recommendation inbox persistence
--
-- Adds project-scoped recommendations that require explicit human acceptance
-- before they become real backlog work.

CREATE TABLE IF NOT EXISTS recommendations (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    proposed_action TEXT NOT NULL,
    evidence TEXT NOT NULL,
    source_task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    source_run_id TEXT NOT NULL REFERENCES workflow_runs(id) ON DELETE CASCADE,
    dedupe_key TEXT NOT NULL UNIQUE,
    decided_by TEXT,
    decided_at TIMESTAMPTZ,
    decision_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recommendations_status ON recommendations(status);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_task ON recommendations(source_task_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_run ON recommendations(source_run_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_created_at ON recommendations(created_at DESC);

CREATE TABLE IF NOT EXISTS recommendation_history (
    id BIGSERIAL PRIMARY KEY,
    recommendation_id TEXT NOT NULL REFERENCES recommendations(id) ON DELETE CASCADE,
    from_status TEXT,
    to_status TEXT NOT NULL,
    decided_by TEXT,
    decision_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_recommendation_history_recommendation_id
    ON recommendation_history(recommendation_id, created_at DESC);
