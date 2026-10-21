ALTER TABLE outbox_events
    ADD COLUMN occurred_at timestamptz NOT NULL DEFAULT now();

UPDATE outbox_events SET occurred_at = created_at;

CREATE INDEX outbox_processed_retention_idx
    ON outbox_events (processed_at)
    WHERE processed_at IS NOT NULL;

CREATE INDEX analytics_processed_retention_idx
    ON analytics_events (processed_at)
    WHERE processed_at IS NOT NULL;

CREATE INDEX analytics_dead_letter_idx
    ON analytics_events (dead_lettered_at)
    WHERE dead_lettered_at IS NOT NULL;
