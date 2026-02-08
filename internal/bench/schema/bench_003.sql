-- Migration 003: Add test_patch and category to tasks
--
-- test_patch: test-only diff from the reference PR, applied to the worktree
-- AFTER the model finishes (for evaluation only — model never sees these tests).
-- category: what kind of task this is (bug, feature, refactor, etc.)

ALTER TABLE bench_tasks ADD COLUMN test_patch TEXT DEFAULT '';
ALTER TABLE bench_tasks ADD COLUMN category TEXT DEFAULT '';
