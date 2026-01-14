-- Initiative dependencies (blocked_by relationships between initiatives)
-- Mirrors the task_dependencies pattern for initiatives

CREATE TABLE IF NOT EXISTS initiative_dependencies (
    initiative_id TEXT NOT NULL,
    depends_on TEXT NOT NULL,
    PRIMARY KEY (initiative_id, depends_on),
    FOREIGN KEY (initiative_id) REFERENCES initiatives(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on) REFERENCES initiatives(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_initiative_deps_init ON initiative_dependencies(initiative_id);
CREATE INDEX IF NOT EXISTS idx_initiative_deps_dep ON initiative_dependencies(depends_on);
