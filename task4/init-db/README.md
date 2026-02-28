# Инициализация БД CRM и Телеметрии

Схема и первоначальные данные загружаются при **первом** запуске контейнеров PostgreSQL (когда том данных пуст).

- **init-db/crm/** — монтируется в `crm_db` как `/docker-entrypoint-initdb.d/`:
  - `01_schema.sql` — таблицы customers, orders, REPLICA IDENTITY, публикация dbz_crm
  - `02_load.sql` — загрузка из `data/customers.csv` и `data/orders.csv`

- **init-db/telemetry/** — монтируется в `telemetry_db`:
  - `01_schema.sql` — таблица telemetry_events, публикация dbz_telemetry
  - `02_load.sql` — загрузка из `data/telemetry_events.csv`

При уже существующих данных (непустой том) скрипты не выполняются. Чтобы переинициализировать, удалите каталоги `./data/postgres/crm` и `./data/postgres/telemetry` (или `docker compose down -v` для полного сброса).
