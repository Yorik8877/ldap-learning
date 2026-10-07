# LDAP Admin — дизайн учебной админ-панели

Дата: 2026-10-07. Статус: на ревью.

## 1. Цель и границы

Учебный проект: показать работу с OpenLDAP из кода на живых операциях протокола.
Админ-панель — бэкенд на Go, фронтенд на Vue — умеет:

- входить по паролю пользователя из каталога (LDAP bind) с проверкой членства в группе `admins`;
- обходить дерево каталога и показывать атрибуты любой записи;
- создавать, редактировать, удалять пользователей и менять им пароль;
- создавать и удалять группы, добавлять и убирать участников.

**Критерий успеха:** после применения `seed.ldif` можно войти как `alice`, увидеть дерево, создать
пользователя, добавить его в группу — и тот же `ldapsearch` из терминала показывает новые записи
и атрибут `memberOf` у пользователя.

**Вне границ (сознательно):** продакшн-готовность (HTTPS, хранение сессий вне памяти,
горизонтальное масштабирование), переименование записей (`modrdn`), запись произвольных записей
через браузер дерева, управление схемой и ACL сервера, i18n, тесты компонентов фронтенда.

Бэкенд разложен по луковичной архитектуре сознательно, хотя для учебного проекта это избыточно:
цель — заодно показать, как LDAP-источник встраивается в слои.

## 2. Каталог

### 2.1 Сервер

`docker-compose.yml` остаётся как есть: образ `osixia/openldap:1.5.0`, домен `example.com`,
base DN `dc=example,dc=com`, администратор каталога `cn=admin,dc=example,dc=com` / `admin`,
порт 389 без TLS.

Особенности образа, на которые опирается дизайн (проверено на чистом контейнере того же образа):

| Что | Как ведёт себя | Что из этого следует |
| --- | --- | --- |
| Оверлей `memberof` | Настроен на `groupOfUniqueNames` / `uniqueMember`. При добавлении пользователя в такую группу сервер сам пишет в его запись `memberOf: <DN группы>`. Атрибут служебный: править нельзя, в обычном поиске не возвращается — только по явному запросу. | Группы — `groupOfUniqueNames`. Группы пользователя читаются из его `memberOf`. |
| Оверлей `refint` | Следит за `member`, `uniqueMember`, `memberOf`, `owner`, `manager`. При удалении записи сам удаляет её DN из этих атрибутов в других записях. | Бэкенд не чистит группы при удалении пользователя. |
| `refint` и последний участник | Если удалённый пользователь был единственным участником группы, удаление проходит, а в группе остаётся DN несуществующей записи. Ошибки нет. | Перед удалением пользователя сервис проверяет, нет ли групп, где он единственный участник, и отказывает. |
| Удаление последнего значения `uniqueMember` вручную | Ошибка `65 objectClassViolation`: `uniqueMember` обязателен в `groupOfUniqueNames`. | Домен запрещает убрать последнего участника заранее; код 65 — страховка. |
| `uniqueMember` и несуществующий DN | Синтаксис атрибута проверяет только формат DN, не существование записи. | Сервис проверяет, что пользователь существует, до добавления в группу. |
| Bind с DN и пустым паролем | Сервер отвечает `53 unwilling to perform` (unauthenticated bind запрещён). | Пустой пароль отсекается в домене всё равно: на других серверах такой bind проходит как анонимный и даёт «успех». |
| Password Modify (RFC 3062) | Сервер сохраняет пароль хэшем `{SSHA}`. | Пароли задаются только этой операцией, а не записью `userPassword` напрямую. |
| ACL | `cn=admin` пишет всё; пользователь читает только себя; `userPassword` доступен только для `auth`. | Все операции бэкенд выполняет от `cn=admin`; пользовательский bind — только проверка пароля. |

### 2.2 Структура дерева

```
dc=example,dc=com
├── cn=admin                      # администратор каталога (создан образом)
├── ou=people
│   └── uid=<uid>                 # objectClass: inetOrgPerson
└── ou=groups
    └── cn=<name>                 # objectClass: groupOfUniqueNames
```

### 2.3 Начальные данные — `seed.ldif`

Файл в корне проекта, применяется вручную командой из README:

