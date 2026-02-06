-- Transcript pagination support
-- Adds composite index for efficient cursor-based pagination

-- Composite index for cursor-based pagination (task_id + id as cursor)
-- Supports queries: WHERE task_id = $1 AND id > $2 ORDER BY id
CREATE INDEX IF NOT EXISTS idx_transcripts_task_id ON transcripts(task_id, id);
