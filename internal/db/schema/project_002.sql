-- Project database schema extension for initiatives
-- Adds initiative tables and task dependencies

-- Initiatives (grouping of related tasks)
CREATE TABLE IF NOT EXISTS initiatives (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',  -- draft, active, completed, archived
    owner_initials TEXT,
    owner_display_name TEXT,
    owner_email TEXT,
    vision TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_initiatives_status ON initiatives(status);

-- Initiative decisions (recorded decisions with rationale)
CREATE TABLE IF NOT EXISTS initiative_decisions (
    id TEXT PRIMARY KEY,
    initiative_id TEXT NOT NULL,
    decision TEXT NOT NULL,
    rationale TEXT,
    decided_by TEXT,
    decided_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_initiative_decisions_init ON initiative_decisions(initiative_id);

-- Initiative tasks (linking tasks to initiatives)
CREATE TABLE IF NOT EXISTS initiative_tasks (
    initiative_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    sequence INTEGER DEFAULT 0,
    PRIMARY KEY (initiative_id, task_id),
    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_initiative_tasks_init ON initiative_tasks(initiative_id);

-- Task dependencies (which tasks depend on which)
CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id TEXT NOT NULL,
    depends_on TEXT NOT NULL,
    PRIMARY KEY (task_id, depends_on)
);

CREATE INDEX IF NOT EXISTS idx_task_deps_task ON task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_task_deps_dep ON task_dependencies(depends_on);

-- Add initiative_id to tasks table
-- Note: SQLite doesn't support ADD COLUMN IF NOT EXISTS, so we use a workaround
-- The column will be added if it doesn't exist, ignored otherwise
-- This requires a check in Go code
