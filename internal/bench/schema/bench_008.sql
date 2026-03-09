-- bench_008: Task exclusion flags for filtering noise from analysis
-- Tasks where all models produce correct solutions but test patches never match
-- (name mismatches, valid alternative implementations) should be excluded from
-- comparative analysis since they don't discriminate between variants.

ALTER TABLE bench_tasks ADD COLUMN excluded BOOLEAN DEFAULT FALSE;
ALTER TABLE bench_tasks ADD COLUMN exclude_reason TEXT DEFAULT '';
