-- Migration 002: Add eval metrics to runs, schema integrity improvements
--
-- 1. Add evaluation metric columns to bench_runs (eval is run-level, not per-phase)
-- 2. Add unique constraint on bench_judgments to prevent duplicate evaluations
-- 3. Add cross-phase index for efficient phase-level reporting

-- Evaluation metrics on runs (populated after evaluator.RunAll)
ALTER TABLE bench_runs ADD COLUMN test_pass BOOLEAN DEFAULT FALSE;
ALTER TABLE bench_runs ADD COLUMN test_count INTEGER DEFAULT 0;
ALTER TABLE bench_runs ADD COLUMN regression_count INTEGER DEFAULT 0;
ALTER TABLE bench_runs ADD COLUMN lint_warnings INTEGER DEFAULT 0;
ALTER TABLE bench_runs ADD COLUMN build_success BOOLEAN DEFAULT FALSE;
ALTER TABLE bench_runs ADD COLUMN security_findings INTEGER DEFAULT 0;

-- Prevent duplicate judge evaluations for the same phase by the same judge
CREATE UNIQUE INDEX IF NOT EXISTS idx_judgments_unique
    ON bench_judgments(run_id, phase_id, judge_model, judge_provider);

-- Cross-phase query index for efficient leaderboard/report generation
CREATE INDEX IF NOT EXISTS idx_phase_results_phase_model
    ON bench_phase_results(phase_id, was_frozen, provider, model);
