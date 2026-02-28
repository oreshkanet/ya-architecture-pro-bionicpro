-- Загрузка первоначальных данных телеметрии
COPY telemetry_events (user_id, device_id, event_type, value)
FROM '/docker-entrypoint-initdb.d/data/telemetry_events.csv'
WITH (FORMAT csv, HEADER true);
