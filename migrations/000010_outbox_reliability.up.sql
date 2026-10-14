ALTER TABLE outbox_events
    ADD COLUMN event_version integer NOT NULL DEFAULT 1 CHECK (event_version > 0),
    ADD COLUMN correlation_id uuid NOT NULL DEFAULT gen_random_uuid(),
    ADD COLUMN dead_lettered_at timestamptz;

DROP INDEX IF EXISTS outbox_ready_idx;
CREATE INDEX outbox_ready_idx
    ON outbox_events (available_at, created_at)
    WHERE processed_at IS NULL AND dead_lettered_at IS NULL;

CREATE INDEX outbox_dead_letter_idx
    ON outbox_events (dead_lettered_at DESC)
    WHERE dead_lettered_at IS NOT NULL;
