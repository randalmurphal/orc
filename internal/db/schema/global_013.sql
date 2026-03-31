-- orc:disable_fk
-- Migration 013: Remove dead workflow columns
--
-- workflow_type and default_max_iterations no longer affect runtime behavior.
-- Recreate workflows without those columns so new and migrated DBs match the
-- current workflow model.

DROP INDEX IF EXISTS idx_workflows_type;

CREATE TABLE workflows_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    default_model TEXT,
    default_thinking BOOLEAN DEFAULT FALSE,
    is_builtin BOOLEAN DEFAULT FALSE,
    based_on TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    triggers TEXT,
    completion_action TEXT DEFAULT '',
    target_branch TEXT DEFAULT '',
    default_provider TEXT DEFAULT '',
    FOREIGN KEY (based_on) REFERENCES workflows(id) ON DELETE SET NULL
);

INSERT INTO workflows_new (
    id,
    name,
    description,
    default_model,
    default_thinking,
    is_builtin,
    based_on,
    created_at,
    updated_at,
    triggers,
    completion_action,
    target_branch,
    default_provider
)
SELECT
    id,
    name,
    description,
    default_model,
    default_thinking,
    is_builtin,
    based_on,
    created_at,
    updated_at,
    triggers,
    completion_action,
    target_branch,
    default_provider
FROM workflows;

DROP TABLE workflows;
ALTER TABLE workflows_new RENAME TO workflows;

CREATE INDEX IF NOT EXISTS idx_workflows_builtin ON workflows(is_builtin);
