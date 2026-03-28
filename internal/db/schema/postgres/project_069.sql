-- Migration 069: Enforce single canonical task/initiative thread links
--
-- Cleans up contradictory typed associations created before task/initiative
-- links were treated as single-target relationships, then adds uniqueness
-- guards so each thread can link to at most one task and one initiative.

DELETE FROM thread_links AS duplicate
USING thread_links AS canonical
WHERE duplicate.link_type IN ('task', 'initiative')
  AND canonical.thread_id = duplicate.thread_id
  AND canonical.link_type = duplicate.link_type
  AND (
    canonical.created_at < duplicate.created_at OR
    (canonical.created_at = duplicate.created_at AND canonical.id < duplicate.id)
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_thread_links_single_task
    ON thread_links(thread_id)
    WHERE link_type = 'task';

CREATE UNIQUE INDEX IF NOT EXISTS idx_thread_links_single_initiative
    ON thread_links(thread_id)
    WHERE link_type = 'initiative';
