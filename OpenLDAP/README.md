# LDAP Admin

Учебная админ-панель для OpenLDAP: вход по паролю из каталога, просмотр дерева,
управление пользователями и группами. Бэкенд — Go, фронтенд — Vue 3 + Vuetify.

## Что нужно

Docker, Go 1.26+, Node.js 24+.

## Запуск

1. Каталог:
   ```bash
   docker compose up -d
   docker exec -i ldap-openldap-1 ldapadd -x -H ldap://localhost \
     -D cn=admin,dc=example,dc=com -w admin < seed.ldif
   ```
   `seed.ldif` создаёт `ou=people`, `ou=groups`, пользователя `alice` (пароль `alice-secret`)
   и группу `admins` с ним. Повторное применение падает с `Already exists (68)` — это нормально.

2. Бэкенд:
   ```bash
   cd backend
   cp .env.example .env
   make run            # http://localhost:8080
   ```

3. Фронтенд:
   ```bash
   cd frontend
   npm install
   npm run dev         # http://localhost:5173
   ```

4. Открыть http://localhost:5173 и войти как `alice` / `alice-secret`.

## Как посмотреть, что изменилось в каталоге

```bash
# все записи ветки ou=people
docker exec ldap-openldap-1 ldapsearch -x -H ldap://localhost -LLL \
  -D cn=admin,dc=example,dc=com -w admin -b ou=people,dc=example,dc=com

# группы пользователя: служебный атрибут memberOf нужно запросить явно
docker exec ldap-openldap-1 ldapsearch -x -H ldap://localhost -LLL \
  -D cn=admin,dc=example,dc=com -w admin -b uid=alice,ou=people,dc=example,dc=com -s base memberOf
```

## Тесты

```bash
cd backend && make test               # без LDAP
cd backend && make test-integration   # нужен запущенный контейнер
cd frontend && npm run test:run
```

## Сброс каталога

Данные живут внутри контейнера, без volume:

```bash
docker compose down && docker compose up -d
```

После этого снова примените `seed.ldif`.

## Настройки бэкенда

Переменные окружения (пример — `backend/.env.example`):

| Переменная | По умолчанию |
|---|---|
| `LDAP_ADMIN_HTTP_ADDRESS` | `:8080` |
| `LDAP_ADMIN_LDAP_URL` | `ldap://localhost:389` |
| `LDAP_ADMIN_LDAP_BIND_DN` | `cn=admin,dc=example,dc=com` |
| `LDAP_ADMIN_LDAP_BIND_PASSWORD` | обязательна |
| `LDAP_ADMIN_LDAP_BASE_DN` | `dc=example,dc=com` |
| `LDAP_ADMIN_ADMINS_GROUP` | `admins` |
| `LDAP_ADMIN_SESSION_TTL` | `8h` |

Устройство проекта и правила для разработки — в `CLAUDE.md` и
`docs/superpowers/specs/2026-10-07-ldap-admin-design.md`.
