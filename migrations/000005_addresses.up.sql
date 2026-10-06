CREATE TABLE user_addresses (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    label text NOT NULL,
    address text NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_addresses_label_length CHECK (char_length(label) BETWEEN 1 AND 50),
    CONSTRAINT user_addresses_address_length CHECK (char_length(address) BETWEEN 3 AND 500)
);

CREATE INDEX user_addresses_user_idx ON user_addresses (user_id, created_at);
CREATE UNIQUE INDEX user_addresses_one_default_idx ON user_addresses (user_id) WHERE is_default;
