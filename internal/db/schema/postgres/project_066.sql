-- Migration 066: Recommendation source thread and promotion provenance
--
-- Adds source-thread context and promotion provenance fields to
-- existing recommendation records.

ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS source_thread_id TEXT REFERENCES threads(id) ON DELETE CASCADE;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS promoted_to_type TEXT;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS promoted_to_id TEXT;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS promoted_by TEXT;
ALTER TABLE recommendations ADD COLUMN IF NOT EXISTS promoted_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS idx_recommendations_source_thread ON recommendations(source_thread_id);
