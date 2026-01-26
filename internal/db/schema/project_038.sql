-- Migration 038: Simplify phase status to completion-only
-- Phase status now only tracks completion (pending, completed, skipped).
-- Execution state (running, paused, failed, etc.) is tracked at the task level.
-- Use task.status + task.current_phase to determine which phase is active.

-- Map all "in progress" phase statuses to pending (needs to complete)
-- These phases have session_id stored for resume capability
UPDATE phases
SET status = 'pending'
WHERE status IN ('running', 'interrupted', 'paused', 'blocked');

-- Map failed phases to pending (will be retried)
-- The task.status captures that the task itself failed
UPDATE phases
SET status = 'pending'
WHERE status = 'failed';
