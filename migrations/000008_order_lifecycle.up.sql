ALTER TABLE orders
    ADD COLUMN version bigint NOT NULL DEFAULT 1 CHECK (version > 0),
    ADD COLUMN status_reason_code text;

ALTER TABLE order_status_history
    ADD COLUMN reason_code text,
    ADD COLUMN order_version bigint NOT NULL DEFAULT 1 CHECK (order_version > 0);

CREATE INDEX orders_status_updated_idx ON orders (status, updated_at);
