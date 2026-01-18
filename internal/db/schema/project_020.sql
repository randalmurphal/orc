-- QA results for persisting QA phase output
-- Stores structured QA session results for reporting and task state visibility

CREATE TABLE IF NOT EXISTS qa_results (
    task_id TEXT NOT NULL PRIMARY KEY,
    status TEXT NOT NULL,
    summary TEXT NOT NULL,
    tests_written_json TEXT,
    tests_run_json TEXT,
    coverage_json TEXT,
    documentation_json TEXT,
    issues_json TEXT,
    recommendation TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_qa_results_status ON qa_results(status);
