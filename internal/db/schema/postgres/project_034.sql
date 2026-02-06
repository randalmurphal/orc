-- Migration 034: Rename perspective column to agent_id in review_findings
-- This reflects the shift from hardcoded perspectives to database-backed agents

-- PostgreSQL supports ALTER TABLE RENAME COLUMN directly
ALTER TABLE review_findings RENAME COLUMN perspective TO agent_id;

-- Recreate index
CREATE INDEX IF NOT EXISTS idx_review_findings_task ON review_findings(task_id);
