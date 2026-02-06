-- Branch targeting: Add merge status tracking to initiatives
-- Tracks the state of merging the initiative branch to its target branch.

-- ============================================================================
-- INITIATIVES TABLE: Add merge tracking columns
-- ============================================================================

-- merge_status tracks the state of merging the initiative branch
-- Values: '' (none), 'pending', 'in_progress', 'merged', 'failed'
ALTER TABLE initiatives ADD COLUMN merge_status TEXT DEFAULT '';

-- merge_commit is the SHA of the merge commit (set when merge_status = 'merged')
ALTER TABLE initiatives ADD COLUMN merge_commit TEXT;
