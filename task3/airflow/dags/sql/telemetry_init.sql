
-- Телеметрия: события с датчиков протезов по пользователям
CREATE TABLE IF NOT EXISTS telemetry_events (
    id          BIGSERIAL PRIMARY KEY,
    user_id     VARCHAR(255) NOT NULL,
    device_id   VARCHAR(255),
    event_type  VARCHAR(64),
    value       NUMERIC(12, 4),
    created_at  TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_telemetry_user_created ON telemetry_events(user_id, created_at);

-- CDC: полная строка при UPDATE/DELETE для Debezium
ALTER TABLE telemetry_events REPLICA IDENTITY FULL;

-- Публикация для Change Data Capture (Debezium)
DROP PUBLICATION IF EXISTS dbz_telemetry;
CREATE PUBLICATION dbz_telemetry FOR TABLE telemetry_events;
