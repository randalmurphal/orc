-- Migration 067: Persisted attention signals for operator control-plane views
--
-- Stores project-scoped attention signals as first-class records instead of
-- recomputing them from task scans.

CREATE TABLE IF NOT EXISTS attention_signals (
    id TEXT PRIMARY KEY,
    kind TEXT NOT NULL,
    status TEXT NOT NULL,
    reference_type TEXT NOT NULL,
    reference_id TEXT NOT NULL,
    title TEXT NOT NULL,
    summary TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP,
    resolved_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_attention_signals_created
    ON attention_signals(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_attention_signals_reference
    ON attention_signals(reference_type, reference_id);

CREATE INDEX IF NOT EXISTS idx_attention_signals_active
    ON attention_signals(status, resolved_at, updated_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS idx_attention_signals_active_unique
    ON attention_signals(kind, reference_type, reference_id)
    WHERE resolved_at IS NULL;
