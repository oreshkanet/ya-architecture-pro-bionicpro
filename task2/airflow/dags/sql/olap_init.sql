-- OLAP: витрина отчётности для сервиса отчётов
-- Доступ по user_id (Keycloak sub) для быстрой выдачи отчёта по пользователю

CREATE TABLE IF NOT EXISTS report_mart (
    user_id         VARCHAR(255) NOT NULL PRIMARY KEY,
    full_name       VARCHAR(255),
    email           VARCHAR(255),
    registered_at   TIMESTAMP WITH TIME ZONE,
    period_start    DATE NOT NULL,
    period_end      DATE NOT NULL,
    usage_hours     NUMERIC(10, 2) DEFAULT 0,
    session_count   INTEGER DEFAULT 0,
    avg_daily_use   NUMERIC(10, 2) DEFAULT 0,
    updated_at      TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_report_mart_updated ON report_mart(updated_at);

COMMENT ON TABLE report_mart IS 'Витрина отчётов: телеметрия и данные CRM по пользователям';
