-- Таблицы Kafka Engine для приёма событий Debezium из топиков Kafka
-- Топики: crm_db.public.customers, telemetry_db.public.telemetry_events

CREATE TABLE IF NOT EXISTS reports.kafka_crm_customers (
    value String
) ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka:29092',
    kafka_topic_list = 'crm_db.public.customers',
    kafka_group_name = 'clickhouse_crm_customers',
    kafka_format = 'JSONAsString',
    kafka_num_consumers = 1,
    kafka_thread_per_consumer = 1;

CREATE TABLE IF NOT EXISTS reports.kafka_telemetry_events (
    value String
) ENGINE = Kafka
SETTINGS
    kafka_broker_list = 'kafka:29092',
    kafka_topic_list = 'telemetry_db.public.telemetry_events',
    kafka_group_name = 'clickhouse_telemetry',
    kafka_format = 'JSONAsString',
    kafka_num_consumers = 1,
    kafka_thread_per_consumer = 1;
