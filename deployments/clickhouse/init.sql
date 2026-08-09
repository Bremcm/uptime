CREATE TABLE IF NOT EXISTS default.checks
(
    `monitor_id` UInt64,
    `status` String,
    `status_code` UInt16,
    `latency_ms` UInt32,
    `error` String,
    `checked_at` DateTime
)
ENGINE = MergeTree
PARTITION BY toYYYYMM(checked_at)
ORDER BY (monitor_id, checked_at);

CREATE TABLE IF NOT EXISTS default.checks_hourly
(
    `monitor_id` UInt64,
    `hour` DateTime,
    `sum_latency` UInt64,
    `count_checks` UInt64,
    `count_up` UInt64
)
ENGINE = SummingMergeTree
PARTITION BY toYYYYMM(hour)
ORDER BY (monitor_id, hour);

CREATE MATERIALIZED VIEW IF NOT EXISTS default.checks_hourly_mv TO default.checks_hourly
AS SELECT
    monitor_id,
    toStartOfHour(checked_at) AS hour,
    sum(latency_ms) AS sum_latency,
    count() AS count_checks,
    countIf(status = 'up') AS count_up
FROM default.checks
GROUP BY
    monitor_id,
    hour;