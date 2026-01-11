-- Project database schema extension for initiatives and dependencies
-- PostgreSQL version

-- Initiatives (grouping of related tasks)
CREATE TABLE IF NOT EXISTS initiatives (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft',
    owner_initials TEXT,
    owner_display_name TEXT,
    owner_email TEXT,
    vision TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_initiatives_status ON initiatives(status);

-- Initiative decisions (recorded decisions with rationale)
CREATE TABLE IF NOT EXISTS initiative_decisions (
    id TEXT PRIMARY KEY,
    initiative_id TEXT NOT NULL REFERENCES initiatives(id) ON DELETE CASCADE,
    decision TEXT NOT NULL,
    rationale TEXT,
    decided_by TEXT,
    decided_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_initiative_decisions_init ON initiative_decisions(initiative_id);

-- Initiative tasks (linking tasks to initiatives)
CREATE TABLE IF NOT EXISTS initiative_tasks (
    initiative_id TEXT NOT NULL REFERENCES initiatives(id) ON DELETE CASCADE,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    sequence INTEGER DEFAULT 0,
    PRIMARY KEY (initiative_id, task_id)
);

CREATE INDEX IF NOT EXISTS idx_initiative_tasks_init ON initiative_tasks(initiative_id);

-- Task dependencies (which tasks depend on which)
CREATE TABLE IF NOT EXISTS task_dependencies (
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    depends_on TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, depends_on)
);

CREATE INDEX IF NOT EXISTS idx_task_deps_task ON task_dependencies(task_id);
CREATE INDEX IF NOT EXISTS idx_task_deps_dep ON task_dependencies(depends_on);

-- Review comments for code review UI
CREATE TABLE IF NOT EXISTS review_comments (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    review_round INTEGER NOT NULL,
    file_path TEXT,
    line_number INTEGER,
    content TEXT NOT NULL,
    severity TEXT DEFAULT 'suggestion',
    status TEXT DEFAULT 'open',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by TEXT
);

CREATE INDEX IF NOT EXISTS idx_review_comments_task ON review_comments(task_id);
CREATE INDEX IF NOT EXISTS idx_review_comments_status ON review_comments(status);
