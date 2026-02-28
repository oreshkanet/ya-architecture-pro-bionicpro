-- MergeTree-таблицы для данных из CDC (после парсинга Debezium envelope)
-- ReplacingMergeTree по версии (ts_ms) для дедупликации при обновлениях

CREATE TABLE IF NOT EXISTS reports.customers_ch (
    user_id String,
    full_name Nullable(String),
    email Nullable(String),
    registered_at Nullable(DateTime64(3)),
    _version UInt64,
    _op String
) ENGINE = ReplacingMergeTree(_version)
ORDER BY user_id
SETTINGS allow_nullable_key = 1;

CREATE TABLE IF NOT EXISTS reports.telemetry_events_ch (
    id Nullable(Int64),
    user_id String,
    device_id Nullable(String),
    event_type Nullable(String),
    value Nullable(Float64),
    created_at Nullable(DateTime64(3)),
    _version UInt64,
    _op String
) ENGINE = ReplacingMergeTree(_version)
ORDER BY (user_id, created_at, id)
SETTINGS allow_nullable_key = 1;
