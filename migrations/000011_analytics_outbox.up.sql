CREATE TABLE analytics_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    event_name text NOT NULL,
    event_version integer NOT NULL DEFAULT 1 CHECK (event_version > 0),
    trace_id text,
    properties jsonb NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    attempts integer NOT NULL DEFAULT 0,
    available_at timestamptz NOT NULL DEFAULT now(),
    processed_at timestamptz,
    dead_lettered_at timestamptz,
    last_error text
);

CREATE INDEX analytics_events_ready_idx
    ON analytics_events (available_at, occurred_at)
    WHERE processed_at IS NULL AND dead_lettered_at IS NULL;

