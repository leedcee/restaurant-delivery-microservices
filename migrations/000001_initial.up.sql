CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE order_status AS ENUM (
    'pending',
    'accepted',
    'rejected',
    'preparing',
    'ready',
    'delivering',
    'delivered',
    'cancelled'
);

CREATE TABLE restaurants (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id text NOT NULL UNIQUE,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    is_active boolean NOT NULL DEFAULT true,
    is_open boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE restaurant_integrations (
    restaurant_id uuid PRIMARY KEY REFERENCES restaurants(id) ON DELETE CASCADE,
    order_endpoint text NOT NULL,
    api_key_hash text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE menu_categories (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id uuid NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    external_id text NOT NULL,
    name text NOT NULL,
    position integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (restaurant_id, external_id)
);

CREATE TABLE products (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    restaurant_id uuid NOT NULL REFERENCES restaurants(id) ON DELETE CASCADE,
    category_id uuid NOT NULL REFERENCES menu_categories(id) ON DELETE RESTRICT,
    external_id text NOT NULL,
    name text NOT NULL,
    description text NOT NULL DEFAULT '',
    price_minor bigint NOT NULL CHECK (price_minor >= 0),
    currency char(3) NOT NULL DEFAULT 'RUB',
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (restaurant_id, external_id)
);

CREATE TABLE product_inventory (
    product_id uuid PRIMARY KEY REFERENCES products(id) ON DELETE CASCADE,
    quantity integer NOT NULL CHECK (quantity >= 0),
    is_available boolean NOT NULL DEFAULT true,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL,
    restaurant_id uuid NOT NULL REFERENCES restaurants(id) ON DELETE RESTRICT,
    idempotency_key text NOT NULL,
    status order_status NOT NULL DEFAULT 'pending',
    total_minor bigint NOT NULL CHECK (total_minor >= 0),
    currency char(3) NOT NULL DEFAULT 'RUB',
    delivery_address text NOT NULL,
    rejection_reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, idempotency_key)
);

CREATE TABLE order_items (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id uuid NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    product_name text NOT NULL,
    unit_price_minor bigint NOT NULL CHECK (unit_price_minor >= 0),
    quantity integer NOT NULL CHECK (quantity > 0),
    total_minor bigint NOT NULL CHECK (total_minor >= 0),
    UNIQUE (order_id, product_id)
);

CREATE TABLE order_status_history (
    id bigserial PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    from_status order_status,
    to_status order_status NOT NULL,
    source text NOT NULL CHECK (source IN ('system', 'user', 'restaurant')),
    reason text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX products_restaurant_active_idx
    ON products (restaurant_id, is_active);
CREATE INDEX orders_user_created_idx
    ON orders (user_id, created_at DESC);
CREATE INDEX orders_restaurant_created_idx
    ON orders (restaurant_id, created_at DESC);
CREATE INDEX order_history_order_created_idx
    ON order_status_history (order_id, created_at);
CREATE INDEX outbox_ready_idx
    ON outbox_events (available_at, created_at)
    WHERE processed_at IS NULL;

INSERT INTO restaurants (id, external_id, name, description)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'demo-cafe',
    'Тёплый хлеб',
    'Завтраки и домашняя кухня'
);

INSERT INTO restaurant_integrations (restaurant_id, order_endpoint, api_key_hash)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'http://demo-restaurant:8081/integration/v1/orders',
    encode(digest('demo-secret', 'sha256'), 'hex')
);
