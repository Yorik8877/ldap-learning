# samba-admin — учебная админ-панель Samba AD DC

Пользователь изучает Active Directory на практике. **Домен, сервисы, репозитории, слой LDAP,
тела ручек и middleware сессии он пишет сам.** Claude здесь подсказывает: объясняет устройство
AD и Samba, отвечает на вопросы, ревьюит. Код в этих местах Claude не пишет, пока пользователь
прямо не попросит. Фронтенд (позже) пишет Claude.

Спецификация — `docs/superpowers/specs/2026-10-07-samba-admin-design.md`, план каркаса —
`docs/superpowers/plans/2026-10-07-samba-admin-skeleton.md`.

## Команды

```bash
docker compose up -d --build     # Samba AD DC, LDAPS на localhost:636; первый старт ~1 мин — создаётся домен
./seed.sh                        # OU, svc-panel, PanelAdmins, alice; копирует CA в backend/certs/ca.pem

cd backend
make build | make vet | make test
make test-integration            # против контейнера Samba, с -race; без Samba тесты пропускаются
make run                         # читает backend/.env (пример — .env.example), слушает :8081
```

Учётные записи:

| Кто | Вход | Пароль |
|---|---|---|
| Пользователь админки (в `PanelAdmins`) | `alice` | `Alice-Secret1` |
| Сервисный аккаунт бэкенда | `CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com` | `Svc-Panel-Passw0rd` |
| Администратор домена | `Administrator@corp.example.com` | `Admin-Passw0rd` |

Посмотреть каталог руками:

```bash
docker compose exec samba sh -c 'LDAPTLS_CACERT=/var/lib/samba/private/panel-tls/ca.pem \
  ldapsearch -x -H ldaps://localhost -D svc-panel@corp.example.com -w Svc-Panel-Passw0rd \
  -b OU=Staff,DC=corp,DC=example,DC=com -LLL'
```

## Бэкенд: слои

Каркас: `cmd/samba-admin`, `internal/{config,app,server}`, `internal/api/{httpjson,auth,users,groups}`.
Ручки отвечают `501 not_implemented`, `auth.RequireSession` пока пропускает всех.

Правила те же, что в `../OpenLDAP/CLAUDE.md`:

| Слой | Каталог | Правило |
|---|---|---|
| 0. Домен | `internal/domain` | Только stdlib. Без `go-ldap`, `net/http`, JSON-тегов, `time.Now()`. |
| 1. Вход | `internal/api/*`, `internal/server` | Разбор JSON, вызов сервиса, ошибка → `httpjson.WriteError`. |
| 2. Сервисы | `internal/services` | Сценарии. Порты (интерфейсы) объявлены здесь. |
| 3. Репозитории | `internal/repos` | Домен ↔ записи LDAP, DN, имена атрибутов. Ошибки LDAP → доменные. |
| 4. Транспорт | `internal/db` | Тонкая обёртка над `go-ldap`. |

- Зависимости только внутрь. Сборка зависимостей — только в `internal/app`.
- Новая доменная ошибка → строка в `errorRules` (`internal/api/httpjson`). Статусы и коды — из контракта в спецификации.
- Текст ошибки драйвера не уходит в HTTP-ответ: неизвестная ошибка → 500 `internal`, текст в лог.
- Пароли — никогда в логах, ошибках и JSON.
- Контракт API (пути, поля, коды ошибок) зафиксирован в спецификации: по нему пишется фронтенд.

## Инфраструктура и её упрощения

- Образ — `samba/Dockerfile`: Debian trixie, Samba 4.22. Домен `corp.example.com`, NetBIOS `CORP`, хост `dc1`.
- Наружу опубликован только 636, и только на 127.0.0.1 (Docker обходит файрвол хоста, а пароль Domain Admin лежит в репозитории). OpenLDAP-проект занимает 389 — оба работают одновременно.
- Сертификат LDAPS выпускает `samba/entrypoint.sh` (свой CA и SAN `localhost`): сертификат,
  который выпускает сама Samba, без SAN, и Go его не примет.
