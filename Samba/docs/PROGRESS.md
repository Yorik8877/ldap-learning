# Ход работы над samba-admin

Документ для продолжения работы с любого места: самому или с другим агентом.
Обновлять при каждом заметном шаге. Последнее обновление — 2026-10-09, коммит `61d3dad` (сессии в сервисе входа) + тесты к нему.

## Формат работы

- Бэкенд пишет **автор проекта руками**, чтобы научиться работе с AD/LDAP. Агент объясняет,
  подсказывает следующий шаг и ревьюит; код в `internal/{domain,services,repos,db}` и тела ручек
  пишет только по прямой просьбе. Фронтенд потом пишет агент.
- Подавать материал небольшими порциями, по одному шагу за раз.
- Ревью — с запуском против живой Samba, а не только чтением кода.
- Когда код автора принят, агент дописывает в репозиторий тест ровно по тем случаям, на которых проверял
  (unit — без тега, против Samba — `//go:build integration`). Наперёд тесты не пишутся. Коммиты раздельные:
  код — автор, тесты к нему — агент, следующим коммитом.

## Где мы сейчас

Готово:

- Samba AD DC в Docker, `seed.sh`, каркас бэкенда: маршруты контракта API отвечают `501`.
  Подробности — `Samba/CLAUDE.md` и спецификация `docs/superpowers/specs/2026-10-07-samba-admin-design.md`.
- Слой транспорта `internal/db/ldap_db` (пишет автор):
  - `NewClient(ldapURL, certPath, bindDN, bindPassword)` — читает CA, собирает `tls.Config`,
    сразу подключается от сервисного аккаунта `svc-panel`;
  - `dial(bindDN, password)` — LDAPS-соединение + `Bind` с любыми учётными данными, без сохранения;
    пустой DN или пароль → `ErrInvalidCredentials` до запроса к серверу (иначе это анонимный bind);
    при неудачном `Bind` закрывает соединение сам — у вызывающего его нет;
  - `connection()` — под мьютексом отдаёт живое общее соединение или переподключается и сохраняет новое;
  - `Close()` — под мьютексом;
  - `VerifyPassword(bindDN, password)` — `dial` + `Close` на отдельном соединении, общее не трогает;
    неверный или пустой пароль → `ErrInvalidCredentials`. Проверено: после проверки пароля общее
    соединение по-прежнему работает от `CORP\svc-panel`.
- `internal/app/dbconnect.go`: `initLDAP` создаёт клиент; `app.run` закрывает его через `defer`.
- `CommonName(dn)` (`ldap_db/dn.go`) — значение `CN` первой части DN через `ldap.ParseDN`
  (экранирование снимает сам: `Smith\, John` → `Smith, John`). Ноль частей (пустая строка, строка из пробелов,
  в том числе Unicode) → `ErrEmptyDNGiven`: `ParseDN` такое ошибкой не считает, поэтому проверяется длина `RDNs`,
  а не входная строка. Первая часть без `CN` → `ErrNoCommonName`.
- Тесты (`make test` — unit, `make test-integration` — против Samba, без неё пропускаются):
  `ldap_db` — соединение и переподключение, `VerifyPassword`, `CommonName` (`dn_test.go`);
  `user_repo` — `FindByLogin`, `Authenticate` (с группами), `convertToDomain` (`user_record_test.go`);
  `auth_service` — `Login`, `Current`, `Logout` без Samba: подставные пользователи, генератор ID и часы,
  настоящее `session_repo` (`fakes_test.go`, `login_test.go`, `session_test.go`, `logout_test.go`);
  `httpjson` — статусы и коды всех доменных ошибок, `message` без цепочки `op`.
- Сервис входа `internal/services/auth_service` (пишет автор):
  - порт `UserAuthenticator` (`Authenticate(login, password)`) объявлен в сервисе, `user_repo.Repo` подходит
    под него неявно; `New(users, adminGroup)`; в `app` пока не подключён — подключим вместе с ручкой `Login`;
  - порты `SessionStore` (под него подходит `session_repo`), `IDGenerator` (`NewID() (string, error)`),
    `Clock` (`Now()`); `New(users, sessionStore, idGenerator, clock, adminGroup, sessionTTL)`;
  - `Login(login, password) (session.Session, error)`: сначала пароль, потом группа (иначе 403 без пароля выдал бы,
    что логин существует); `user.ErrNotFound` → `user.ErrWrongLoginOrPassword` (одинаковый 401);
    нет `ADMIN_GROUP` в `Groups` (сравнение через `strings.EqualFold`) → `user.ErrNoAdminPrivilege`;
    затем ID от генератора, сессия `{ID, Login, DisplayName, ExpiresAt = now + TTL}` сохраняется и возвращается
    (ручке нужны ID для cookie и имя для ответа); при любой неудаче ничего не сохраняется;
  - `Current(id)`: `Find` (ошибки хранилища — как есть); истёкшая удаляется → `session.ErrExpired`;
    `Logout(id)` — `Delete`;
  - `errorRules`: `user.ErrNoAdminPrivilege` → 403 `forbidden`.
