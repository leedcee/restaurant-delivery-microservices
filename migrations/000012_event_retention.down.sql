DROP INDEX IF EXISTS analytics_dead_letter_idx;
DROP INDEX IF EXISTS analytics_processed_retention_idx;
DROP INDEX IF EXISTS outbox_processed_retention_idx;

ALTER TABLE outbox_events DROP COLUMN IF EXISTS occurred_at;
