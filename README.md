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


---

## Разработка сервиса отчётов





---

## Снижение нагрузки на базу данных




---

## Повышение оперативности и стабильности работы CRM


