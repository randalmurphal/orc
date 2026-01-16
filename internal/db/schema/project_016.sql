-- Migration 016: Remove StatusFinished
-- Consolidate 'finished' status into 'completed' as the single terminal success state.
-- Tasks now only reach 'completed' when all phases AND sync/PR/merge succeed.

-- Migrate existing 'finished' tasks to 'completed'
UPDATE tasks SET status = 'completed' WHERE status = 'finished';
