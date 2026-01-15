-- Pure SQL storage: Full task/state/plan/spec storage in database
-- Enables database-only mode without YAML files

-- ============================================================================
-- TASK TABLE EXPANSIONS
-- ============================================================================

-- PR info (from task.yaml pr: section)
ALTER TABLE tasks ADD COLUMN pr_url TEXT;
ALTER TABLE tasks ADD COLUMN pr_number INTEGER;
ALTER TABLE tasks ADD COLUMN pr_status TEXT;
ALTER TABLE tasks ADD COLUMN pr_checks_status TEXT;
ALTER TABLE tasks ADD COLUMN pr_mergeable INTEGER DEFAULT 1;
ALTER TABLE tasks ADD COLUMN pr_review_count INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN pr_approval_count INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN pr_merged INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN pr_merged_at TEXT;
ALTER TABLE tasks ADD COLUMN pr_merge_commit_sha TEXT;
ALTER TABLE tasks ADD COLUMN pr_target_branch TEXT;
ALTER TABLE tasks ADD COLUMN pr_last_checked_at TEXT;

-- Testing requirements (JSON: {"unit": true, "e2e": false, "visual": false})
ALTER TABLE tasks ADD COLUMN testing_requirements TEXT;

-- UI testing flag
ALTER TABLE tasks ADD COLUMN requires_ui_testing INTEGER DEFAULT 0;

-- Metadata tags (JSON array: ["auth", "feature"])
ALTER TABLE tasks ADD COLUMN tags TEXT;

-- Initiative link
ALTER TABLE tasks ADD COLUMN initiative_id TEXT;

-- Execution tracking (from state.yaml execution section)
ALTER TABLE tasks ADD COLUMN executor_pid INTEGER;
ALTER TABLE tasks ADD COLUMN executor_hostname TEXT;
ALTER TABLE tasks ADD COLUMN executor_started_at TEXT;
ALTER TABLE tasks ADD COLUMN last_heartbeat TEXT;

-- Session tracking (from state.yaml session section)
ALTER TABLE tasks ADD COLUMN session_id TEXT;
ALTER TABLE tasks ADD COLUMN session_model TEXT;
ALTER TABLE tasks ADD COLUMN session_status TEXT;
ALTER TABLE tasks ADD COLUMN session_created_at TEXT;
ALTER TABLE tasks ADD COLUMN session_last_activity TEXT;
ALTER TABLE tasks ADD COLUMN session_turn_count INTEGER DEFAULT 0;

-- Token tracking aggregates
ALTER TABLE tasks ADD COLUMN input_tokens INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN output_tokens INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN cache_creation_tokens INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN cache_read_tokens INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN total_tokens INTEGER DEFAULT 0;

-- Retry context (JSON with from_phase, to_phase, reason, attempt, etc.)
ALTER TABLE tasks ADD COLUMN retry_context TEXT;

-- State status (separate from task status: pending, running, completed, failed, paused, interrupted, skipped)
ALTER TABLE tasks ADD COLUMN state_status TEXT DEFAULT 'pending';

-- Created by user/source
ALTER TABLE tasks ADD COLUMN created_by TEXT;
ALTER TABLE tasks ADD COLUMN metadata_source TEXT;

-- Task metadata (JSON object: {"resolved": "true", "resolution_message": "...", ...})
ALTER TABLE tasks ADD COLUMN metadata TEXT;

-- Indexes for new columns
CREATE INDEX IF NOT EXISTS idx_tasks_pr_status ON tasks(pr_status);
CREATE INDEX IF NOT EXISTS idx_tasks_initiative ON tasks(initiative_id);
CREATE INDEX IF NOT EXISTS idx_tasks_executor ON tasks(executor_pid, executor_hostname);

-- ============================================================================
-- PHASE TABLE EXPANSIONS
-- ============================================================================

