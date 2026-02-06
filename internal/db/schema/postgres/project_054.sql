-- Migration 054: Rename task status 'resolved' to 'closed'
-- The 'resolve' command is renamed to 'close' to avoid confusion with "resolving a problem".
-- 'close' means: "I'm done with this task, I don't want to pursue it further."

-- Migrate task status
UPDATE tasks SET status = 'closed' WHERE status = 'resolved';

-- Migrate metadata keys in JSON blob
UPDATE tasks SET metadata = REPLACE(
    REPLACE(
        REPLACE(
            REPLACE(metadata, '"force_resolved"', '"force_closed"'),
            '"resolution_message"', '"close_message"'),
        '"resolved_at"', '"closed_at"'),
    '"resolved":', '"closed":')
WHERE metadata LIKE '%resolved%';