```bash
docker exec -i ldap-openldap-1 ldapadd -x -H ldap://localhost \
  -D cn=admin,dc=example,dc=com -w admin < seed.ldif
```

Содержимое:

- `ou=people,dc=example,dc=com` и `ou=groups,dc=example,dc=com` (`organizationalUnit`);
- `uid=alice,ou=people,…` — `inetOrgPerson`, `cn: Alice Admin`, `sn: Admin`,
  `mail: alice@example.com`, `userPassword` — хэш `{SSHA}` пароля `alice-secret`, заранее
  полученный через `slappasswd` в контейнере;
- `cn=admins,ou=groups,…` — `groupOfUniqueNames`, `description: Admin panel access`,
  `uniqueMember: uid=alice,ou=people,…`.

Повторное применение падает с `68 entryAlreadyExists` — это ожидаемо. Сброс каталога —
пересоздание контейнера (`docker compose down && docker compose up -d`), данные не в volume.

## 3. Раскладка репозитория

```
ldap/
├── docker-compose.yml
├── seed.ldif
├── CLAUDE.md
├── README.md
├── docs/superpowers/specs/      # этот документ
├── backend/                     # Go-модуль `ldap-admin`
└── frontend/                    # Vue 3 + Vite
```

## 4. Бэкенд

Go 1.26, модуль `ldap-admin` (bare path, как в matrix-sso-backend). Зависимости: `go-ldap/ldap/v3`
и stdlib (`net/http`, `log/slog`). Своего роутера нет: `http.ServeMux` с шаблонами
`GET /api/users/{uid}`.

### 4.1 Слои

Зависимости направлены только внутрь. Порт объявляется в пакете, который им пользуется.

```
backend/
├── cmd/ldap-admin/main.go           # вызывает app.Main()
└── internal/
    ├── app/                         # composition root: конфиг → ldap_db → repos → services → api
    ├── config/                      # загрузка env LDAP_ADMIN_*
    ├── domain/                      # слой 0
    │   ├── directory/               # DN, Node, Entry
    │   ├── user/                    # User, UID, правила пароля, ошибки
    │   ├── group/                   # Group, Member, инварианты участников, ошибки
    │   └── session/                 # Session, ошибки входа
    ├── api/                         # слой 1
    │   ├── auth/  users/  groups/  directory/
    │   └── httpjson/                # запись JSON-ответов и ошибок
    ├── server/                      # маршруты, middleware сессии и логирования, http.Server
    ├── services/                    # слой 2
    │   ├── auth_service/  user_service/  group_service/  directory_service/
    ├── repos/                       # слой 3
    │   ├── user_repo/  group_repo/  directory_repo/  session_repo/
    └── db/ldap_db/                  # слой 4
```

### 4.2 Слой 0 — домен

Только stdlib. Без тегов JSON, без `go-ldap`, без `time.Now()`.

**`directory`**
- `DN` — строковый тип. Домен не разбирает DN: синтаксис DN (экранирование, регистр) — знание
  источника, оно в слое 4.
- `Node{DN, RDN, ObjectClasses []string, HasChildren bool}` — узел дерева.
- `Entry{DN, Attributes, OperationalAttributes map[string][]string}` — запись целиком; обычные и
  служебные атрибуты разделены, чтобы в интерфейсе было видно, что ведёт сервер.
- Ошибки: `ErrNotFound`, `ErrOutsideBase`.

**`user`**
- `UID` — строковый тип, `ParseUID` проверяет формат `^[a-z][a-z0-9._-]{0,63}$`.
- `User{UID, CommonName, Surname, Emails []string}`; `New(uid, commonName, surname, emails)`
  проверяет: `cn` и `sn` непустые после обрезки пробелов и не длиннее 256 символов; каждый
  email разбирается `net/mail.ParseAddress`.
- `ValidatePassword(password)` — непустой, не короче 8 символов.
- Ошибки: `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalid` (оборачивается с пояснением поля:
  `fmt.Errorf("%w: surname is required", user.ErrInvalid)`), `ErrSelfDelete`.

**`group`**
- `Name` — строковый тип, формат как у `UID`.
- `Member{DN directory.DN, UID user.UID}` — `UID` пустой, если участник вне `ou=people`
  (например, `cn=admin`).
- `Group{Name, Description, Members []Member}`; `New` требует минимум одного участника.
- `(Group) CheckRemoval(uid) error` — `ErrNotMember`, если такого участника нет; `ErrLastMember`,
  если он последний.
