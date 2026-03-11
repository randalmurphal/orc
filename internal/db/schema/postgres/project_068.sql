-- Migration 068: Thread typed links and persisted discussion drafts
--
-- Adds explicit typed linkage plus persisted recommendation/decision drafts
-- so discussion threads retain context and can promote drafts deliberately.

CREATE TABLE IF NOT EXISTS thread_links (
    id BIGSERIAL PRIMARY KEY,
    thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    link_type TEXT NOT NULL,
    target_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(thread_id, link_type, target_id)
);

CREATE INDEX IF NOT EXISTS idx_thread_links_thread
    ON thread_links(thread_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_thread_links_target
    ON thread_links(link_type, target_id);

CREATE TABLE IF NOT EXISTS thread_recommendation_drafts (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT NOT NULL,
    proposed_action TEXT NOT NULL,
    evidence TEXT NOT NULL,
    dedupe_key TEXT NOT NULL DEFAULT '',
    source_task_id TEXT NOT NULL DEFAULT '',
    source_run_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    promoted_recommendation_id TEXT NOT NULL DEFAULT '',
    promoted_by TEXT NOT NULL DEFAULT '',
    promoted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_thread_recommendation_drafts_thread
    ON thread_recommendation_drafts(thread_id, created_at ASC);

CREATE TABLE IF NOT EXISTS thread_decision_drafts (
    id TEXT PRIMARY KEY,
    thread_id TEXT NOT NULL REFERENCES threads(id) ON DELETE CASCADE,
    initiative_id TEXT NOT NULL DEFAULT '',
    decision TEXT NOT NULL,
    rationale TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    promoted_decision_id TEXT NOT NULL DEFAULT '',
    promoted_by TEXT NOT NULL DEFAULT '',
    promoted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_thread_decision_drafts_thread
    ON thread_decision_drafts(thread_id, created_at ASC);
