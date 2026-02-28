# BionicPRO

> Проектная работа 9 спринта курса "Архитектура ПО: продвинутый уровень"

## Повышение безопасности системы

### Архитектурное решение для управления учётными данными

Основные принципы:

- Централизованная аутентификация через новый сервис bionicpro-auth,
- Локальное хранение персональных данных в стране представительства,
- Безопасная схема токенов с хранением на бэкенде,
- Поддержка внешних удостоверяющих служб через брокеринг.

Диаграмма C4:
[BionicPRO_C4_model](./diagram/BionicPRO_C4_model_task1_tobe.png)

![BionicPRO_C4_model](./diagram/BionicPRO_C4_model_task1_tobe.png)

На схеме отражено несколько новых компонентов:

1. **bionikpro-auth**:
   - Прием аутентификационных запросов от фронтенда,
   - Генерация и управление сессиями,
   - Хранение токенов в защищенном хранилище,
   - Интеграция с различными внешними IdP,
   - Ротация сессий;
2. Keycloak:
   - Поддержка PKCE,
   - Access- и Refresh-токены,
   - MFA (TOTP),
   - Identity Brokering для Яндекс ID и других провайдеров;
3. OpenLDAP:
   - Хранение локальных учетных данных пользователей,
   - Синхронизация с Keycloak,
   - Маппинг ролей для разных представительств.

### Улучшение безопасности существующего приложения

Для повышения безопасности приложения используется Authorization Code + PKCE (S256) вместо обычного Code Grant: код авторизации нельзя подменить без `code_verifier`, который есть только у фронтенда.

Для перехода на PCKE выполнено:

1. Фронтенд (`frontend/src/App.tsx`):
   - Включён PKCE с методом S256 в опциях инициализации Keycloak.
   - В `ReactKeycloakProvider` передаётся `initOptions: { pkceMethod: 'S256' }`, чтобы при логине и обмене кода использовались code_verifier и `code_challenge` (S256).
   ```ts
   const initOptions = {
     pkceMethod: 'S256' as const,
   };
   // ...
   <ReactKeycloakProvider authClient={keycloak} initOptions={initOptions}>
   ```
2. Keycloak (`keycloak/realm-export.json`):
   Для клиента **reports-drontend**:
   - В атрибуты клиента добавлено: `pkce.code.challenge.method": "S256"` — разрешён только метод S256 для PKCE.
   - `directAccessGrantsEnabled` переведён в `false`, чтобы отключить Resource Owner Password Credentials (логин/пароль напрямую) и оставить только Authorization Code + PKCE для SPA.

![pkce_1](./asset/task1/pkce_1.png)
![pkce_2](./asset/task1/pkce_2.png)

### Обеспечение безопасного получения и хранение access-и refresh-токенов.

#### bionicpro-auth (Go)
- **Роуты:** `/login` (редирект в Keycloak с `scope=openid offline_access`), `/auth/callback` (обмен code на токены, сохранение сессии, установка cookie, редирект на фронт), `/logout`, `/session/check`, `/session/validate` (с ротацией), `/api/reports` (прокси к reports API с Bearer).
- **Сессии:** in-memory store с TTL; данные сессии — access_token, зашифрованный `refresh_token` (AES-GCM), expires_at. Привязка токенов к session id.
- **Cookie:** имя `bionicpro_session`, HTTP-only, Secure (по конфигу), SameSite=Lax, MaxAge=SessionMaxAge (по умолчанию 1800 с).
- **Обновление access_token:** при обращении к защищённому ресурсу, если `access_token` истёк, обмен `refresh_token` на новую пару в Keycloak, обновление данных сессии (при пустом новом `refresh_token` старый сохраняется).
- **Ротация сессии:** при каждом успешном запросе к защищённому ресурсу (`requireSession`) — новый session id, перенос данных, установка новой cookie (защита от session fixation).
- **Конфиг:** порт, Keycloak (URL, realm, client id/secret), FrontendURL, ReportsAPIURL, SessionCookieName, SessionMaxAge, SecureCookie, EncryptionKey; SessionMaxAge читается из env.