- Ошибки: `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalid`, `ErrLastMember`, `ErrNotMember`,
  `ErrAlreadyMember`, `ErrProtected`, `ErrSoleMember` — тип `SoleMemberError{Groups []Name}`
  (через `errors.As`), чтобы ответ назвал группы, мешающие удалению пользователя.

**`session`**
- `Session{ID, UID, CommonName, ExpiresAt time.Time}`; `(Session) IsExpired(now time.Time) bool`.
- `Clock` (`Now() time.Time`) и `IDGenerator` (`NewID() (string, error)`) — интерфейсы.
- Ошибки: `ErrInvalidCredentials`, `ErrNotAdmin`, `ErrNotFound` (сессии нет или истекла).

### 4.3 Слой 2 — сервисы и порты

Порты перечислены у сервиса, который их объявляет. Реализации — в репозиториях.

**`auth_service.Service`**
- `Login(ctx, uid, password) (session.Session, error)`:
  1. `user.ParseUID`, `user.ValidatePassword` — иначе `ErrInvalidCredentials` (не `ErrInvalid`:
     ответ не должен подсказывать, что именно не так);
  2. `PasswordVerifier.Verify(ctx, uid, password)` — нет пользователя или неверный пароль →
     `ErrInvalidCredentials`;
  3. `AdminChecker.IsMember(ctx, adminsGroup, uid)` — `false` → `ErrNotAdmin`;
  4. `UserReader.Get` — за `cn` для ответа;
  5. сессия: `IDGenerator.NewID()`, `ExpiresAt = Clock.Now() + ttl`, `SessionStore.Save`.
- `Authenticate(ctx, id) (session.Session, error)` — нет или истекла → `session.ErrNotFound`;
  истёкшую удаляет.
- `Logout(ctx, id) error`.
- Порты: `PasswordVerifier`, `AdminChecker`, `UserReader`, `SessionStore`, `session.Clock`,
  `session.IDGenerator`. Имя группы админов и TTL приходят в конструктор.

**`user_service.Service`**
- `List`, `Get(uid) (user.User, []group.Name, error)`, `Create(user, password)`,
  `Update(user)`, `SetPassword(uid, password)`.
- `Delete(ctx, actorUID, uid)`:
  1. `actorUID == uid` → `ErrSelfDelete`;
  2. `GroupReader.GroupsOf(uid)`; группы, где пользователь — единственный участник →
     `SoleMemberError`;
  3. `UserStore.Delete`. Из остальных групп его уберёт `refint`.
- `Create`: сначала `UserStore.Create`, затем `UserStore.SetPassword`. Если пароль не задался,
  запись удаляется обратно, ошибка возвращается. Транзакций между записями в OpenLDAP по
  умолчанию нет, компенсация — явная.
- Порты: `UserStore`, `GroupReader`.

**`group_service.Service`**
- `List`, `Get`, `Create(group)`, `Delete(name)`, `AddMember(name, uid)`,
  `RemoveMember(name, uid)`.
- `Create` и `AddMember` проверяют через `UserReader`, что пользователи существуют:
  `uniqueMember` примет DN несуществующей записи.
- `RemoveMember`: `GroupStore.Get` → `group.CheckRemoval` → `GroupStore.RemoveMember`.
- `Delete` группы админов → `ErrProtected`.
- Порты: `GroupStore`, `UserReader`. Имя группы админов — в конструктор.

**`directory_service.Service`**
- `Children(ctx, dn *directory.DN)` — `nil` означает base DN; `Entry(ctx, dn)`.
- Порт: `Browser`. Сервис тонкий: бизнес-правил у просмотра нет, слой нужен ради единообразия.

### 4.4 Слой 3 — репозитории

Знают, как домен ложится на LDAP: DN, `objectClass`, имена атрибутов. Ошибки транспорта
превращают в доменные. `go-ldap` не импортируют — работают через `ldap_db`.

