-- Migration 071: Project-scoped artifact index for accepted recommendations,
-- initiative decisions, promoted drafts, and high-signal task outcomes.

CREATE TABLE IF NOT EXISTS artifact_index (
    id BIGSERIAL PRIMARY KEY,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    dedupe_key TEXT,
    initiative_id TEXT REFERENCES initiatives(id) ON DELETE SET NULL,
    source_task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    source_run_id TEXT REFERENCES workflow_runs(id) ON DELETE SET NULL,
    source_thread_id TEXT REFERENCES threads(id) ON DELETE SET NULL,
    search_vector tsvector,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

UPDATE artifact_index
SET search_vector = to_tsvector('english', COALESCE(title, '') || ' ' || COALESCE(content, ''))
WHERE search_vector IS NULL;

CREATE INDEX IF NOT EXISTS idx_artifact_index_kind ON artifact_index(kind);
CREATE INDEX IF NOT EXISTS idx_artifact_index_initiative ON artifact_index(initiative_id);
CREATE INDEX IF NOT EXISTS idx_artifact_index_source_task ON artifact_index(source_task_id);
CREATE INDEX IF NOT EXISTS idx_artifact_index_source_run ON artifact_index(source_run_id);
CREATE INDEX IF NOT EXISTS idx_artifact_index_source_thread ON artifact_index(source_thread_id);
CREATE INDEX IF NOT EXISTS idx_artifact_index_dedupe_key ON artifact_index(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_artifact_index_created_at ON artifact_index(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifact_index_deleted_at ON artifact_index(deleted_at);
CREATE INDEX IF NOT EXISTS idx_artifact_index_fts ON artifact_index USING GIN (search_vector);

CREATE OR REPLACE FUNCTION artifact_index_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.title, '') || ' ' || COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_artifact_index_search_vector ON artifact_index;
CREATE TRIGGER trg_artifact_index_search_vector
    BEFORE INSERT OR UPDATE OF title, content ON artifact_index
    FOR EACH ROW
    EXECUTE FUNCTION artifact_index_search_vector_update();