#### Keycloak (realm-export.json)
- **realm:** `accessTokenLifespan: 120` (2 мин), поддержка offline-сессий.
- **Клиент bionicpro-auth:** confidential, client-secret, redirectUris для auth сервиса, `defaultClientScopes` включают `offline_access` для выдачи refresh_token.
- **Realm-роль** `offline_access`**:** для выдачи refresh-токена при запросе `scope=offline_access` пользователь должен иметь realm-роль `offline_access`, добавлена роль в список realm roles.

#### Frontend
- Убран прямой обмен с Keycloak; все запросы к auth идут на `REACT_APP_AUTH_URL` (например `http://localhost:8001`) с `credentials: 'include'`.
- Логин — редирект на `${authUrl}/login`, логаут — на `${authUrl}/logout`, проверка авторизации — `GET ${authUrl}/session/check`, скачивание отчёта — `GET ${authUrl}/api/reports`. Сессионная cookie передаётся автоматически.

#### docker-compose.yaml
- Добавлен сервис **bionicpro-auth**: сборка из `./bionicpro-auth`, порт 8001, переменные KEYCLOAK_\*, FRONTEND_URL, REPORTS_API_URL, SESSION_\*, ENCRYPTION_KEY; `depends_on: keycloak`.
- У **frontend** добавлены `REACT_APP_AUTH_URL: http://localhost:8001` и `depends_on: bionicpro-auth`.

#### Примеры работы

![auth_1](./asset/task1/auth_1.png)
![auth_2](./asset/task1/auth_2.png)
![auth_3](./asset/task1/auth_3.png)

### LDAP для возможности получения данных о пользователях представительства BionicPRO в другой стране

#### OpenLDAP в `docker-compose.yaml`:

- Сервис openldap (образ osixia/openldap:1.5.0) с базой dc=example,dc=com.
- При первом старте подхватывается `ldap/config.ldif` как bootstrap 50-data.ldif (OU, пользователи, группы).
- Для keycloak добавлена зависимость от openldap.
- Порты: `389`, `636`.

![ldap_1](./asset/task1/ldap_1.png)
![ldap_2](./asset/task1/ldap_2.png)
![ldap_3](./asset/task1/ldap_3.png)
![ldap_4](./asset/task1/ldap_4.png)

#### Маппинг ролей

- Маппер role-ldap-mapper: группы из `ou=Groups,dc=example,dc=com` (objectClass groupOfNames, атрибут `cn`) отображаются на realm roles с тем же именем. Пользователи получают роли по членству в этих группах (member/DN).

![ldap_5](./asset/task1/ldap_5.png)

#### Синхронизация пользователей

![ldap_6](./asset/task1/ldap_6.png)
![ldap_7](./asset/task1/ldap_7.png)

#### Авторизация под доменным пользователем

![ldap_8](./asset/task1/ldap_8.png)
![ldap_9](./asset/task1/ldap_9.png)

### Настройка MFA

Для настройки обязательной OTP-аутентификации (MFA) в Keycloak для всех пользователей, были внесены следующие изменения в файл `realm-export.json`:

- `syncRegistrations: true` — обязательно для хранения секрета OTP пользователей из LDAP. Без этого шага MFA для LDAP-пользователей работать не будет.
- `browserFlow: "browser-with-otp"` - активирует кастомный поток аутентификации для всех браузерных сессий.
- `auth-cookie` - сохраняет SSO: если сессия активна — OTP не запрашивается повторно.
- `auth-otp-form` - гарантирует обязательный ввод OTP после пароля для новых сессий.
- `CONFIGURE_TOTP` - автоматически перенаправляет пользователя на настройку приложения-аутентификатора при первом входе после включения MFA.
- `otpPolicy` - Явно задаёт параметры TOTP (совместимость с Google Authenticator, Authy и др.).

#### Результат настройки MFA в UI

![totp_1](./asset/task1/totp_1.png)
![totp_2](./asset/task1/totp_2.png)
![totp_3](./asset/task1/totp_3.png)
![totp_4](./asset/task1/totp_4.png)

#### Первый вход пользователя

Пользователи (включая синхронизированных из LDAP) автоматически перенаправятся на страницу настройки OTP после ввода пароля, на которой нужно:
- Отсканировать QR-код приложением (Google Authenticator, Authy и др.)
- Ввести проверочный код для завершения настройки.