| Репозиторий | Реализует | Как |
| --- | --- | --- |
| `user_repo` | `UserStore`, `UserReader`, `PasswordVerifier` | DN `uid=<uid>,ou=people,<base>`; `objectClass: inetOrgPerson`; `cn`, `sn`, `uid`, `mail`. `Update` — modify **replace** `cn`, `sn`, `mail` (пустой список `mail` — удаление атрибута). `Verify` — `ldap_db.VerifyPassword` по DN. |
| `group_repo` | `GroupStore`, `GroupReader`, `AdminChecker` | DN `cn=<name>,ou=groups,<base>`; `objectClass: groupOfUniqueNames`; `uniqueMember`. `AddMember` / `RemoveMember` — modify **add** / **delete** одного значения. `GroupsOf` — читает `memberOf` пользователя (запрошен явно), затем сами группы. `IsMember` — операция **Compare** `uniqueMember=<DN пользователя>` на записи группы. `uid` участника выводится из DN, если тот лежит прямо в `ou=people`. |
| `directory_repo` | `Browser` | `Children` — поиск `scope=one` с атрибутами `objectClass`, `hasSubordinates`. `Entry` — `scope=base` с атрибутами `*` и `+` (все обычные и все служебные); `userPassword` вырезается. DN вне base → `directory.ErrOutsideBase`. |
| `session_repo` | `SessionStore` | `map` под `sync.Mutex`. Транспортного слоя у памяти нет. |

Маппинг ошибок `ldap_db` → домен (каждый репозиторий — в свои доменные ошибки):

| Ошибка `ldap_db` | Код LDAP | Домен |
| --- | --- | --- |
| `ErrNoSuchObject` | 32 | `ErrNotFound` своего пакета |
| `ErrAlreadyExists` | 68 | `ErrAlreadyExists` |
| `ErrObjectClassViolation` | 65 | `group.ErrLastMember` при удалении участника, иначе `ErrInvalid` |
| `ErrValueExists` | 20 | `group.ErrAlreadyMember` |
| `ErrNoSuchValue` | 16 | `group.ErrNotMember` |
| `ErrInvalidCredentials` | 49 | `session.ErrInvalidCredentials` |
| `ErrUnavailable` | сеть, 51, 52 | пробрасывается как есть → 503 |

### 4.5 Слой 4 — `ldap_db`

Тонкая обёртка над `go-ldap`. На каждую операцию: соединение → bind сервисным аккаунтом →
операция → закрытие. Это медленнее пула, но нет устаревших соединений и вопроса «под кем сейчас
bind». Дедлайн `ctx` переносится в таймаут соединения.

- `Search(ctx, SearchRequest{BaseDN, Scope, Filter, Attributes}) ([]Entry, error)`
- `Add(ctx, dn, attributes map[string][]string) error`
- `Modify(ctx, dn, []Change{Operation, Attribute, Values}) error` — операции `Add`, `Delete`,
  `Replace`
- `Delete(ctx, dn) error`
- `Compare(ctx, dn, attribute, value) (bool, error)`
- `SetPassword(ctx, dn, password) error` — расширенная операция Password Modify
- `VerifyPassword(ctx, dn, password) error` — bind пользователем на отдельном соединении
  без сервисного bind
- Построители, скрывающие экранирование: `FilterEquals(attribute, value)` (`ldap.EscapeFilter`),
  `RDN(attribute, value)` (`ldap.EscapeDN`), `IsWithin(dn, base) bool` (разбор DN `ldap.ParseDN`)
- Свои sentinel-ошибки (таблица выше), чтобы слой 3 не зависел от `go-ldap`.

Пароли не попадают в ошибки и логи.

### 4.6 Слой 1 — HTTP

- Хендлеры разбирают JSON, вызывают сервис, маппят ответ в DTO и доменную ошибку — в код.
- Middleware `RequireSession` читает cookie, вызывает `auth_service.Authenticate`, кладёт сессию
  в `context`. Middleware логирования пишет метод, путь, статус, длительность через `slog`.
- `http.Server` с таймаутами чтения и записи; остановка по SIGINT/SIGTERM через
  `signal.NotifyContext` и `Shutdown`.

### 4.7 Конфиг и composition root

Переменные окружения; пример — `backend/.env.example`. `make run` подгружает `backend/.env`.

