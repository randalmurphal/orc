-- Migration 047: Add branch control fields to tasks
-- Supports task-level branch naming and PR settings overrides.

-- Custom branch name (overrides auto-generated from task ID)
ALTER TABLE tasks ADD COLUMN branch_name TEXT;

-- PR settings overrides
ALTER TABLE tasks ADD COLUMN pr_draft INTEGER;  -- 0/1/NULL (NULL = use default)
ALTER TABLE tasks ADD COLUMN pr_labels TEXT;    -- JSON array
ALTER TABLE tasks ADD COLUMN pr_reviewers TEXT; -- JSON array
ALTER TABLE tasks ADD COLUMN pr_labels_set INTEGER DEFAULT 0;    -- True if pr_labels explicitly set
ALTER TABLE tasks ADD COLUMN pr_reviewers_set INTEGER DEFAULT 0; -- True if pr_reviewers explicitly set
