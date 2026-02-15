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

---

## Разработка сервиса отчётов





---

## Снижение нагрузки на базу данных




---

## Повышение оперативности и стабильности работы CRM


