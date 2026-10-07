-- Persist a stable request fingerprint for idempotency conflict detection.
ALTER TABLE orders ADD COLUMN request_hash text;
UPDATE orders SET request_hash = encode(digest(id::text, 'sha256'), 'hex');
ALTER TABLE orders ALTER COLUMN request_hash SET NOT NULL;
