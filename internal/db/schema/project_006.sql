-- Knowledge queue for pending pattern/gotcha/decision entries
CREATE TABLE IF NOT EXISTS knowledge_queue (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,               -- pattern, gotcha, decision
    name TEXT NOT NULL,               -- Short name/title
    description TEXT NOT NULL,        -- Full description/rationale
    scope TEXT DEFAULT 'project',     -- project, global
    source_task TEXT,                 -- Task ID that generated this
    status TEXT DEFAULT 'pending',    -- pending, approved, rejected
    proposed_by TEXT,
    proposed_at TEXT DEFAULT (datetime('now')),
    approved_by TEXT,
    approved_at TEXT,
    rejected_reason TEXT
);

CREATE INDEX IF NOT EXISTS idx_knowledge_queue_status ON knowledge_queue(status);
CREATE INDEX IF NOT EXISTS idx_knowledge_queue_type ON knowledge_queue(type);
CREATE INDEX IF NOT EXISTS idx_knowledge_queue_source ON knowledge_queue(source_task);
