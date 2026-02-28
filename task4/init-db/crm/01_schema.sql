-- CRM: клиенты (связь с Keycloak по user_id = sub)
CREATE TABLE IF NOT EXISTS customers (
    id                  SERIAL PRIMARY KEY,
    user_id             VARCHAR(255) NOT NULL UNIQUE,
    full_name           VARCHAR(255),
    email               VARCHAR(255),
    registered_at       TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS orders (
    id                  SERIAL PRIMARY KEY,
    order_number        VARCHAR(255) NOT NULL UNIQUE,
    total               NUMERIC(10,2),
    discount            NUMERIC(10,2),
    buyer_id            VARCHAR(255),
    registered_at       TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- CDC: полная строка при UPDATE/DELETE для Debezium
ALTER TABLE customers REPLICA IDENTITY FULL;
ALTER TABLE orders REPLICA IDENTITY FULL;

-- Публикация для Change Data Capture (Debezium)
DROP PUBLICATION IF EXISTS dbz_crm;
CREATE PUBLICATION dbz_crm FOR TABLE customers, orders;
