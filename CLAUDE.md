# ldap-admin — учебная админ-панель OpenLDAP

Проект для знакомства с LDAP: бэкенд на Go и фронтенд на Vue поверх OpenLDAP из `docker-compose.yml`.
Спецификация — `docs/superpowers/specs/2026-10-07-ldap-admin-design.md`, план —
`docs/superpowers/plans/2026-10-07-ldap-admin.md`.

Бэкенд намеренно разложен по луковичной архитектуре, хотя для учебного проекта это избыточно:
цель — заодно показать, как LDAP-источник встраивается в слои. Не «упрощай» слои.

## Команды

```bash
docker compose up -d                       # OpenLDAP на localhost:389
# начальные данные (один раз на свежий контейнер):
docker exec -i ldap-openldap-1 ldapadd -x -H ldap://localhost -D cn=admin,dc=example,dc=com -w admin < seed.ldif

cd backend
make build | make vet | make test          # test — без LDAP
make test-integration                      # против контейнера; без LDAP тесты сами пропускаются
make run                                   # читает backend/.env (пример — .env.example)

cd frontend
npm run dev                                # http://localhost:5173, /api проксируется на :8080
npm run test:run | npm run typecheck | npm run build
```

Вход: `alice` / `alice-secret` (участник `cn=admins`).

## Бэкенд: слои

Зависимости — только внутрь. Порт (интерфейс) объявляется в пакете, который им пользуется.

| Слой | Каталог | Правило |
|---|---|---|
| 0. Домен | `internal/domain/{directory,user,group,session}` | Только stdlib. Без `go-ldap`, `net/http`, JSON-тегов, `time.Now()`. Инварианты — в конструкторах (`user.New`, `group.New`). |
| 1. Вход | `internal/api/*`, `internal/server` | Разбор JSON, вызов сервиса, ошибка → код через `httpjson.WriteError`. |
| 2. Сервисы | `internal/services/*_service` | Сценарии. Порты объявлены здесь. Транзакций и LDAP здесь нет. |
| 3. Репозитории | `internal/repos/*_repo`, `tree_layout` | Домен ↔ записи LDAP, DN, имена атрибутов. Ошибки `ldap_db` → доменные. |
| 4. Транспорт | `internal/db/ldap_db` | Тонкая обёртка над `go-ldap`, экранирование DN и фильтров. |

- Сборка зависимостей — только в `internal/app` (composition root). Глобальных синглтонов нет.
- Именование: пакет `x_service`/`x_repo`/`x_db`, внутри — `Service`/`Repo`/`Client`; конструктор `New`.
- Доменные ошибки — sentinel `ErrXxx` или тип `XxxError`; сравнение через `errors.Is`/`errors.As`.
  Новая доменная ошибка → строка в таблице `errorRules` в `internal/api/httpjson`.
- Текст ошибки драйвера не уходит в HTTP-ответ: 5xx отвечают общим текстом и пишутся в лог.
- Пароли — никогда в логах, ошибках и JSON. Пароль задаётся только через `ldap_db.SetPassword`
  (операция Password Modify); писать `userPassword` напрямую нельзя — он сохранится открытым текстом.
- Тесты: домен и сервисы — на фейках портов (`make test`); репозитории и `ldap_db` —
  интеграционные с build-тегом `integration` во временной ветке `ou=test-<random>` (`ldapstand`).

## Особенности каталога, о которые легко споткнуться

Проверены на образе `osixia/openldap:1.5.0`; интеграционные тесты `ldap_db` их фиксируют.

- Пользователи — `inetOrgPerson` в `ou=people`, группы — `groupOfUniqueNames` в `ou=groups`
  (участники в `uniqueMember`). Не `groupOfNames`: оверлей `memberof` образа настроен только
  на `groupOfUniqueNames`.
- `memberOf` у пользователя ведёт сервер; это служебный атрибут — его нужно запрашивать по имени.
- Оверлей `refint` при удалении пользователя сам убирает его DN из групп. Если пользователь был
  **единственным** участником группы, сервер удалит запись, но оставит в группе битую ссылку
  без ошибки — поэтому `user_service.Delete` такое удаление запрещает.
- Группа без участников запрещена схемой: удаление последнего `uniqueMember` → код 65.
- `uniqueMember` принимает DN несуществующей записи — существование проверяет сервис.
- Bind с пустым паролем — анонимный вход (здесь сервер его отвергает, другие серверы — нет);
  пустой пароль отсекается до похода в LDAP.
- `cn=admin,dc=example,dc=com` — rootdn из конфигурации сервера, записи в дереве у него нет.
- Транзакций между записями нет: создание пользователя (запись + пароль) откатывается вручную.

## Фронтенд

Vue 3 + Vuetify 4 + Pinia + Vue Router. Модули — `src/features/{auth,directory,users,groups}`,
наружу только через `index.ts`. Запросы — через `apiService` (`src/common/services/api`):
репозиторий на домен, DTO (имена атрибутов LDAP) → модели (`models.ts`). Ошибки бэкенда
приходят как `ApiError` с `code` из тела ответа. Логику (репозитории, stores, composables,
помощники) покрываем Vitest; компоненты не тестируем.
