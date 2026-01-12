-- Add validation tracking to knowledge queue
ALTER TABLE knowledge_queue ADD COLUMN validated_at TEXT;
ALTER TABLE knowledge_queue ADD COLUMN validated_by TEXT;

-- Index for finding stale entries
CREATE INDEX IF NOT EXISTS idx_knowledge_queue_validated ON knowledge_queue(validated_at);
