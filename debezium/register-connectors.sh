#!/bin/sh
# Регистрация Debezium PostgreSQL коннекторов для CRM и Телеметрии
set -e
CONNECT_URL="${DEBEZIUM_CONNECT_URL:-http://debezium-connect:8083}"

wait_for_connect() {
  i=1
  while [ $i -le 30 ]; do
    if curl -sf "$CONNECT_URL/" > /dev/null; then
      echo "Debezium Connect is ready."
      return 0
    fi
    echo "Waiting for Debezium Connect... ($i/30)"
    sleep 5
    i=$(( i + 1 ))
  done
  echo "Timeout waiting for Debezium Connect"
  exit 1
}

register_connector() {
  name=$1
  body=$2
  if curl -sf -X POST -H "Content-Type: application/json" --data "$body" "$CONNECT_URL/connectors" 2>/dev/null; then
    echo "Registered connector: $name"
    return 0
  fi
  if curl -sf -X PUT -H "Content-Type: application/json" --data "$body" "$CONNECT_URL/connectors/$name/config" 2>/dev/null; then
    echo "Updated connector: $name"
    return 0
  fi
  echo "Failed to register connector: $name"
  return 1
}

wait_for_connect

# Коннектор CRM (customers, orders)
register_connector "crm-connector" '{
  "name": "crm-connector",
  "config": {
    "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
    "database.hostname": "crm_db",
    "database.port": "5432",
    "database.user": "crm_user",
    "database.password": "crm_password",
    "database.dbname": "crm_db",
    "topic.prefix": "crm_db",
    "plugin.name": "pgoutput",
    "publication.name": "dbz_crm",
    "schema.include.list": "public",
    "table.include.list": "public.customers,public.orders"
  }
}'

# Коннектор Телеметрии
register_connector "telemetry-connector" '{
  "name": "telemetry-connector",
  "config": {
    "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
    "database.hostname": "telemetry_db",
    "database.port": "5432",
    "database.user": "telemetry_user",
    "database.password": "telemetry_password",
    "database.dbname": "telemetry_db",
    "topic.prefix": "telemetry_db",
    "plugin.name": "pgoutput",
    "publication.name": "dbz_telemetry",
    "schema.include.list": "public",
    "table.include.list": "public.telemetry_events"
  }
}'

echo "All connectors registered."
