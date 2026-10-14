DROP INDEX IF EXISTS orders_status_updated_idx;

ALTER TABLE order_status_history
    DROP COLUMN IF EXISTS order_version,
    DROP COLUMN IF EXISTS reason_code;

ALTER TABLE orders
    DROP COLUMN IF EXISTS status_reason_code,
    DROP COLUMN IF EXISTS version;