- Контейнер работает без `--privileged`: ACL файлов Samba хранит в своей базе (`xattr_tdb`).
- `svc-panel` — участник `Domain Admins`. Это упрощение: в реальном домене сервисному аккаунту
  делегируют права только на нужные OU.
- Пароли `Administrator`, `svc-panel` и `alice` не истекают (`samba-tool user setexpiry --noexpiry`),
  иначе политика просрочит их через 42 дня. `seed.sh` выставляет это при каждом запуске.
- Сбросить домен целиком: `docker compose down -v`, затем `docker compose up -d --build` и `./seed.sh`.

## Особенности Samba, проверенные на этом контейнере

Проверено вручную (`ldapsearch`, `ldapmodify`, `samba-tool`) 2026-10-07 на Samba 4.22.11.

Подключение и вход:

- Простой bind по 389 без шифрования → код 8 (strongerAuthRequired). По LDAPS работает;
  StartTLS тоже, но 389 наружу не опубликован.
- Bind принимает UPN (`alice@corp.example.com`), `CORP\alice` и полный DN. Просто `alice` → 49.
- UPN работает, даже если атрибута `userPrincipalName` у записи нет: сервер подставляет
  `sAMAccountName@corp.example.com`.
- Неверный пароль и выключенная учётка — оба код 49. Различаются только текстом ошибки:
  `data 52e` — неверный пароль, `data 533` — учётка выключена.
- Bind с пустым паролем этот сервер отвергает (49), но по стандарту это анонимный вход —
  пустой пароль нужно отсекать до LDAP.

Пользователи:

- Минимум для создания через LDAP: `objectClass: user` и `sAMAccountName`; `cn` берётся из DN.
  `userPrincipalName` и `displayName` сервер сам не заполняет.
- Созданный так пользователь получает `userAccountControl: 546` (выключен + пароль не обязателен)
  и `primaryGroupID: 513` (Domain Users).
- Включить: сначала пароль, потом `userAccountControl: 512`. Выключенный — `514` (бит 2, ACCOUNTDISABLE).
  У учёток из `seed.sh` значение `66048` (512 + DONT_EXPIRE_PASSWORD) — меняй бит, а не всё число.
- `samba-tool user add` с `--given-name`/`--surname` делает `cn` = «Имя Фамилия». `cn` — это RDN:
  сменить его можно только операцией ModifyDN, обычный modify не даст.
- `sAMAccountName` уникален во всём домене: дубль в любой OU → 68 (`sAMAccountName ... already in use`).
  Дубль DN → тоже 68.
- Удаление несуществующей записи → 32.

Пароли:

- Password Modify (RFC 3062) не поддерживается → код 2, `Extended Operation(...) not supported`.
- Пароль ставится заменой атрибута `unicodePwd`: значение — пароль в двойных кавычках
  в кодировке UTF-16LE. Только по шифрованному соединению.
- Пароль не по политике → 19 (constraintViolation), в тексте `0000052D` и `complexity`.
- Политика по умолчанию: сложность включена, длина ≥ 7, история 24 пароля, срок жизни 42 дня.

Группы:

- Минимум для создания: `objectClass: group` и `sAMAccountName`. `groupType` по умолчанию
  `-2147483646` — глобальная группа безопасности.
- Группа без участников разрешена, удалять последнего участника можно.
- `member` с DN несуществующей записи → 32 (OpenLDAP такое принимал).
- Повторное добавление участника → 68 (в OpenLDAP было 20). Удаление не-участника → 16.
- При удалении пользователя сервер сам убирает его из `member` всех групп.
- `memberOf` у пользователя ведёт сервер. Основной группы (`Domain Users`, `primaryGroupID: 513`)
  в `memberOf` нет.

Поиск:

- `hasSubordinates` не поддерживается. Запрос `+` служебные атрибуты не возвращает — их просят по имени.
- Лимита на число записей в ответе нет (проверено на 1100). Постраничный поиск
  (control `1.2.840.113556.1.4.319`) поддерживается.
- Поиск поддеревом от `DC=corp,DC=example,DC=com` кроме записей возвращает search references
  (Configuration, DomainDnsZones, ForestDnsZones). Искать удобнее от `OU=Staff` и `OU=Groups`.
