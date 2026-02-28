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
  config=$2
  body="{\"name\": \"$name\", \"config\": $config}"
  tmp=$(mktemp)

  # Если коннектор уже есть и в состоянии FAILED — удаляем, затем создаём заново
  status=$(curl -sf -o /dev/null -w '%{http_code}' "$CONNECT_URL/connectors/$name" 2>/dev/null || true)
  if [ "$status" = "200" ]; then
    echo "Connector $name exists, checking status..."
    state=$(curl -sf "$CONNECT_URL/connectors/$name/status" 2>/dev/null | grep -o '"state":"[^"]*"' | head -1)
    if [ -n "$state" ] && echo "$state" | grep -q FAILED; then
      echo "Deleting failed connector $name..."
      curl -sf -X DELETE "$CONNECT_URL/connectors/$name" 2>/dev/null || true
      sleep 2
    fi
  fi

  curl -s -X POST -H "Content-Type: application/json" --data "$body" "$CONNECT_URL/connectors" -w "\n%{http_code}" -o "$tmp" 2>/dev/null || true
  code=$(tail -1 "$tmp" 2>/dev/null || echo "000")

  if [ "$code" = "201" ] || [ "$code" = "200" ]; then
    echo "Registered connector: $name"
    rm -f "$tmp"
    return 0
  fi

  if [ "$code" = "409" ]; then
    echo "Connector $name already exists, updating config..."
  else
    echo "POST failed (HTTP $code). Response: $(sed '$d' "$tmp" 2>/dev/null || echo '(no body)')"
  fi

  # Обновить конфиг (коннектор уже существует или при 409)
  if curl -sf -X PUT -H "Content-Type: application/json" --data "$config" "$CONNECT_URL/connectors/$name/config" 2>/dev/null; then
    echo "Updated connector config: $name"
    rm -f "$tmp"
    return 0
  fi
  resp=$(curl -s -X PUT -H "Content-Type: application/json" --data "$config" "$CONNECT_URL/connectors/$name/config" -o "$tmp" 2>/dev/null; cat "$tmp")
  echo "PUT failed. Response: $resp"
  echo "Failed to register connector: $name"
  rm -f "$tmp"
  return 1
}

check_status() {
  name=$1
  sleep 5
  status=$(curl -s "$CONNECT_URL/connectors/$name/status" | grep -o '"state":"[^"]*"' | head -1)
  echo "Status $name: $status"
  if echo "$status" | grep -q FAILED; then
    echo "Connector $name FAILED! Check logs."
    return 1
  fi
}

wait_for_connect

# Коннектор CRM (customers, orders). Публикация dbz_crm создаётся при инициализации БД (init-db/crm).
register_connector "crm-connector" '{
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
}'

# Коннектор Телеметрии. Публикация dbz_telemetry создаётся при инициализации БД (init-db/telemetry).
register_connector "telemetry-connector" '{
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
}'

# После register_connector
check_status "telemetry-connector"
check_status "crm-connector"

echo "All connectors registered."
