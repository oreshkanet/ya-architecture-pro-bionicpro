-- При появлении новых событий телеметрии добавляем строки в витрину для затронутых user_id.

CREATE MATERIALIZED VIEW IF NOT EXISTS reports.mv_report_mart_from_telemetry TO reports.report_mart AS
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
FROM (SELECT DISTINCT user_id FROM reports.telemetry_events_ch) AS u
INNER JOIN reports.customers_ch AS c FINAL ON c.user_id = u.user_id
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
) AS t ON u.user_id = t.user_id;
