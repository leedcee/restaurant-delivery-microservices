-- Roll back idempotency request fingerprinting.
ALTER TABLE orders DROP COLUMN request_hash;