| Переменная | По умолчанию |
| --- | --- |
| `LDAP_ADMIN_HTTP_ADDRESS` | `:8080` |
| `LDAP_ADMIN_LDAP_URL` | `ldap://localhost:389` |
| `LDAP_ADMIN_LDAP_BIND_DN` | `cn=admin,dc=example,dc=com` |
| `LDAP_ADMIN_LDAP_BIND_PASSWORD` | нет, обязательна — секрет не зашивается в код |
| `LDAP_ADMIN_LDAP_BASE_DN` | `dc=example,dc=com` |
| `LDAP_ADMIN_ADMINS_GROUP` | `admins` |
| `LDAP_ADMIN_SESSION_TTL` | `8h` |

`internal/app` собирает зависимости руками; отсутствие обязательной переменной — падение
при старте с понятным сообщением (допустимо только здесь). Доступность LDAP на старте не
проверяется: каталог может подняться позже, запросы до этого получат 503.

## 5. HTTP API

Префикс `/api`. Всё, кроме `POST /auth/login`, требует сессию. Тела — JSON.

### 5.1 Вход

| Метод и путь | Запрос → ответ |
| --- | --- |
| `POST /auth/login` | `{uid, password}` → `200 {uid, cn}` и cookie |
| `POST /auth/logout` | → `204`, cookie сбрасывается |
| `GET /auth/me` | → `200 {uid, cn}` |

Cookie `ldap_admin_session`: `HttpOnly`, `SameSite=Strict`, `Path=/api`, срок равен TTL сессии.
ID — 32 случайных байта из `crypto/rand` в base64url. Сессии в памяти, перезапуск бэкенда
разлогинивает всех. Членство в `admins` проверяется только при входе: исключённый из группы
сохраняет доступ до конца сессии — принято для учебного проекта.

### 5.2 Пользователи

| Метод и путь | Запрос → ответ |
| --- | --- |
| `GET /users` | → `200 [{uid, cn, sn, mail[]}]`, по `uid` |
| `GET /users/{uid}` | → `200 {uid, cn, sn, mail[], groups[]}` |
| `POST /users` | `{uid, cn, sn, mail[], password}` → `201` пользователь |
| `PUT /users/{uid}` | `{cn, sn, mail[]}` → `200` пользователь |
| `PUT /users/{uid}/password` | `{password}` → `204` |
| `DELETE /users/{uid}` | → `204` |

### 5.3 Группы

| Метод и путь | Запрос → ответ |
| --- | --- |
| `GET /groups` | → `200 [{cn, description, memberCount}]`, по `cn` |
| `GET /groups/{cn}` | → `200 {cn, description, members: [{dn, uid}]}` (`uid` пустой для участников вне `ou=people`) |
| `POST /groups` | `{cn, description, members: [uid]}` → `201` группа |
| `DELETE /groups/{cn}` | → `204` |
| `POST /groups/{cn}/members` | `{uid}` → `204` |
| `DELETE /groups/{cn}/members/{uid}` | → `204` |

### 5.4 Дерево

| Метод и путь | Ответ |
| --- | --- |
| `GET /directory/children?dn=…` | `200 [{dn, rdn, objectClass[], hasChildren}]`; без `dn` — потомки base DN |
| `GET /directory/entry?dn=…` | `200 {dn, attributes: {name: [values]}, operationalAttributes: {…}}` |

Корневой узел дерева (сам base DN) фронтенд получает через `GET /directory/entry` без `dn`.

### 5.5 Ошибки

Тело: `{"error": "<code>", "message": "<текст>"}`. `message` — текст доменной ошибки на
английском; детали драйвера наружу не уходят.

| HTTP | `error` | Доменная ошибка |
| --- | --- | --- |
| 400 | `invalid_input` | `ErrInvalid`, `directory.ErrOutsideBase`, битый JSON |
| 401 | `unauthenticated` | `session.ErrNotFound`, нет cookie |
| 401 | `invalid_credentials` | `session.ErrInvalidCredentials` — одинаково для «нет uid» и «неверный пароль» |
| 403 | `not_admin` | `session.ErrNotAdmin` |
| 404 | `not_found` | `ErrNotFound` любого пакета, `group.ErrNotMember` |
| 409 | `already_exists` | `ErrAlreadyExists`, `group.ErrAlreadyMember` |
| 409 | `last_member` | `group.ErrLastMember` |
| 409 | `sole_member` | `group.SoleMemberError`, в ответе дополнительно `groups: [cn]` |
| 409 | `protected` | `group.ErrProtected` |
| 409 | `self_delete` | `user.ErrSelfDelete` |
| 503 | `directory_unavailable` | `ldap_db.ErrUnavailable` |
| 500 | `internal` | остальное; подробности — только в лог |

