-- Migration 041: Add PostgreSQL full-text search for transcripts
-- Replaces ILIKE fallback with native tsvector/tsquery FTS
-- Adds GIN index for fast search, trigger for auto-indexing new rows

-- Add tsvector column for full-text search
ALTER TABLE transcripts ADD COLUMN IF NOT EXISTS search_vector tsvector;

-- Backfill existing rows with tsvector data
UPDATE transcripts
SET search_vector = to_tsvector('english', COALESCE(content, ''))
WHERE search_vector IS NULL;

-- Create GIN index for fast full-text search
CREATE INDEX IF NOT EXISTS idx_transcripts_fts ON transcripts USING GIN (search_vector);

-- Create trigger function to auto-update search_vector on INSERT/UPDATE
CREATE OR REPLACE FUNCTION transcripts_search_vector_update() RETURNS trigger AS $$
BEGIN
    NEW.search_vector := to_tsvector('english', COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger (drop first for idempotency, then create)
DROP TRIGGER IF EXISTS trg_transcripts_search_vector ON transcripts;
CREATE TRIGGER trg_transcripts_search_vector
    BEFORE INSERT OR UPDATE OF content ON transcripts
    FOR EACH ROW
    EXECUTE FUNCTION transcripts_search_vector_update();
