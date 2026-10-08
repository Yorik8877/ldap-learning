# Ход работы над samba-admin

Документ для продолжения работы с любого места: самому или с другим агентом.
Обновлять при каждом заметном шаге. Последнее обновление — 2026-10-08, коммит `0707082` + перенос ошибок в домен.

## Формат работы

- Бэкенд пишет **автор проекта руками**, чтобы научиться работе с AD/LDAP. Агент объясняет,
  подсказывает следующий шаг и ревьюит; код в `internal/{domain,services,repos,db}` и тела ручек
  пишет только по прямой просьбе. Фронтенд потом пишет агент.
- Подавать материал небольшими порциями, по одному шагу за раз.
- Ревью — с запуском против живой Samba, а не только чтением кода.

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
- Интеграционные тесты клиента: `make test-integration` (нужна запущенная Samba, иначе тесты пропускаются).
- Поиск в `ldap_db` (пишет автор):
  - свои типы: `SearchRequest` (база, `Scope`, фильтр, атрибуты) и `Entry` (прячет `*ldap.Entry`,
    методы `DN()` и `Unmarshal(target)` — раскладка по структуре с тегами `ldap:"..."`);
  - `Search(request) ([]*Entry, error)` через `connection()`; пустой результат — пустой срез, не ошибка;
  - `FilterEquals(attribute, value)` — собирает `(attribute=value)` и экранирует значение;
  - свои ошибки (`errors.go`): код 49 → `ErrInvalidCredentials`, 32 → `ErrNoSuchObject`,
    200 → `ErrUnavailable` (в том числе при подключении); `translateError` переводит.
- Доменный тип `internal/domain/user`: `user.User` — `Login`, `FirstName`, `LastName`, `DisplayName`,
  `Email`, `Enabled`, `Groups`. Только stdlib, без тегов и без DN.
- Доменные ошибки `internal/domain/user/errors.go`: `user.ErrNotFound`, `user.ErrWrongLoginOrPassword`.
  Репозиторий возвращает их; в `user_repo` осталась только внутренняя `ErrTooManyUsersByLogin` (для HTTP — 500).
- `errorRules` в `internal/api/httpjson`: `user.ErrNotFound` → 404 `not_found`,
  `user.ErrWrongLoginOrPassword` → 401 `unauthorized`, обе с `detailed: false`.
- Репозиторий `internal/repos/user_repo` (пишет автор):
  - интерфейс `directory` (`Search`, `VerifyPassword`) объявлен в самом репозитории; `New(baseDN, client)`;
  - `userRecord` с тегами `ldap` и список `userAttributes`; `convertToDomain()` переводит в `user.User`:
    `Enabled` — бит `accountDisabledFlag` (2) в `userAccountControl` не установлен;
  - `FindByLogin(login) (user.User, error)`: поиск в `OU=Staff,<base DN>`, `ScopeOneLevel`,
    `FilterEquals("sAMAccountName", login)`; ноль записей → `ErrUserNotFound`, больше одной →
    `ErrTooManyUsersByLogin`. Проверено на живой Samba: `alice` находится, `nobody` и `*` → `ErrUserNotFound`,
    `svc-panel` не виден (он в `CN=Users`, а не в `OU=Staff`). Поиск записи вынесен в `findRecordByLogin`
    (возвращает `userRecord` с DN);
  - `Authenticate(login, password) (user.User, error)`: `findRecordByLogin` → `VerifyPassword(record.DN, password)`
    → `convertToDomain()`. Неверный или пустой пароль → `user.ErrWrongLoginOrPassword` (перевод `ldap_db`-ошибки
    через `errors.Is` в `translateError`), логин не найден → `user.ErrNotFound`. Проверено на живой Samba:
    `alice` и `ALICE` с верным паролем → логин `alice`; `nobody` и `svc-panel` → «не найден».

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

Сервис входа разбит на части; часть 1 (ошибки в домен) сделана.

1. **Группы именами, а не DN.** В конфиге `ADMIN_GROUP=PanelAdmins`, а в `user.Groups` лежит
   `CN=PanelAdmins,OU=Groups,...`. Резать DN по запятой нельзя (в имени бывает экранированная запятая);
   правильно разбирает DN только `go-ldap`, а он живёт в `ldap_db`.
2. **Сервис входа** (`internal/services`), пока без сессий: порт с `Authenticate` объявить в сервисе;
   `user.ErrNotFound` при входе заменять на `user.ErrWrongLoginOrPassword` (одинаковый 401, чтобы по ответу
   нельзя было подбирать логины); не в `ADMIN_GROUP` → 403.
3. **Сессии, ручки `Login`/`Me`/`Logout` и настоящий `RequireSession`.**
4. Дальше — пользователи и группы по контракту API, затем фронтенд.

## Отложенные замечания

- `NewClient` принимает четыре строки подряд — легко перепутать при вызове; можно заменить структурой
  `ldap_db.Config` с именованными полями (автор пока оставил как есть).
- Поле `certPath` нужно только в конструкторе, хранить его в клиенте незачем.
- `translateError` возвращает только свою ошибку и теряет текст сервера; `fmt.Errorf("%w: %w", ErrX, err)`
  сохранил бы подробности для лога.
- `convertToDomain` всегда возвращает `nil` в качестве ошибки — задел под `user.User` с конструктором;
  если конструктора не будет, ошибку из сигнатуры убрать.
- Сброс соединения при сетевой ошибке (`discard`) отложен: `go-ldap` сам помечает разорванное
  соединение (`IsClosing()`), и `connection()` переподключается.
- Нет таймаутов: подключения (`ldap.DialWithDialer(&net.Dialer{Timeout: ...})`) и запросов
  (`conn.SetTimeout`). Если Samba зависнет, запрос будет ждать вечно.
- `op` пишется руками и может разойтись с именем функции (один раз уже разошёлся). Решено оставить ручной `op`;
  если начнёт мешать — писать в обёртке действие (`"find user %q: %w"`), а не имя функции.
- `user_repo.translateError` — метод `Repo`, хотя `r` не использует; в `FindByLogin` переменная `userRecord`
  совпадает по имени с типом.
- В цепочке ошибок `VerifyPassword` дважды встречается «failed to dial».
- `ldap_db.ErrUnavailable` пока не доходит до HTTP как 503: `httpjson` не может импортировать `ldap_db`,
  нужна доменная (или общая) ошибка «каталог недоступен» и перевод в репозитории.
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
