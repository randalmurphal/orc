-- Migration 072: Tighten event deduplication to true duplicate payloads only
--
-- The old unique index used only (task_id, event_type, phase, created_at).
-- With coarse clock resolution, distinct activity/heartbeat events published in
-- the same instant were incorrectly dropped. Include iteration/source/data so
-- only true duplicate events are ignored.

DROP INDEX IF EXISTS idx_event_log_dedup;

CREATE UNIQUE INDEX IF NOT EXISTS idx_event_log_dedup
ON event_log(
    task_id,
    event_type,
    COALESCE(phase, ''),
    COALESCE(iteration, -1),
    COALESCE(source, ''),
    created_at,
    COALESCE(data, '')
);