- Доменный тип сессии `internal/domain/session` (пишет автор): `Session{ID, Login, DisplayName, ExpiresAt}`;
  `ID` — непрозрачная строка (формат решает генератор, не домен); `IsExpired(now)` — `!now.Before(ExpiresAt)`,
  ровно в `ExpiresAt` уже истекла; `session.ErrNotFound` и `session.ErrExpired` — обе → 401 `unauthorized`
  (разделены, чтобы в логе было видно, какой случай).
  Тест — `session_test.go`.
- Хранилище сессий `internal/repos/session_repo` (пишет автор): `map[string]session.Session` под `sync.Mutex`;
  `Save` (повторный — перезапись), `Find` (нет → `session.ErrNotFound`), `Delete` (нет — не ошибка).
  Истечение не проверяет — это делает сервис. Тест — `session_repo_test.go` (одновременный доступ ловится с `-race`).
- Поиск в `ldap_db` (пишет автор):
  - свои типы: `SearchRequest` (база, `Scope`, фильтр, атрибуты) и `Entry` (прячет `*ldap.Entry`,
    методы `DN()` и `Unmarshal(target)` — раскладка по структуре с тегами `ldap:"..."`);
  - `Search(request) ([]*Entry, error)` через `connection()`; пустой результат — пустой срез, не ошибка;
  - `FilterEquals(attribute, value)` — собирает `(attribute=value)` и экранирует значение;
  - свои ошибки (`errors.go`): код 49 → `ErrInvalidCredentials`, 32 → `ErrNoSuchObject`,
    200 → `ErrUnavailable` (в том числе при подключении); `translateError` переводит.
- Доменный тип `internal/domain/user`: `user.User` — `Login`, `FirstName`, `LastName`, `DisplayName`,
  `Email`, `Enabled`, `Groups` (имена групп, `cn`). Только stdlib, без тегов и без DN.
- Доменные ошибки `internal/domain/user/errors.go`: `user.ErrNotFound`, `user.ErrWrongLoginOrPassword`.
  Репозиторий возвращает их; в `user_repo` осталась только внутренняя `ErrTooManyUsersByLogin` (для HTTP — 500).
- `errorRules` в `internal/api/httpjson`: `user.ErrNotFound` → 404 `not_found`,
  `user.ErrWrongLoginOrPassword` → 401 `unauthorized`, обе с `detailed: false`.
- Репозиторий `internal/repos/user_repo` (пишет автор):
  - интерфейс `directory` (`Search`, `VerifyPassword`) объявлен в самом репозитории; `New(baseDN, client)`;
  - `userRecord` с тегами `ldap` и список `userAttributes`; `convertToDomain()` переводит в `user.User`:
    `Enabled` — бит `accountDisabledFlag` (2) в `userAccountControl` не установлен; `Groups` — имена:
    `groupNames(dns)` переводит DN из `memberOf` через `ldap_db.CommonName`, битый DN → ошибка `convertToDomain`,
    без групп → пустой срез (в JSON будет `[]`, а не `null`);
  - `FindByLogin(login) (user.User, error)`: поиск в `OU=Staff,<base DN>`, `ScopeOneLevel`,
    `FilterEquals("sAMAccountName", login)`; ноль записей → `user.ErrNotFound`, больше одной →
    `ErrTooManyUsersByLogin`. Проверено на живой Samba: `alice` находится, `nobody` и `*` → «не найден»,
    `svc-panel` не виден (он в `CN=Users`, а не в `OU=Staff`). Поиск записи вынесен в `findRecordByLogin`
    (возвращает `userRecord` с DN);
  - `Authenticate(login, password) (user.User, error)`: `findRecordByLogin` → `VerifyPassword(record.DN, password)`
    → `convertToDomain()`. Неверный или пустой пароль → `user.ErrWrongLoginOrPassword` (перевод `ldap_db`-ошибки
    через `errors.Is` в `translateError`), логин не найден → `user.ErrNotFound`. Проверено на живой Samba:
    `alice` и `ALICE` с верным паролем → логин `alice`, `Groups: [PanelAdmins]`; `nobody` и `svc-panel` → «не найден».

Принятые решения:

- **Общее соединение + короткие для входа.** Одно долгоживущее соединение от `svc-panel` для всей
  работы; проверка пароля пользователя — на отдельном коротком соединении (`dial` + `Close`), потому что
  `Bind` меняет пользователя, от имени которого работает соединение.
- Мьютекс защищает только указатель `c.conn` (проверить и заменить), не саму операцию:
  `*ldap.Conn` из `go-ldap` сам выполняет параллельные запросы.
- **Свои типы в `ldap_db`, схема — в репозитории.** `go-ldap` не выходит за пределы `ldap_db`;
  `ldap_db` не знает про пользователей и группы. Что где лежит в каталоге (OU, имена атрибутов,
  биты `userAccountControl`) знает только репозиторий.
- **Все рабочие операции — от `svc-panel`.** От имени пользователя — только `Bind` для проверки пароля.
- **Перевод LDAP → домен делает репозиторий**: домен (слой 0) не знает формат хранения. Когда в домене
  появится конструктор с проверками (`user.New`), перевод вызывает его.
