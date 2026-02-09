-- Add lint/security output capture to bench_runs
ALTER TABLE bench_runs ADD COLUMN lint_output TEXT DEFAULT '';
ALTER TABLE bench_runs ADD COLUMN security_output TEXT DEFAULT '';
