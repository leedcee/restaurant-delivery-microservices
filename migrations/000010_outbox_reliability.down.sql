DROP INDEX IF EXISTS outbox_dead_letter_idx;
DROP INDEX IF EXISTS outbox_ready_idx;

ALTER TABLE outbox_events
    DROP COLUMN IF EXISTS dead_lettered_at,
    DROP COLUMN IF EXISTS correlation_id,
    DROP COLUMN IF EXISTS event_version;

CREATE INDEX outbox_ready_idx
    ON outbox_events (available_at, created_at)
    WHERE processed_at IS NULL;
