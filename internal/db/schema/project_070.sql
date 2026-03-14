-- orc:disable_fk
-- Migration 070: Allow recommendation provenance without task/run IDs
--
-- Recommendation drafts can now promote from thread-only discussions or from
-- task-linked threads that intentionally have no workflow run yet. That means
-- source_task_id and source_run_id must allow NULL.

CREATE TABLE recommendations_new (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    proposed_action TEXT NOT NULL,
    evidence TEXT NOT NULL,
    source_task_id TEXT,
    source_run_id TEXT,
    dedupe_key TEXT NOT NULL UNIQUE,
    decided_by TEXT,
    decided_at TEXT,
    decision_reason TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    source_thread_id TEXT REFERENCES threads(id) ON DELETE CASCADE,
    promoted_to_type TEXT,
    promoted_to_id TEXT,
    promoted_by TEXT,
    promoted_at TEXT,
    FOREIGN KEY (source_task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (source_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

INSERT INTO recommendations_new (
    id, kind, status, title, summary, proposed_action, evidence,
    source_task_id, source_run_id, dedupe_key, decided_by, decided_at,
    decision_reason, created_at, updated_at, source_thread_id,
    promoted_to_type, promoted_to_id, promoted_by, promoted_at
)
SELECT
    id, kind, status, title, summary, proposed_action, evidence,
    NULLIF(source_task_id, ''),
    NULLIF(source_run_id, ''),
    dedupe_key, decided_by, decided_at, decision_reason,
    created_at, updated_at, source_thread_id,
    promoted_to_type, promoted_to_id, promoted_by, promoted_at
FROM recommendations;

DROP TABLE recommendations;
ALTER TABLE recommendations_new RENAME TO recommendations;

CREATE INDEX IF NOT EXISTS idx_recommendations_status ON recommendations(status);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_task ON recommendations(source_task_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_run ON recommendations(source_run_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_created_at ON recommendations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_thread ON recommendations(source_thread_id);
