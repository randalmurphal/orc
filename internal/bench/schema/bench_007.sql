-- Add applicable_tiers to variants for tier-scoped combo variants.
-- When non-empty, restricts which task tiers a variant runs against,
-- preventing wasteful runs (e.g., combo variants on trivial tasks).
ALTER TABLE bench_variants ADD COLUMN applicable_tiers TEXT DEFAULT '[]';