## 6. Фронтенд

Облегчённые соглашения matrix-customer-frontend.

- Vue 3 (`<script setup lang="ts">`, strict), Vite, Pinia, Vue Router, axios, Vuetify
  (`VTreeview` с ленивой подгрузкой детей). Без ui-kit-обёрток, i18n, Service Worker.
- Модули `src/features/{auth,directory,users,groups}/` с публичным `index.ts`.
- API: `src/common/services/api/` — один axios-клиент, репозиторий на домен, агрегат
  `apiService`. Перехватчик: `401` → страница входа; остальные ошибки отдаются вызывающему.
- Vite проксирует `/api` на `http://localhost:8080` — cookie работает без CORS.
- Роутер: `meta.publicAccess` у страницы входа; guard проверяет сессию через `GET /auth/me`.

Экраны:

- **Вход** — `uid`, пароль; ошибки `invalid_credentials` и `not_admin` показываются по-разному.
- **Дерево** — слева `VTreeview` (дети грузятся при раскрытии, иконка-стрелка только при
  `hasChildren`), справа две таблицы атрибутов выбранной записи: обычные и служебные.
- **Пользователи** — таблица; диалоги создания, редактирования, смены пароля, подтверждения
  удаления. Карточка пользователя показывает его группы (из `memberOf`).
- **Группы** — список, диалог создания (выбор участников из пользователей). Карточка группы:
  участники, добавление (выбор из пользователей), удаление.

Ошибки бэкенда — snackbar с `message`; для `sole_member` — список групп.

## 7. Тесты

**Бэкенд** (`make test` — без LDAP и сети):
- домен: правила `ParseUID`, `user.New`, `ValidatePassword`, `group.New`, `CheckRemoval`,
  `Session.IsExpired`;
- сервисы на фейках портов: каждый сценарий раздела 4.3, включая отказы (неверный пароль,
  не админ, удаление себя, единственный участник, несуществующий пользователь в группе,
  защищённая группа, откат создания при ошибке пароля);
- хендлеры через `httptest` с фейковыми сервисами: разбор ввода, маппинг ошибок раздела 5.5,
  установка и сброс cookie.

**Интеграция** (`make test-integration`, build-тег `integration`):
- `ldap_db` и репозитории против контейнера из `docker-compose.yml`; если LDAP недоступен —
  `t.Skip`. Каждый тест работает во временной ветке `ou=test-<random>,<base>` и удаляет её
  в `t.Cleanup`. Проверяется в том числе поведение раздела 2.1: появление `memberOf`, работа
  `refint`, коды 65, 20, 16, 49.

**Фронтенд** (`npm run test:run`, Vitest): репозитории API (маппинг ответов и ошибок), stores,
composables. Компоненты не тестируются, порогов покрытия нет.

## 8. CLAUDE.md

Самодостаточный, без ссылок на другие репозитории (сабмодуля `.ai` здесь нет). Содержит:

- назначение проекта и сознательный выбор луковицы;
- слои, их запреты, правила портов, ошибок и именования (кратко, по мотивам `.ai/backend/*`
  экосистемы Matrix);
- особенности каталога из раздела 2.1 — то, обо что легко споткнуться;
- команды запуска и тестов, раскладку фронтенда.

## 9. Допущения и риски

- Поведение оверлеев проверено вручную на `osixia/openldap:1.5.0`. Смена образа или его
  конфигурации может его изменить — интеграционные тесты это поймают.
- Сохранение `memberOf` при modify add/delete `uniqueMember` (а не только при создании группы)
  предполагается по документации оверлея, вручную не проверялось — проверяется интеграционным
  тестом.
- Compare по `uniqueMember` (правило сравнения `uniqueMemberMatch`) вручную не проверялся —
  проверяется интеграционным тестом `group_repo.IsMember`.
- Соединение на операцию ограничивает производительность — для учебного проекта приемлемо.
- Между созданием записи пользователя и установкой пароля есть окно, в котором пользователь
  существует без пароля. Войти в это время нельзя (bind без пароля запрещён), запись удаляется,
  если пароль не задался.
