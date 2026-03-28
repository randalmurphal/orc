-- Migration 071: Project-scoped artifact index for accepted recommendations,
-- initiative decisions, promoted drafts, and high-signal task outcomes.

CREATE TABLE artifact_index (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    dedupe_key TEXT,
    initiative_id TEXT REFERENCES initiatives(id) ON DELETE SET NULL,
    source_task_id TEXT REFERENCES tasks(id) ON DELETE SET NULL,
    source_run_id TEXT REFERENCES workflow_runs(id) ON DELETE SET NULL,
    source_thread_id TEXT REFERENCES threads(id) ON DELETE SET NULL,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now')),
    deleted_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_artifact_index_kind ON artifact_index(kind);
CREATE INDEX IF NOT EXISTS idx_artifact_index_initiative ON artifact_index(initiative_id);
CREATE INDEX IF NOT EXISTS idx_artifact_index_source_task ON artifact_index(source_task_id);
CREATE INDEX IF NOT EXISTS idx_artifact_index_source_run ON artifact_index(source_run_id);
CREATE INDEX IF NOT EXISTS idx_artifact_index_source_thread ON artifact_index(source_thread_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_artifact_index_dedupe_kind ON artifact_index(dedupe_key, kind) WHERE dedupe_key IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_artifact_index_dedupe_key ON artifact_index(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_artifact_index_created_at ON artifact_index(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_artifact_index_deleted_at ON artifact_index(deleted_at);

CREATE VIRTUAL TABLE IF NOT EXISTS artifact_index_fts USING fts5(
    title,
    content,
    kind UNINDEXED,
    dedupe_key UNINDEXED,
    initiative_id UNINDEXED,
    source_task_id UNINDEXED,
    source_run_id UNINDEXED,
    source_thread_id UNINDEXED,
    content=artifact_index,
    content_rowid=id
);

CREATE TRIGGER IF NOT EXISTS artifact_index_ai AFTER INSERT ON artifact_index BEGIN
    INSERT INTO artifact_index_fts(
        rowid, title, content, kind, dedupe_key, initiative_id,
        source_task_id, source_run_id, source_thread_id
    )
    VALUES (
        NEW.id, NEW.title, NEW.content, NEW.kind, NEW.dedupe_key, NEW.initiative_id,
        NEW.source_task_id, NEW.source_run_id, NEW.source_thread_id
    );
END;

CREATE TRIGGER IF NOT EXISTS artifact_index_ad AFTER DELETE ON artifact_index BEGIN
    INSERT INTO artifact_index_fts(
        artifact_index_fts, rowid, title, content, kind, dedupe_key, initiative_id,
        source_task_id, source_run_id, source_thread_id
    )
    VALUES (
        'delete', OLD.id, OLD.title, OLD.content, OLD.kind, OLD.dedupe_key, OLD.initiative_id,
        OLD.source_task_id, OLD.source_run_id, OLD.source_thread_id
    );
END;

CREATE TRIGGER IF NOT EXISTS artifact_index_au AFTER UPDATE ON artifact_index BEGIN
    INSERT INTO artifact_index_fts(
        artifact_index_fts, rowid, title, content, kind, dedupe_key, initiative_id,
        source_task_id, source_run_id, source_thread_id
    )
    VALUES (
        'delete', OLD.id, OLD.title, OLD.content, OLD.kind, OLD.dedupe_key, OLD.initiative_id,
        OLD.source_task_id, OLD.source_run_id, OLD.source_thread_id
    );
    INSERT INTO artifact_index_fts(
        rowid, title, content, kind, dedupe_key, initiative_id,
        source_task_id, source_run_id, source_thread_id
    )
    VALUES (
        NEW.id, NEW.title, NEW.content, NEW.kind, NEW.dedupe_key, NEW.initiative_id,
        NEW.source_task_id, NEW.source_run_id, NEW.source_thread_id
    );
END;
