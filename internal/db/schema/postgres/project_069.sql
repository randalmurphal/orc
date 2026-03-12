-- Migration 069: Allow thread-only recommendation provenance
--
-- Generic discussion threads can promote recommendation drafts without task/run
-- execution provenance, so those columns must be nullable.

ALTER TABLE recommendations
    ALTER COLUMN source_task_id DROP NOT NULL,
    ALTER COLUMN source_run_id DROP NOT NULL;

ALTER TABLE recommendations
    DROP CONSTRAINT IF EXISTS recommendations_source_task_id_fkey,
    DROP CONSTRAINT IF EXISTS recommendations_source_run_id_fkey;

ALTER TABLE recommendations
    ADD CONSTRAINT recommendations_source_task_id_fkey
        FOREIGN KEY (source_task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    ADD CONSTRAINT recommendations_source_run_id_fkey
        FOREIGN KEY (source_run_id) REFERENCES workflow_runs(id) ON DELETE CASCADE;

UPDATE recommendations
SET source_task_id = NULL
WHERE source_task_id = '';

UPDATE recommendations
SET source_run_id = NULL
WHERE source_run_id = '';
