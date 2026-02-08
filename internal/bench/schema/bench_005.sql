-- Add test/build output capture to bench_runs
ALTER TABLE bench_runs ADD COLUMN test_output TEXT DEFAULT '';
ALTER TABLE bench_runs ADD COLUMN build_output TEXT DEFAULT '';
