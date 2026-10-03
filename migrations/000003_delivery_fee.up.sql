ALTER TABLE orders
    ADD COLUMN delivery_fee_minor bigint NOT NULL DEFAULT 13000
    CHECK (delivery_fee_minor >= 0);
