# samba-admin — учебная админ-панель Samba AD DC

## Цель

Пользователь изучает Active Directory на практике: сам пишет на Go слои сервисов, репозиториев и
работы с LDAP для админ-панели поверх Samba AD DC. Claude готовит инфраструктуру и каркас, подсказывает
по устройству AD, а после бэкенда пишет фронтенд на Vue.

Успех: пользователь реализовал ручки каркаса своими сервисами и репозиториями, админка создаёт,
редактирует, включает/выключает и удаляет пользователей, создаёт и удаляет группы, управляет членством.

## Разделение работы

| Часть | Кто |
|---|---|
| Samba в Docker, начальные данные, проверка особенностей Samba | Claude |
| Каркас бэкенда: config, app, server, маршруты, httpjson, DTO, ручки-заглушки | Claude |
| Домен, сервисы, репозитории, слой LDAP, реализация ручек и middleware сессии | пользователь |
| Фронтенд | Claude, после бэкенда, по контракту API ниже |

## Раскладка репозитория

Корень репозитория содержит два независимых проекта: `OpenLDAP/` (готовая админка для OpenLDAP)
и `Samba/` (этот проект). `.gitignore` в корне общий.

```
Samba/
  docker-compose.yml
  samba/Dockerfile, samba/entrypoint.sh
  seed.sh
  backend/
  docs/superpowers/specs/
  CLAUDE.md, README.md
```

## Инфраструктура

- Образ собирается из `Samba/samba/Dockerfile`: `debian:trixie` + пакет `samba`. Готовые образы
  неофициальные, поэтому не используются.
- `entrypoint.sh` при первом старте выполняет `samba-tool domain provision`, при последующих —
  запускает `samba` в foreground. Данные — в именованном томе.
- Домен: realm `CORP.EXAMPLE.COM`, корень `DC=corp,DC=example,DC=com`, NetBIOS-имя `CORP`.
- Наружу публикуется только `636` (LDAPS). OpenLDAP занимает `389`, оба сервера работают одновременно.
- Цель — обойтись без `--privileged` (опция `--use-xattrs=no` у `provision`). Если не выйдет,
  это решение фиксируется в `CLAUDE.md` с причиной.
- Сертификат LDAPS самоподписанный, его выпускает Samba. `seed.sh` копирует CA в
  `Samba/backend/certs/ca.pem` (не в git), бэкенд доверяет ему через `LDAP_CA_FILE`.
- `seed.sh` через `samba-tool` в контейнере создаёт:
  - `OU=Staff` — пользователи админки, `OU=Groups` — группы админки. Админка работает только в них;
    встроенные учётки в `CN=Users` ей недоступны.
  - `svc-panel` — сервисный аккаунт, участник `Domain Admins`. Это упрощение: правильнее делегировать
    права только на две OU. Упрощение описано в `CLAUDE.md`.
  - группу `PanelAdmins` в `OU=Groups` и пользователя `alice` / `Alice-Secret1` в `OU=Staff`,
    участника `PanelAdmins`.
- Повторный запуск `seed.sh` не падает на уже созданных объектах.

### Проверка особенностей Samba

На поднятом контейнере Claude вручную проверяет и записывает в `Samba/CLAUDE.md`:

- простой bind без TLS отвергается, по LDAPS проходит;
- операция Password Modify (RFC 3062) не поддерживается; пароль ставится через `unicodePwd`;
- `userAccountControl` пользователя, созданного через LDAP без этого атрибута (ожидается 546);
- политика паролей по умолчанию;
- `memberOf` у пользователя и автоочистка `member` в группах при удалении пользователя;
- создание группы без участников и удаление последнего участника;
- основная группа (`primaryGroupID`) не видна в `memberOf`;
- поддержка `hasSubordinates`;
- лимит числа записей в одном ответе поиска.

Документация описывает факты (что сервер делает), а не готовый код слоёв пользователя.

## Каркас бэкенда

Модуль `samba-admin`, Go как в `OpenLDAP/backend` (`go 1.25.0` в `go.mod`), stdlib `net/http`
с шаблонами маршрутов, `log/slog`. Сторонних зависимостей в каркасе нет — `go-ldap` пользователь
добавит сам.

