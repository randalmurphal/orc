-- Migration 001: Initial benchmark schema
--
-- Dedicated database for benchmarking model configurations across workflow phases.
-- Lives at ~/.orc/bench/bench.db, separate from GlobalDB and ProjectDB.

-- Test project definitions (pinned repos)
CREATE TABLE bench_projects (
    id TEXT PRIMARY KEY,
    repo_url TEXT NOT NULL,
    commit_hash TEXT NOT NULL,
    language TEXT NOT NULL,
    test_cmd TEXT NOT NULL,
    build_cmd TEXT DEFAULT '',
    lint_cmd TEXT DEFAULT '',
    security_cmd TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Curated tasks (SWE-bench style from real PRs)
CREATE TABLE bench_tasks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES bench_projects(id),
    tier TEXT NOT NULL CHECK(tier IN ('trivial', 'small', 'medium', 'large')),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    pre_fix_commit TEXT NOT NULL,
    reference_pr_url TEXT DEFAULT '',
    reference_diff TEXT DEFAULT '',
    fail_to_pass_tests TEXT DEFAULT '[]',
    pass_to_pass_tests TEXT DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX idx_bench_tasks_project ON bench_tasks(project_id);
CREATE INDEX idx_bench_tasks_tier ON bench_tasks(tier);

-- Model configuration variants (config-driven, no code changes to add models)
CREATE TABLE bench_variants (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    base_workflow TEXT NOT NULL,
    phase_overrides TEXT NOT NULL DEFAULT '{}',
    is_baseline BOOLEAN DEFAULT FALSE,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Execution records
CREATE TABLE bench_runs (
    id TEXT PRIMARY KEY,
    variant_id TEXT NOT NULL REFERENCES bench_variants(id),
    task_id TEXT NOT NULL REFERENCES bench_tasks(id),
    trial_number INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'pass', 'fail', 'error')),
    started_at TEXT,
    completed_at TEXT,
    error_message TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_bench_runs_unique ON bench_runs(variant_id, task_id, trial_number);
CREATE INDEX idx_bench_runs_variant ON bench_runs(variant_id);
CREATE INDEX idx_bench_runs_task ON bench_runs(task_id);
CREATE INDEX idx_bench_runs_status ON bench_runs(status);

-- Per-phase metrics within a run
CREATE TABLE bench_phase_results (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES bench_runs(id),
    phase_id TEXT NOT NULL,
    was_frozen BOOLEAN DEFAULT FALSE,
    provider TEXT NOT NULL DEFAULT '',
    model TEXT NOT NULL DEFAULT '',
    reasoning_effort TEXT DEFAULT '',
    thinking_enabled BOOLEAN DEFAULT FALSE,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    reasoning_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    cache_creation_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    test_pass BOOLEAN DEFAULT FALSE,
    test_count INTEGER DEFAULT 0,
    regression_count INTEGER DEFAULT 0,
    lint_warnings INTEGER DEFAULT 0,
    coverage_delta REAL DEFAULT 0,
    security_findings INTEGER DEFAULT 0,
    frozen_output_id TEXT DEFAULT '',
    output_content TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX idx_phase_results_run ON bench_phase_results(run_id);
CREATE INDEX idx_phase_results_phase ON bench_phase_results(phase_id);

-- Cached phase outputs for controlled replay
CREATE TABLE bench_frozen_outputs (
    id TEXT PRIMARY KEY,
    task_id TEXT NOT NULL,
    phase_id TEXT NOT NULL,
    variant_id TEXT NOT NULL,
    trial_number INTEGER NOT NULL DEFAULT 1,
    output_content TEXT NOT NULL,
    output_var_name TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_frozen_unique ON bench_frozen_outputs(task_id, phase_id, variant_id, trial_number);

-- Cross-model judge evaluations
CREATE TABLE bench_judgments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT NOT NULL REFERENCES bench_runs(id),
    phase_id TEXT NOT NULL,
    judge_model TEXT NOT NULL,
    judge_provider TEXT NOT NULL,
    scores TEXT NOT NULL DEFAULT '{}',
    reasoning TEXT DEFAULT '',
    presentation_order INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX idx_judgments_run ON bench_judgments(run_id);
CREATE INDEX idx_judgments_phase ON bench_judgments(phase_id);
