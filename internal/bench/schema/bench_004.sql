-- Add model_diff to bench_runs for post-mortem inspection
ALTER TABLE bench_runs ADD COLUMN model_diff TEXT DEFAULT '';