| Каталог | Содержимое |
|---|---|
| `cmd/samba-admin/main.go` | загрузка конфигурации, сборка через `app`, запуск сервера |
| `internal/config` | переменные `HTTP_ADDR`, `LDAP_URL`, `LDAP_CA_FILE`, `LDAP_BIND_DN`, `LDAP_BIND_PASSWORD`, `LDAP_BASE_DN`, `ADMIN_GROUP`, `SESSION_TTL`; ошибка при отсутствии обязательных |
| `internal/app` | composition root: создаёт обработчики и сервер; место, куда пользователь подключит свои сервисы |
| `internal/server` | `http.Server` с таймаутами, остановка по SIGINT/SIGTERM, регистрация маршрутов |
| `internal/api/httpjson` | чтение JSON (лимит размера, неизвестные поля — ошибка), ответ JSON, ответ-ошибка `{code, message}`, таблица «доменная ошибка → статус и код» (пустая; неизвестная ошибка → 500 `internal`, текст в лог, не в ответ), ответ `501 not_implemented` |
| `internal/api/auth` | DTO, ручки входа/выхода/текущего пользователя, middleware проверки сессии (заглушка, пропускает всех) |
| `internal/api/users`, `internal/api/groups` | DTO и ручки-заглушки |
| `internal/{domain,services,repos,db}` | пустые каталоги с `.gitkeep` |

`Makefile`: `build`, `vet`, `test`, `run` (читает `backend/.env`). `.env.example` с рабочими
значениями для контейнера.

Тесты каркаса:

- `httpjson`: чтение JSON (корректное тело, неизвестное поле, слишком большое тело), формат ошибки,
  неизвестная ошибка даёт 500 без текста исходной ошибки.
- маршруты: каждый маршрут контракта не отвечает 404 и 405. Тест остаётся верным после реализации ручек.

## Контракт API

Поля JSON в camelCase. `login` — `sAMAccountName`, `name` группы — её `cn`.

| Метод и путь | Тело | Ответ |
|---|---|---|
| `POST /api/auth/login` | `{login, password}` | `200 {login, displayName}` + cookie сессии |
| `POST /api/auth/logout` | — | `204` |
| `GET /api/auth/me` | — | `200 {login, displayName}` |
| `GET /api/users` | — | `200 [{login, displayName, email, enabled}]` |
| `POST /api/users` | `{login, firstName, lastName, displayName, email, password}` | `201` + объект как в `GET /api/users/{login}` |
| `GET /api/users/{login}` | — | `200 {login, firstName, lastName, displayName, email, enabled, groups: [name]}` |
| `PUT /api/users/{login}` | `{firstName, lastName, displayName, email}` | `200` + объект как в `GET /api/users/{login}` |
| `PUT /api/users/{login}/password` | `{password}` | `204` |
| `PUT /api/users/{login}/enabled` | `{enabled}` | `204` |
| `DELETE /api/users/{login}` | — | `204` |
| `GET /api/groups` | — | `200 [{name, description, memberCount}]` |
| `POST /api/groups` | `{name, description}` | `201` + объект как в `GET /api/groups/{name}` |
| `GET /api/groups/{name}` | — | `200 {name, description, members: [{login, displayName}]}` |
| `DELETE /api/groups/{name}` | — | `204` |
| `PUT /api/groups/{name}/members/{login}` | — | `204` |
| `DELETE /api/groups/{name}/members/{login}` | — | `204` |

Все маршруты, кроме `POST /api/auth/login`, требуют сессию. Группы — глобальные группы безопасности.

Ошибки — `{code, message}`:

| Статус | `code` | Когда |
|---|---|---|
| 400 | `invalid_request` | некорректное тело или параметр |
| 401 | `unauthorized` | нет сессии или неверные учётные данные |
| 403 | `forbidden` | пользователь не в `ADMIN_GROUP` |
| 404 | `not_found` | объекта нет |
| 409 | `already_exists` | объект уже существует |
| 422 | `password_policy` | пароль не прошёл политику домена |
| 501 | `not_implemented` | ручка не реализована |
| 503 | `unavailable` | LDAP недоступен |
| 500 | `internal` | прочее |

## Документация

`Samba/CLAUDE.md` и `Samba/README.md`: команды, правила слоёв (как в `OpenLDAP/CLAUDE.md`),
проверенные особенности Samba, упрощения инфраструктуры. Без готовых решений для слоёв пользователя.

## Вне рамок

- Фронтенд — отдельный шаг после бэкенда.
- ACL на записи (`nTSecurityDescriptor`), группы рассылки, области групп кроме глобальной.
- Браузер дерева каталога.
- Kerberos, DNS, вход в Windows.
