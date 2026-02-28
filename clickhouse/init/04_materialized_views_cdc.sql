-- MaterializedView: парсинг сообщений Debezium из Kafka и вставка в MergeTree

CREATE MATERIALIZED VIEW IF NOT EXISTS reports.mv_crm_customers TO reports.customers_ch AS
SELECT
    nullIf(JSONExtractString(value, 'payload', 'after', 'user_id'), '') AS user_id,
    nullIf(JSONExtractString(value, 'payload', 'after', 'full_name'), '') AS full_name,
    nullIf(JSONExtractString(value, 'payload', 'after', 'email'), '') AS email,
    parseDateTime64BestEffortOrNull(JSONExtractString(value, 'payload', 'after', 'registered_at')) AS registered_at,
    coalesce(JSONExtractUInt(value, 'payload', 'ts_ms'), toUnixTimestamp64Milli(now64(3))) AS _version,
    nullIf(JSONExtractString(value, 'payload', 'op'), '') AS _op
FROM reports.kafka_crm_customers
WHERE JSONExtractString(value, 'payload', 'op') IN ('c', 'r', 'u')
  AND length(nullIf(JSONExtractString(value, 'payload', 'after', 'user_id'), '')) > 0;

CREATE MATERIALIZED VIEW IF NOT EXISTS reports.mv_telemetry_events TO reports.telemetry_events_ch AS
SELECT
    JSONExtractInt(raw.value, 'payload', 'after', 'id') AS id,
    nullIf(JSONExtractString(raw.value, 'payload', 'after', 'user_id'), '') AS user_id,
    nullIf(JSONExtractString(raw.value, 'payload', 'after', 'device_id'), '') AS device_id,
    nullIf(JSONExtractString(raw.value, 'payload', 'after', 'event_type'), '') AS event_type,
    if(
        length(JSONExtractString(raw.value, 'payload', 'after', 'value')) > 0 AND length(tryBase64Decode(JSONExtractString(raw.value, 'payload', 'after', 'value'))) > 0,
        reinterpretAsInt32(reverse(
            if(
                length(tryBase64Decode(JSONExtractString(raw.value, 'payload', 'after', 'value'))) < 4,
                concat(repeat('\x00', 4 - length(tryBase64Decode(JSONExtractString(raw.value, 'payload', 'after', 'value')))), tryBase64Decode(JSONExtractString(raw.value, 'payload', 'after', 'value'))),
                substring(tryBase64Decode(JSONExtractString(raw.value, 'payload', 'after', 'value')), 1, 4)
            )
        )) / 10000.0,
        toFloat64OrNull(JSONExtractString(raw.value, 'payload', 'after', 'value'))
    ) AS value,
    parseDateTime64BestEffortOrNull(JSONExtractString(raw.value, 'payload', 'after', 'created_at')) AS created_at,
    coalesce(JSONExtractUInt(raw.value, 'payload', 'ts_ms'), toUnixTimestamp64Milli(now64(3))) AS _version,
    nullIf(JSONExtractString(raw.value, 'payload', 'op'), '') AS _op
FROM reports.kafka_telemetry_events AS raw
WHERE (JSONExtractString(raw.value, 'payload', 'op') IN ('c', 'r', 'u'))
  AND (length(nullIf(JSONExtractString(raw.value, 'payload', 'after', 'user_id'), '')) > 0);
