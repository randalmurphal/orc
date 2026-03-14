-- Migration 070: Allow recommendation provenance without task/run IDs
--
-- Recommendation drafts can now promote from thread-only discussions or from
-- task-linked threads that intentionally have no workflow run yet. That means
-- source_task_id and source_run_id must allow NULL.

ALTER TABLE recommendations ALTER COLUMN source_task_id DROP NOT NULL;
ALTER TABLE recommendations ALTER COLUMN source_run_id DROP NOT NULL;

UPDATE recommendations
SET source_task_id = NULL
WHERE source_task_id = '';

UPDATE recommendations
SET source_run_id = NULL
WHERE source_run_id = '';