- **Репозиторий возвращает значение `user.User`, не указатель.** «Не найден» — только ошибкой, никаких `(nil, nil)`.
- **DN в домен не попадает.** Проверку пароля по логину делает репозиторий: сам находит DN и делает `Bind`.
- **Ошибки, на которые реагируют сервис или HTTP, живут в домене**: сервис не импортирует репозиторий.
- **`detailed: true` в `errorRules` — только для ошибок без внутренностей** (сейчас лишь `ErrMalformedBody`):
  иначе клиент получает всю цепочку обёрток с именами функций (`user_repo.Authenticate: ...`).
- **Ошибки сравнивать только через `errors.Is`/`errors.As`**: каждый слой оборачивает ошибку через `%w`,
  поэтому `==` и `switch err` сравнивают внешнюю обёртку и не срабатывают.
- **Выключенная учётка при `Bind` даёт тот же код 49** — отдельно проверять `Enabled` при входе не нужно.
- **Логин в AD не зависит от регистра**: `ALICE` находит `alice`. Дальше по коду использовать логин
  из найденного пользователя, а не введённый (сессии, проверка «нельзя удалить себя»).

## Следующие шаги

Сервис входа (без сессий) готов. Сессии разбиты на части; части 1 (доменный тип), 2 (хранилище) и 3 (сервис) сделаны.

1. **HTTP**: ручки `Login`/`Me`/`Logout`, cookie, настоящий `RequireSession`; собрать цепочку в `app`:
   `ldap_db` → `user_repo` → `auth_service` → `auth.Handler` (сейчас `initLDAP` отдаёт только `Close()`);
   настоящие часы и генератор ID (`crypto/rand`, например 32 байта в base64) — реализации портов для `app`.
2. Дальше — пользователи и группы по контракту API, затем фронтенд.

## Отложенные замечания

- `NewClient` принимает четыре строки подряд — легко перепутать при вызове; можно заменить структурой
  `ldap_db.Config` с именованными полями (автор пока оставил как есть).
- Поле `certPath` нужно только в конструкторе, хранить его в клиенте незачем.
- `translateError` возвращает только свою ошибку и теряет текст сервера; `fmt.Errorf("%w: %w", ErrX, err)`
  сохранил бы подробности для лога.
- Сброс соединения при сетевой ошибке (`discard`) отложен: `go-ldap` сам помечает разорванное
  соединение (`IsClosing()`), и `connection()` переподключается.
- Нет таймаутов: подключения (`ldap.DialWithDialer(&net.Dialer{Timeout: ...})`) и запросов
  (`conn.SetTimeout`). Если Samba зависнет, запрос будет ждать вечно.
- `op` пишется руками и может разойтись с именем функции (один раз уже разошёлся). Решено оставить ручной `op`;
  если начнёт мешать — писать в обёртке действие (`"find user %q: %w"`), а не имя функции.
- `user_repo.translateError` и `auth_service.translateLoginError` — методы, хотя получатель не используют.
- `auth_service.New` принимает шесть параметров подряд — легко перепутать; можно собрать порты в структуру.
- В цепочке ошибок `VerifyPassword` дважды встречается «failed to dial».
- `ldap_db.ErrUnavailable` пока не доходит до HTTP как 503: `httpjson` не может импортировать `ldap_db`,
  нужна доменная (или общая) ошибка «каталог недоступен» и перевод в репозитории.
- Вход проверяет только **прямое** членство в `ADMIN_GROUP` (через `memberOf`). Участник подгруппы
  `PanelAdmins` для AD — участник `PanelAdmins`, а для панели — нет. Если понадобится вложенность:
  правило поиска по цепочке `1.2.840.113556.1.4.1941`, например фильтр
  `(memberOf:1.2.840.113556.1.4.1941:=<DN группы>)` (на нашей Samba не проверялось).
- Истёкшие сессии, к которым больше не обращаются, остаются в памяти `session_repo` до перезапуска
  (сервис удаляет истёкшую только при обращении к ней). Нужна периодическая чистка, если это станет проблемой.
- Сессия не перепроверяет членство в `ADMIN_GROUP`: убранный из группы админ работает до конца `SESSION_TTL`.
- Мелочи из ревью каркаса: лог Samba не виден в `docker compose logs` (нет `--debug-stdout`);
  `server.Run` пишет «listening» до занятия порта; нет теста, что `RequireSession` оборачивает маршруты.

## Новая машина: как продолжить

```bash
git clone git@github.com:Yorik8877/ldap-learning.git && cd ldap-learning && git switch samba-admin
cd Samba
docker compose up -d --build   # новый домен и новый CA
./seed.sh                      # обязательно: перезаписывает backend/certs/ca.pem под новый CA
cd backend
cp .env.example .env
make vet test test-integration
```

`backend/certs/ca.pem` и `backend/.env` в git не хранятся. Старый `ca.pem` с другой машины
к новому контейнеру не подойдёт: «certificate signed by unknown authority».

**Пушить после каждого коммита** — один раз работа уже потерялась при переезде из-за
незапушенного коммита.
