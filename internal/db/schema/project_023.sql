-- Event log for persisting executor events
-- Stores task lifecycle events, phase transitions, errors, and activity states
-- Used by the Timeline view for historical event queries

CREATE TABLE IF NOT EXISTS event_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT NOT NULL,
    phase TEXT,                           -- NULL for task-level events
    iteration INTEGER,                    -- NULL for task-level events
    event_type TEXT NOT NULL,             -- state, transcript, phase, error, complete, tokens, activity, etc.
    data TEXT,                            -- JSON payload
    source TEXT DEFAULT 'executor',       -- executor, api, cli, etc.
    created_at TEXT DEFAULT (datetime('now')),
    duration_ms INTEGER,                  -- Optional duration for timed events
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

-- Index for task-specific event queries
CREATE INDEX IF NOT EXISTS idx_event_log_task ON event_log(task_id);

-- Index for task timeline ordered by time (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_event_log_task_created ON event_log(task_id, created_at DESC);

-- Index for global timeline queries
CREATE INDEX IF NOT EXISTS idx_event_log_created ON event_log(created_at DESC);

-- Index for filtering by event type
CREATE INDEX IF NOT EXISTS idx_event_log_event_type ON event_log(event_type);

-- Composite index for complex timeline queries (by task, type, and time range)
CREATE INDEX IF NOT EXISTS idx_event_log_timeline ON event_log(task_id, event_type, created_at DESC);