-- Additional phase tracking
ALTER TABLE phases ADD COLUMN artifacts TEXT;  -- JSON array of artifact paths
ALTER TABLE phases ADD COLUMN commit_sha TEXT;
ALTER TABLE phases ADD COLUMN cache_creation_tokens INTEGER DEFAULT 0;
ALTER TABLE phases ADD COLUMN cache_read_tokens INTEGER DEFAULT 0;
ALTER TABLE phases ADD COLUMN total_tokens INTEGER DEFAULT 0;
ALTER TABLE phases ADD COLUMN interrupted_at TEXT;
ALTER TABLE phases ADD COLUMN skip_reason TEXT;

-- ============================================================================
-- NEW TABLES
-- ============================================================================

-- Plans table (from plan.yaml)
CREATE TABLE IF NOT EXISTS plans (
    task_id TEXT PRIMARY KEY,
    version INTEGER NOT NULL DEFAULT 1,
    weight TEXT NOT NULL,
    description TEXT,
    phases TEXT NOT NULL,  -- JSON array of phase definitions
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Specs table (from spec.md)
CREATE TABLE IF NOT EXISTS specs (
    task_id TEXT PRIMARY KEY,
    content TEXT NOT NULL,
    content_hash TEXT,
    source TEXT,  -- 'file', 'db', 'generated', 'migrated'
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- FTS for spec search (matches transcripts_fts pattern)
CREATE VIRTUAL TABLE IF NOT EXISTS specs_fts USING fts5(
    content,
    task_id UNINDEXED,
    content=specs,
    content_rowid=rowid
);

-- Triggers to keep specs FTS in sync
CREATE TRIGGER IF NOT EXISTS specs_ai AFTER INSERT ON specs BEGIN
    INSERT INTO specs_fts(rowid, content, task_id)
    VALUES (NEW.rowid, NEW.content, NEW.task_id);
END;

CREATE TRIGGER IF NOT EXISTS specs_ad AFTER DELETE ON specs BEGIN
    INSERT INTO specs_fts(specs_fts, rowid, content, task_id)
    VALUES('delete', OLD.rowid, OLD.content, OLD.task_id);
END;

CREATE TRIGGER IF NOT EXISTS specs_au AFTER UPDATE ON specs BEGIN
    INSERT INTO specs_fts(specs_fts, rowid, content, task_id)
    VALUES('delete', OLD.rowid, OLD.content, OLD.task_id);
    INSERT INTO specs_fts(rowid, content, task_id)
    VALUES (NEW.rowid, NEW.content, NEW.task_id);
END;

-- Gate decisions table (from state.yaml gates array)
CREATE TABLE IF NOT EXISTS gate_decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    phase TEXT NOT NULL,
    gate_type TEXT NOT NULL,  -- 'auto', 'ai', 'human', 'skip'
    approved INTEGER NOT NULL,
    reason TEXT,
    decided_by TEXT,
    decided_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_gate_decisions_task ON gate_decisions(task_id);
CREATE INDEX IF NOT EXISTS idx_gate_decisions_phase ON gate_decisions(task_id, phase);

-- Task attachments table (from .orc/tasks/{id}/attachments/)
CREATE TABLE IF NOT EXISTS task_attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT,
    size_bytes INTEGER,
    data BLOB,  -- Store file content directly in DB
    is_image INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    UNIQUE(task_id, filename)
);

CREATE INDEX IF NOT EXISTS idx_attachments_task ON task_attachments(task_id);

-- Sync state table (for CR-SQLite P2P sync tracking)
CREATE TABLE IF NOT EXISTS sync_state (
    id INTEGER PRIMARY KEY CHECK (id = 1),  -- Single row
    site_id TEXT NOT NULL,  -- Unique identifier for this instance
    last_sync_version INTEGER DEFAULT 0,
    last_sync_at TEXT,
    sync_enabled INTEGER DEFAULT 0,
    sync_mode TEXT DEFAULT 'none',  -- 'none', 'folder', 'http'
    sync_endpoint TEXT,  -- Folder path or HTTP URL
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Initialize sync state with random site_id
INSERT OR IGNORE INTO sync_state (id, site_id, sync_enabled)
VALUES (1, lower(hex(randomblob(16))), 0);
