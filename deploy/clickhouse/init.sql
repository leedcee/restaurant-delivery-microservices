CREATE DATABASE IF NOT EXISTS analytics;

CREATE TABLE IF NOT EXISTS analytics.raw_events
(
    event_id UUID,
    event_version UInt16,
    event_name LowCardinality(String),
    occurred_at DateTime64(3, 'UTC'),
    ingested_at DateTime64(3, 'UTC') DEFAULT now64(3),
    trace_id String,
    properties String
)
ENGINE = ReplacingMergeTree(ingested_at)
PARTITION BY toYYYYMM(occurred_at)
ORDER BY event_id
TTL occurred_at + INTERVAL 365 DAY DELETE;

CREATE VIEW IF NOT EXISTS analytics.order_funnel AS
SELECT
    toDate(occurred_at) AS day,
    event_name,
    if(event_name = 'order_created', JSONExtractString(properties, 'status'), JSONExtractString(properties, 'to_status')) AS status,
    count() AS events,
    sumIf(JSONExtractInt(properties, 'total_minor'), event_name = 'order_created') AS gmv_minor
FROM analytics.raw_events FINAL
GROUP BY day, event_name, status;

