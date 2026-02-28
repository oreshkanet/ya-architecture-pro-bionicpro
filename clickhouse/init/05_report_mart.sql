-- Витрина отчётности: объединение данных CRM (customers) и агрегатов телеметрии
-- MV срабатывает при вставке в customers_ch; для каждого пользователя считаем агрегаты телеметрии за последние 30 дней.

CREATE TABLE IF NOT EXISTS reports.report_mart (
    user_id String,
    full_name Nullable(String),
    email Nullable(String),
    registered_at Nullable(DateTime64(3)),
    period_start Date,
    period_end Date,
    usage_hours Float64,
    session_count UInt32,
    avg_daily_use Float64,
    updated_at DateTime64(3)
) ENGINE = ReplacingMergeTree(updated_at)
ORDER BY user_id;

CREATE MATERIALIZED VIEW IF NOT EXISTS reports.mv_report_mart TO reports.report_mart AS
SELECT
    c.user_id,
    c.full_name,
    c.email,
    c.registered_at,
    toDate(now()) - 30 AS period_start,
    toDate(now()) AS period_end,
    coalesce(t.usage_hours, 0) AS usage_hours,
    coalesce(t.session_count, 0) AS session_count,
    coalesce(t.avg_daily_use, 0) AS avg_daily_use,
    now64(3) AS updated_at
FROM reports.customers_ch AS c
LEFT JOIN (
    SELECT
        user_id,
        sum(CASE WHEN event_type = 'usage_hours' THEN value ELSE 0 END) AS usage_hours,
        count() AS session_count,
        if(countDistinct(toDate(created_at)) > 0,
           sum(CASE WHEN event_type = 'usage_hours' THEN value ELSE 0 END) / countDistinct(toDate(created_at)),
           0) AS avg_daily_use
    FROM reports.telemetry_events_ch
    FINAL
    WHERE created_at >= toDateTime(toDate(now()) - 30)
      AND created_at < toDateTime(toDate(now()) + 1)
    GROUP BY user_id
) AS t ON c.user_id = t.user_id;