![totp_5](./asset/task1/totp_5.png)
![totp_6](./asset/task1/totp_6.png)
![totp_7](./asset/task1/totp_7.png)
![totp_8](./asset/task1/totp_8.png)

#### Второй вход пользователя

При повторном входе в систему зарпашивается код MFA:

![totp_10](./asset/task1/totp_10.png)

#### TOTP в профиле пользователя

В профиле пользователя отображается настройка OTP:

![totp_9](./asset/task1/totp_9.png)

### Добавление OAuth 2.0 от Яндекс ID

В личном кабинете [https://oauth.yandex.ru/](https://oauth.yandex.ru/) зарегистрировано новое приложения для подключения аутентификации.
![yaid_1](./asset/task1/yaid_1.png)

![yaid_2](./asset/task1/yaid_2.png)

Для подключения к Keycloak в секцию `identityProviders` добавлена настройка для Яндекс ID и маппинг полей, а в секцию `authenticationFlows` добавлен новый флоу `first broker login`, а в основной флоу добавлен альтернативный шаг `identity-provider-redirector`.

В итоге, на форме авторизации появилась кнопка авторизации Yandex ID
![yaid_3](./asset/task1/yaid_3.png)

Дополнительное предупреждение (сервис в Яндекс не проходил верификацию)
![yaid_4](./asset/task1/yaid_4.png)

Форма авторизации в Яндекс и запрашиваемые права:
![yaid_5](./asset/task1/yaid_5.png)

После авторизации для пользователя в Keycloak автоматически добавляется новая связь IdP:
![yaid_6](./asset/task1/yaid_6.png)

### Прокси-сервис для Яндекс ID

Напрямую Яндекс ID не захотел подключаться к Keycloak из-за различий в протоколах обмена. Например:

- `/authorize` - флоу в Яндекс падал при наличии в scoupe значения `openid`, поэтому в прокси реализовано "вырезание" этого значения
- `/token` - Яндекс отдаёт значение немного не в том формате, который ждёт Keycloak.
- `/info` - в JSON от Яндекс нехватает идентификатора пользователя (поле `sub`)

Поэтому было принято решение реализовать промежуточный прокси-сервис, который "подгоняет" форматы запросов и ответов. 

Пример логов сервиса:
![yaid_7](./asset/task1/yaid_7.png)
![yaid_8](./asset/task1/yaid_8.png)

---

## Разработка сервиса отчётов

Реализован отдельный сервис отчётов: ETL (Airflow) формирует витрину в OLAP, бэкенд на Go отдаёт отчёт по пользователю через API. Доступ только к своему отчёту (по JWT `sub`).

### Архитектура

В реализации сервиса Отчётов принято решение использовать BFF-подход, при котором сервис авторизации `BionicPRO-Auth` выступает в качестве gateway и проксирует запросы к `Reports-Backend`. Кроме того, `BionicPRO-Auth` получает id сессии от фронтенда, находит соответствие с токеном и добавляет его в авторизацию. 

Диаграмма C4: 
[BionicPRO_C4_model_task2_tobe](./diagram/BionicPRO_C4_model_task2_tobe.drawio)

![BionicPRO_C4_model_task2_tobe](./diagram/BionicPRO_C4_model_task2_tobe.png)

Компоненты:

1. **Источники данных**
   - **CRM (PostgreSQL)** — данные о клиентах (`keycloak_user_id`, имя, контакты, дата регистрации), данные о заказах (номер, сумма, скидка, покупатель, дата регистрации)
   - **Телеметрия** — события с датчиков протезов: использование по дням, часы работы, метрики.

2. **ETL (Apache Airflow)**
   - **Extract**: чтение из CRM (клиенты) и из источников телеметрии (события по устройствам/пользователям).
   - **Transform**: объединение по пользователю, агрегация телеметрии в разрезе клиента (суммы, средние, периоды).
   - **Load**: запись в OLAP-БД в таблицу витрины `report_mart`.

3. **OLAP-БД (PostgreSQL)**
   - Витрина **report_mart**: одна или несколько записей на пользователя (например, по периодам), индексы по `user_id` для быстрого доступа по пользователю.
   - Данные пересчитываются по расписанию DAG, без тяжёлых вычислений в момент запроса.

4. **Reports API (Go)**
   - Эндпоинт `GET /reports`: по JWT определяет пользователя (`sub`), запрашивает из витрины только данные этого пользователя и возвращает отчёт (JSON или файл).

5. **Доступ**
   - Запрос к отчёту идёт через bionicpro-auth с сессионной cookie; auth подставляет Bearer и проксирует на Reports API.
   - Reports API проверяет JWT и отдаёт отчёт только для `sub` из токена (доступ только к своему отчёту).

### Airflow DAG и витрина

Для инициализации баз данных используются DAG с ограничением выполнения "только один раз":

- **[dag_crm_init](/airflow/dags/dag_crm_init.py)** - использует [crm_init.sql](/airflow/dags/sql/crm_init.sql) для инициализации структуры базы и [customers.csv](/airflow/dags/data/customers.csv), [orders.csv](/airflow/dags/data/orders.csv) для загрузки первоначальных данных.
   ![db_1](/asset/task2/db_1.png)
   ![db_2](/asset/task2/db_2.png)
- **[dag_telemetry_init](/airflow/dags/dag_telemetry_init.py)** - использует [crm_init.sql](/airflow/dags/sql/telemetry_init.sql) для инициализации структуры базы и [telemetry_events.csv](/airflow/dags/data/telemetry_events.csv), для загрузки первоначальных данных
   ![db_3](/asset/task2/db_3.png)
- **[dag_olap_init](/airflow/dags/dag_olap_init.py)** - использует [olap_init.sql](/airflow/dags/sql/olap_init.sql) для инициализации структуры базы.
   ![db_4](/asset/task2/db_4.png)

Основной ETL реализован в DAG `reports_etl_dag`:
- **DAG:** [reports_etl_dag](/airflow/dags/reports_etl_dag.py) — объединяет клиентов из CRM и агрегаты телеметрии, пишет в `report_mart`.
- **Расписание:** каждые 10 минут (`schedule_interval="*/10 * * * *"`).
- **Подключения Airflow:** `crm_postgres`, `olap_postgres` (задаются через `AIRFLOW_CONN_CRM_POSTGRES`, `AIRFLOW_CONN_OLAP_POSTGRES` в `docker-compose.yaml`).
- **Структура витрины:** `user_id` (PK), данные из CRM, период, часы использования, число сессий, среднее по дням, `updated_at`.

Список DAG в UI AirFlow:
![dag_1](/asset/task2/dag_1.png)

Настроенные подключения к базам данных:
![dag_2](/asset/task2/dag_2.png)

Инициализация БД с использованием DAG на примере CRM:
![dag_3](/asset/task2/dag_3.png)

Выполнение DAG с основным ETL:
![dag_4](/asset/task2/dag_4.png)
![dag_5](/asset/task2/dag_5.png)

Записи в БД Olap после выполнения DAG ETL:
![dag_6](/asset/task2/dag_6.png)

### Бэкенд API (Go)

В качестве бэкенда для формирования отчётов реализован сервис на Go ([reports-backend](/reports-backend/main.go)):

- **Сервис:** `reports-backend`, порт 8000.
- **Эндпоинт:** `GET /reports` — возвращает JSON-отчёт по текущему пользователю из витрины (без сложных вычислений в реальном времени).
- **Сборка:** добавлен в общий docker-compose.yaml, для сборки - `docker compose build reports-backend`

Логи сервиса Reports-backend:
![report_1](/asset/task2/report_1.png)

### Ограничение доступа

Отчёт выдаётся только для пользователя, чей JWT передан в `Authorization: Bearer <token>`. `user_id` в запросе к БД берётся только из поля `sub` токена; параметр пользователя в URL не используется.

### UI

На странице отчётов (`ReportPage`) добавлена кнопка **«Получить отчёт»**, вызывающая эндпоинт генерации отчёта через auth-прокси (`GET ${REACT_APP_AUTH_URL}/api/reports` с cookie).

После ответа отображаются период, часы использования, число сессий и дата обновления отчёта.

Вывод отчёта в UI пользователю:
![report_2](/asset/task2/report_2.png)

---

## Снижение нагрузки на базу данных

### Запись отчётов в S3

Для снижения нагрузки на базу данных OLAP сформированные отчёты (в формате HTML) сохраняются в хранилище S3. Для реализации этой функции в системе были выполнены изменения в сервисе **Reports-backend**:

- добавлен клиент для работы с S3 ([s3.go](/reports-backend/internal/storage/s3.go)), который поддерживает все необходимые функции работы с бакетами, публикации и загрузки файлов.
- добавлен лёгкий метод GetPeriodByUserID (только period_start, period_end) для построения ключа в S3 без полного запроса к витрине ([repository.go](/reports-backend/internal/repository/repository.go)). В перспективе определение периода построения отчёта лучше вынести в пользовательский UI, чтобы полностью исключить запросы к БД.
- добавлен handler для получения готового отчёта ([server.go](/reports-backend/internal/server/server.go)). 
- Реализована логика проверки наличия готового отчёта в S3, и генерации нового отчёта, если не найден:
   1. Запрос периода по `user_id`.
   2. Ключ S3: `reports/{hash(user_id)}/{period}.html`.
   3. Если объект есть в S3 — в ответе сразу отдаётся ссылка на CDN.
   4. Если нет — полный запрос к OLAP, рендер HTML, сохранение в S3, затем в ответе та же ссылка на CDN.


### Отчёты в S3 защищены

При работе с S3 не используются публичные бакеты. Доступ к объектам только с учётными данными бэкенда. Для этого реализована раздача через защищённый endpoint: вместо прямой ссылки на CDN возвращается ссылка на `/api/reports/serve?token=<токен auth-сервиса>`. 

При переходе по ней:
- проверяется сессия (cookie);
- запрос проксируется в reports-backend с заголовком `X-User-Id`;
- бэкенд проверяет подпись токена и совпадение user_id в токене с `X-User-Id`, затем отдаёт HTML из S3.

В путях S3 нет имён пользователей, а для ключа объекта используется `reports/{hash(user_id)}/{period}.html`, где `hash` — первые 16 символов hex от SHA256 от `user_id`.

### Структура в S3 и обновление кеша CDN

Структура в S3: `reports/{hash(user_id)}/{period}.html` — один объект на пользователя и период, что обеспечивает быстрый доступ по `user_id` и периоду. При смене периода после ETL создаётся новый ключ, CDN отдаёт новый контент по новой ссылке.

Обновление кеша: при перезаписи того же ключа в S3 у объекта выставляется `Cache-Control: public, max-age=300`; у Nginx — `proxy_cache_valid 200 5m`. Через 5 минут кеш CDN обновляется, а при необходимости можно добавить purge по URL или версию в ключе.

Автоудаление по времени жизни
В storage ([s3.go](/reports-backend/internal/storage/s3.go)) добавлен `EnsureBucketLifecycle` - правило `lifecycle` для префикса reports/ с `Expiration` — N дней. Количество дней задаётся через env `S3_REPORT_TTL_DAYS` (по умолчанию 7); при старте бэкенда правило выставляется в Minio.

### Фронтенд

Во фронтенде ([ReportPage.tsx](/frontend/src/components/ReportPage.tsx)) изменена страница с отчётом: по кнопке "Получить отчёт" отображается кнопка "Открыть отчёт в новой вкладке" и iframe с отчётом по этой ссылке.

### Примеры работы

Личный кабинет:
![report_1](./asset/task3/report_1.png)

Сформированный отчёт в личном кабинете:
![report_2](./asset/task3/report_2.png)

Отчёт по ссылке:
![report_3](./asset/task3/report_3.png)

Ссылка на отчёт в адресной строке:
![report_4](./asset/task3/report_4.png)

Попытка открыть отчёт по ссылке в режиме "Инкогнито":
![report_5](./asset/task3/report_5.png)
![report_6](./asset/task3/report_6.png)

---

## Повышение оперативности и стабильности работы CRM

Разделение потоков операций достигнуто за счёт CDC (Change Data Capture), Kafka и OLAP в ClickHouse: массовые выгрузки больше не нагружают CRM и телеметрию.

### CDC в PostgreSQL (CRM и Телеметрия)

Для работы с CDC в PostgreSQL:

- В **crm_init.sql** и **telemetry_init.sql**: для таблиц включено `REPLICA IDENTITY FULL`, созданы публикации `dbz_crm` и `dbz_telemetry` для логической репликации.
- В **docker-compose**: для `crm_db` и `telemetry_db` задан запуск с `wal_level=logical` и `max_replication_slots=8`.

На предыдущем этапе скрипты инициализации БД запускались в AirFlow отдельными DAG. Для полного перехода на CDC скрипты теперь монтируются в контейнер postgres.

### Debezium и Kafka

Добавлены сервисы **Zookeeper**, **Kafka**, **Debezium Connect**, **Kafka UI** (веб-интерфейс для просмотра топиков и сообщений: [http://localhost:8084](http://localhost:8084)):

- Скрипт **debezium/register-connectors.sh** регистрирует два коннектора:
  - **crm-connector** — таблицы `customers`, `orders` из CRM в топики `crm_db.public.*`.
  - **telemetry-connector** — таблица `telemetry_events` из телеметрии в топик `telemetry_db.public.telemetry_events`.
- Сервис **debezium-register** после старта Connect выполняет этот скрипт.

Логи регистрации коннекторов в `debezium-register`:
![debezium_1](/task4/asset/debezium_1.png)

Логи `debezium-connect` с отправкой данных в топики Kafka:
![debezium_2](/task4/asset/debezium_2.png)

Состояние топиков в Kafka:
![kafka_1](/task4/asset/kafka_1.png)

Сообщения в топике `crm_db.public.customers`:
![kafka_2](/task4/asset/kafka_2.png)

### 3. ClickHouse и KafkaEngine

Для формирования структуры таблиц и View в ClickHouse настроен сервис **ClickHouse** с init-скриптами в **clickhouse/init/**:

  - **01_create_db.sql** — база `reports`.
  - **02_kafka_tables.sql** — таблицы с движком Kafka для топиков `crm_db.public.customers` и `telemetry_db.public.telemetry_events`.
  - **03_merge_tree_tables.sql** — `customers_ch`, `telemetry_events_ch` (ReplacingMergeTree по версии из Debezium).
  - **04_materialized_views_cdc.sql** — MaterializedView для парсинга envelope Debezium из Kafka и вставки в `customers_ch` и `telemetry_events_ch`.
  - **05_report_mart.sql** — таблица `report_mart` и MaterializedView, которое обновляет таблицу при вставке в `customers_ch`.
  - **06_report_mart_telemetry_mv.sql** — MaterializedView при вставке в `telemetry_events_ch` обновляет витрину по затронутым пользователям.

Тестирование синхронизации данных (используется **DBeaver** для подключения к БД):

1. Изменяем `customers` в базе данных CRM
![sync_1](/task4/asset/sync_1.png)
Изменения поступают в **ClickHouse**:
![sync_2](/task4/asset/sync_2.png)
2. Изменяем `telemetry_events` в базе данных телеметрии:
![sync_3](/task4/asset/sync_3.png)
Изменения отражаются в **ClickHouse**:
![sync_4](/task4/asset/sync_4.png)

### 4. Витрина отчётности в ClickHouse

Для организации витрины отчётности в ClickHouse создана таблица **report_mart** — ReplacingMergeTree по `updated_at`, объединяет данные клиентов и агрегаты телеметрии (период последние 30 дней: `usage_hours`, `session_count`, `avg_daily_use`).

Заполняется двумя MaterializedView: при появлении данных в `customers_ch` и в `telemetry_events_ch`.

### 5. Переход API на новую витрину

Для построения отчётов из нового источника данных **reports-backend** переведён на чтение из ClickHouse:

- Конфиг: **CLICKHOUSE_DSN** (по умолчанию `clickhouse://default@clickhouse:9000/reports`).
- Репозиторий запрашивает `reports.report_mart FINAL` по `user_id` для `GetByUserID` и `GetPeriodByUserID`.
- В **docker-compose** у reports-backend задан **CLICKHOUSE_DSN** и зависимость от **clickhouse** (вместо olap_db).

Для тестирования скорректировано имя пользователя в базе CRM:
![reports_1](/task4/asset/reports_1.png)

После синхронизации отчёт отображает изменённую информацию из базы ClickHouse:
![reports_2](/task4/asset/reports_2.png)
