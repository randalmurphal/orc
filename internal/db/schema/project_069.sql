-- orc:disable_fk
-- Migration 069: Allow thread-only recommendation provenance
--
-- Recommendation inbox entries promoted from generic discussion threads do not
-- always have a source task/run pair. Rebuild the recommendations table so
-- source_task_id and source_run_id can be NULL while preserving thread linkage.

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
    source_thread_id TEXT REFERENCES threads(id) ON DELETE CASCADE,
    dedupe_key TEXT NOT NULL UNIQUE,
    decided_by TEXT,
    decided_at TEXT,
    decision_reason TEXT,
    promoted_to_type TEXT,
    promoted_to_id TEXT,
    promoted_by TEXT,
    promoted_at TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (source_task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (source_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE
);

INSERT INTO recommendations_new (
    id, kind, status, title, summary, proposed_action, evidence,
    source_task_id, source_run_id, source_thread_id, dedupe_key,
    decided_by, decided_at, decision_reason,
    promoted_to_type, promoted_to_id, promoted_by, promoted_at,
    created_at, updated_at
)
SELECT
    id, kind, status, title, summary, proposed_action, evidence,
    NULLIF(source_task_id, ''),
    NULLIF(source_run_id, ''),
    source_thread_id,
    dedupe_key,
    decided_by, decided_at, decision_reason,
    promoted_to_type, promoted_to_id, promoted_by, promoted_at,
    created_at, updated_at
FROM recommendations;

DROP TABLE recommendations;

ALTER TABLE recommendations_new RENAME TO recommendations;

CREATE INDEX IF NOT EXISTS idx_recommendations_status ON recommendations(status);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_task ON recommendations(source_task_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_run ON recommendations(source_run_id);
CREATE INDEX IF NOT EXISTS idx_recommendations_created_at ON recommendations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_recommendations_source_thread ON recommendations(source_thread_id);
