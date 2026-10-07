# LDAP Admin Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Учебная админ-панель для OpenLDAP: вход через LDAP bind, браузер дерева, управление пользователями и группами.

**Architecture:** Бэкенд на Go по луковице: домен (слой 0) → HTTP-хендлеры (1) → сервисы с портами (2) → репозитории, знающие раскладку дерева (3) → тонкий транспорт `ldap_db` поверх `go-ldap` (4). Сборка зависимостей — руками в `internal/app`. Фронтенд — Vue 3 + Vuetify, фичевые модули, слой API через репозитории; Vite проксирует `/api` на бэкенд.

**Tech Stack:** Go 1.26, `github.com/go-ldap/ldap/v3` v3.4.14, stdlib `net/http` и `log/slog`; Vue 3.5, Vuetify 4, Pinia 3, Vue Router 5, axios, Vite 8, Vitest 4, TypeScript 6.

**Spec:** `docs/superpowers/specs/2026-10-07-ldap-admin-design.md` — читать вместе с планом.

## Global Constraints

- Пути ниже — относительно каталога `OpenLDAP/`. Бэкенд — `backend/`, фронтенд — `frontend/`.
- Go-модуль называется `ldap-admin` (bare path), импорты — `ldap-admin/internal/...`.
- Слои: зависимости только внутрь. В `internal/domain/` — только stdlib: без `go-ldap`, `net/http`, `slog`, JSON-тегов, `time.Now()`.
- Порты объявляются в пакете-потребителе (сервисе), реализуются в репозиториях. Конструкторы — `New(...)`.
- Пакеты — snake_case с суффиксом слоя: `user_service`, `user_repo`, `ldap_db`. Типы внутри — `Service`, `Repo`, `Client`.
- Каталог: base DN `dc=example,dc=com`, пользователи `uid=<uid>,ou=people,<base>` (`inetOrgPerson`), группы `cn=<name>,ou=groups,<base>` (`groupOfUniqueNames`, участники в `uniqueMember`).
- Пароли задаются только операцией Password Modify, никогда не записываются в `userPassword` напрямую. Пароли не попадают в логи, ошибки и JSON-ответы.
- Сообщения ошибок и логов — на английском; комментарии в коде — на русском и только там, где код не объясняет «почему».
- Имена без сокращений (`connection`, а не `conn`; `request`, а не `req`), без однобуквенных переменных; вложенность в функции — не больше трёх уровней.
- `make test` в `backend/` проходит без LDAP. Интеграционные тесты — build-тег `integration`, запуск `make test-integration`, без поднятого LDAP они сами пропускаются.
- Конфиг — переменные окружения `LDAP_ADMIN_*` (раздел 4.7 спецификации); `LDAP_ADMIN_LDAP_BIND_PASSWORD` обязательна, значения по умолчанию у неё нет.
- Коммит после каждой задачи. Сообщение коммита заканчивается строкой `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

## Отличия от спецификации, принятые при планировании

- `directory.ErrUnavailable` — доменная ошибка недоступности каталога. Репозитории переводят в неё `ldap_db.ErrUnavailable`, иначе слою 1 пришлось бы импортировать слой 4.
- `directory.ErrInvalidDN` — DN с битым синтаксисом в query-параметре даёт 400, а не 500.
- При входе пароль проверяется только на непустоту, а не на минимальную длину: иначе пользователь с коротким паролем, заведённым в обход админки, не смог бы войти. Минимальная длина действует при создании пользователя и смене пароля.
- `group_service.Create` возвращает созданную группу, перечитанную из каталога: DN участников знает только репозиторий.
- Записи в `ou=people` и `ou=groups`, не проходящие доменные правила (например, `uid` с заглавными буквами, заведённый через `ldapadd`), не попадают в списки и не ломают их; их видно в браузере дерева.

## Review Focus

1. **Записи, созданные в обход админки.** Пользователь `uid=Bob` или группа без `description`, заведённые через `ldapadd`. Ожидание: списки пользователей и групп загружаются, «чужая» запись просто отсутствует в списке. Тест — в задачах 5 и 6.
2. **Повтор участника при создании группы** (`members: ["alice", "alice"]`). Ожидание: 400 `invalid_input`, а не 500 из-за кода LDAP 20. Тест — в задаче 4 (домен) и задаче 13 (хендлер).
3. **Спецсимволы во вводе, попадающем в DN и фильтры.** `cn: "Doe, John"` сохраняется как есть; `?dn=` с битым синтаксисом даёт 400; `uid` вида `a*)(uid=*` отвергается до похода в LDAP. Тесты — в задачах 2, 5, 7, 8.
4. **LDAP недоступен или неверный пароль сервисного аккаунта.** Ожидание: 503 `directory_unavailable` в первом случае, 500 `internal` во втором; текст ошибки драйвера не уходит в ответ. Тесты — в задачах 2 и 11.
5. **Вход с `uid` в другом регистре (`Alice`) или пустым паролем.** Ожидание: 401 `invalid_credentials`, без похода в LDAP и без 500. Тест — в задаче 8.

## Структура файлов

```
ldap/
├── seed.ldif                                   # задача 1
├── .gitignore                                  # задача 1
├── CLAUDE.md, README.md                        # задача 19
├── backend/
│   ├── go.mod, go.sum, Makefile, .env.example
│   ├── cmd/ldap-admin/main.go                  # задача 14
│   └── internal/
│       ├── app/app.go, runtime.go              # задача 14: composition root, Clock, IDGenerator
│       ├── config/config.go                    # задача 14
│       ├── domain/directory/directory.go       # задача 3
│       ├── domain/user/{user,errors}.go        # задача 3
│       ├── domain/group/{group,errors}.go      # задача 4
│       ├── domain/session/session.go           # задача 4
│       ├── db/ldap_db/{client,errors,names}.go # задача 2
│       ├── testsupport/ldapstand/ldapstand.go  # задача 2: временная ветка для интеграционных тестов
│       ├── repos/tree_layout/layout.go         # задача 5: где в дереве лежат пользователи и группы
│       ├── repos/user_repo/repo.go             # задача 5
│       ├── repos/group_repo/repo.go            # задача 6
│       ├── repos/directory_repo/repo.go        # задача 7
│       ├── repos/session_repo/repo.go          # задача 7
│       ├── services/auth_service/service.go    # задача 8
│       ├── services/user_service/service.go    # задача 9
│       ├── services/group_service/service.go   # задача 10
│       ├── services/directory_service/service.go # задача 10
│       ├── api/httpjson/httpjson.go            # задача 11: JSON и маппинг ошибок
│       ├── api/sessionctx/sessionctx.go        # задача 11: сессия в context
│       ├── api/auth/handler.go                 # задача 11
│       ├── api/users/handler.go                # задача 12
│       ├── api/groups/handler.go               # задача 13
│       ├── api/directory/handler.go            # задача 13
│       └── server/{routes,server}.go           # задача 14
└── frontend/
    ├── package.json, tsconfig.json, vite.config.ts, index.html, env.d.ts
    └── src/
        ├── main.ts, App.vue
        ├── libs/vuetify.ts
        ├── router/{index,guards,types}.ts
        ├── layouts/DefaultLayout.vue
        ├── views/HomeView.vue
        ├── components/{AppNotifier,ConfirmDialog}.vue
        ├── stores/notifier.store.ts
        ├── common/services/api/
        │   ├── api-error.ts, http-client.ts, models.ts, dtos.ts, api.service.ts
        │   └── repositories/{auth,users,groups,directory}.repository.ts
        └── features/
            ├── auth/       # задача 15
            ├── directory/  # задача 16
            ├── users/      # задача 17
            └── groups/     # задача 18
```

---
### Task 1: Начальные данные каталога

**Files:**
- Create: `seed.ldif`
- Create: `.gitignore`
- Commit: `docker-compose.yml` (уже лежит в корне, ещё не в git)

**Interfaces:**
- Consumes: контейнер `ldap-openldap-1` из `docker-compose.yml` (должен быть запущен: `docker compose up -d`).
- Produces: в каталоге есть `ou=people`, `ou=groups`, пользователь `uid=alice` с паролем `alice-secret`, группа `cn=admins` с участником `alice`. Все следующие задачи и ручная проверка опираются на эту учётку.

- [ ] **Step 1: Убедиться, что структуры ещё нет**

Run:
```bash
docker exec ldap-openldap-1 ldapsearch -x -H ldap://localhost -LLL \
  -D cn=admin,dc=example,dc=com -w admin -b dc=example,dc=com -s one dn
```
Expected: пустой вывод (у корня нет потомков). Если `ou=people` уже есть — seed применялся раньше, перейти к шагу 4.

- [ ] **Step 2: Создать `seed.ldif`**

Хэш получен командой `docker exec ldap-openldap-1 slappasswd -s alice-secret`.

```ldif
# Начальная структура каталога для админ-панели.
# Применение: docker exec -i ldap-openldap-1 ldapadd -x -H ldap://localhost \
#   -D cn=admin,dc=example,dc=com -w admin < seed.ldif
# Пароль alice: alice-secret

dn: ou=people,dc=example,dc=com
objectClass: organizationalUnit
ou: people

dn: ou=groups,dc=example,dc=com
objectClass: organizationalUnit
ou: groups

dn: uid=alice,ou=people,dc=example,dc=com
objectClass: inetOrgPerson
uid: alice
cn: Alice Admin
sn: Admin
mail: alice@example.com
userPassword: {SSHA}HRCWpbXgZuZY1Xy73dbIOQMl3BvO0PIB

dn: cn=admins,ou=groups,dc=example,dc=com
objectClass: groupOfUniqueNames
cn: admins
description: Admin panel access
uniqueMember: uid=alice,ou=people,dc=example,dc=com
```

- [ ] **Step 3: Применить seed**

Run:
```bash
docker exec -i ldap-openldap-1 ldapadd -x -H ldap://localhost \
  -D cn=admin,dc=example,dc=com -w admin < seed.ldif
```
Expected: четыре строки `adding new entry "..."`, без ошибок.

- [ ] **Step 4: Проверить вход alice и атрибут memberOf**

Run:
```bash
docker exec ldap-openldap-1 ldapwhoami -x -H ldap://localhost \
  -D uid=alice,ou=people,dc=example,dc=com -w alice-secret
docker exec ldap-openldap-1 ldapsearch -x -H ldap://localhost -LLL \
  -D cn=admin,dc=example,dc=com -w admin \
  -b uid=alice,ou=people,dc=example,dc=com -s base memberOf
```
Expected: `dn:uid=alice,ou=people,dc=example,dc=com` и строка `memberOf: cn=admins,ou=groups,dc=example,dc=com`.

- [ ] **Step 5: Создать `.gitignore`**

```gitignore
.claude/
backend/.env
frontend/node_modules/
frontend/dist/
```

- [ ] **Step 6: Commit**

```bash
git add docker-compose.yml seed.ldif .gitignore
git commit -m "Add LDAP seed data and gitignore

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Транспорт `ldap_db` и проверка поведения сервера

**Files:**
- Create: `backend/go.mod` (через `go mod init`), `backend/go.sum`
- Create: `backend/Makefile`
- Create: `backend/internal/db/ldap_db/client.go`
- Create: `backend/internal/db/ldap_db/errors.go`
- Create: `backend/internal/db/ldap_db/names.go`
- Create: `backend/internal/testsupport/ldapstand/ldapstand.go`
- Test: `backend/internal/db/ldap_db/names_test.go`
- Test: `backend/internal/db/ldap_db/errors_test.go`
- Test: `backend/internal/db/ldap_db/client_test.go`
- Test: `backend/internal/db/ldap_db/client_integration_test.go`

**Interfaces:**
- Consumes: ничего из предыдущих задач в коде; интеграционные тесты — контейнер из задачи 1.
- Produces (пакет `ldap-admin/internal/db/ldap_db`):
  - `type Config struct { URL, BindDN, BindPassword string }`; `func New(config Config) *Client`
  - `type Scope int` с `ScopeBase`, `ScopeOneLevel`, `ScopeSubtree`
  - `type SearchRequest struct { BaseDN string; Scope Scope; Filter string; Attributes []string }` (пустой `Filter` = `(objectClass=*)`)
  - `type Entry struct { DN string; Attributes map[string][]string }`; `func (Entry) Values(attribute string) []string`; `func (Entry) First(attribute string) string` — поиск имени без учёта регистра
  - `type ChangeOperation int` с `ChangeAdd`, `ChangeDelete`, `ChangeReplace`; `type Change struct { Operation ChangeOperation; Attribute string; Values []string }`
  - `func (*Client) Search(ctx, SearchRequest) ([]Entry, error)`
  - `func (*Client) Add(ctx, dn string, attributes map[string][]string) error`
  - `func (*Client) Modify(ctx, dn string, changes []Change) error`
  - `func (*Client) Delete(ctx, dn string) error`
  - `func (*Client) Compare(ctx, dn, attribute, value string) (bool, error)`
  - `func (*Client) SetPassword(ctx, dn, password string) error`
  - `func (*Client) VerifyPassword(ctx, dn, password string) error`
  - `func FilterEquals(attribute, value string) string`, `func RDN(attribute, value string) string`, `func Join(rdn, parentDN string) string`
  - `func IsWithin(dn, base string) (bool, error)`, `func FirstRDN(dn string) (string, error)`, `func ChildValue(dn, attribute, parentDN string) (string, bool)`
  - Ошибки: `ErrNoSuchObject`, `ErrAlreadyExists`, `ErrObjectClassViolation`, `ErrValueExists`, `ErrNoSuchValue`, `ErrInvalidCredentials`, `ErrUnavailable`, `ErrServiceBind`, `ErrInvalidDN`
- Produces (пакет `ldap-admin/internal/testsupport/ldapstand`):
  - `type Stand struct { Client *ldap_db.Client; Config ldap_db.Config; Base string }`
  - `func Connect(t *testing.T) Stand` — пропускает тест, если LDAP недоступен; создаёт временную ветку `ou=test-<random>` и удаляет её после теста
  - `func (Stand) CreateOU(t, name string) string`, `func (Stand) CreatePerson(t, parentDN, uid string) string`, `func (Stand) CreateGroup(t, parentDN, name string, memberDNs ...string) string`

- [ ] **Step 1: Инициализировать модуль и Makefile**

Run:
```bash
mkdir -p backend && cd backend && go mod init ldap-admin && go get github.com/go-ldap/ldap/v3@v3.4.14
```
Expected: `go.mod` с `module ldap-admin` и `go 1.26`, `go.sum` создан.

`backend/Makefile`:
```make
.PHONY: build vet test test-integration run

build:
	go build ./...

vet:
	go vet ./...
	go vet -tags integration ./...

test:
	go test ./...

test-integration:
	go test -tags integration -count=1 ./...

run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/ldap-admin
```

- [ ] **Step 2: Написать падающие юнит-тесты построителей имён**

`backend/internal/db/ldap_db/names_test.go`:
```go
package ldap_db_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/db/ldap_db"
)

func TestFilterEqualsEscapesFilterSyntax(t *testing.T) {
	got := ldap_db.FilterEquals("uid", "a*)(uid=*")
	want := `(uid=a\2a\29\28uid=\2a)`
	if got != want {
		t.Fatalf("FilterEquals() = %q, want %q", got, want)
	}
}

func TestRDNEscapesDNSyntax(t *testing.T) {
	got := ldap_db.RDN("cn", "Doe, John")
	want := `cn=Doe\, John`
	if got != want {
		t.Fatalf("RDN() = %q, want %q", got, want)
	}
}

func TestIsWithin(t *testing.T) {
	const base = "dc=example,dc=com"
	cases := []struct {
		name string
		dn   string
		want bool
	}{
		{name: "base itself in another case", dn: "DC=Example,DC=Com", want: true},
		{name: "direct child", dn: "ou=people,dc=example,dc=com", want: true},
		{name: "deep descendant", dn: "uid=alice,ou=people,dc=example,dc=com", want: true},
		{name: "other tree", dn: "cn=config", want: false},
		{name: "empty dn", dn: "", want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ldap_db.IsWithin(testCase.dn, base)
			if err != nil {
				t.Fatalf("IsWithin() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("IsWithin(%q) = %v, want %v", testCase.dn, got, testCase.want)
			}
		})
	}
}

func TestIsWithinRejectsMalformedDN(t *testing.T) {
	_, err := ldap_db.IsWithin("not a dn", "dc=example,dc=com")
	if !errors.Is(err, ldap_db.ErrInvalidDN) {
		t.Fatalf("IsWithin() error = %v, want ErrInvalidDN", err)
	}
}

func TestFirstRDN(t *testing.T) {
	got, err := ldap_db.FirstRDN("uid=alice,ou=people,dc=example,dc=com")
	if err != nil {
		t.Fatalf("FirstRDN() error = %v", err)
	}
	if got != "uid=alice" {
		t.Fatalf("FirstRDN() = %q, want %q", got, "uid=alice")
	}
}

func TestChildValue(t *testing.T) {
	const people = "ou=people,dc=example,dc=com"
	cases := []struct {
		name      string
		dn        string
		wantValue string
		wantFound bool
	}{
		{name: "direct child", dn: "uid=alice,ou=people,dc=example,dc=com", wantValue: "alice", wantFound: true},
		{name: "other attribute", dn: "cn=alice,ou=people,dc=example,dc=com", wantFound: false},
		{name: "outside parent", dn: "cn=admin,dc=example,dc=com", wantFound: false},
		{name: "grandchild", dn: "uid=x,ou=nested,ou=people,dc=example,dc=com", wantFound: false},
		{name: "malformed", dn: "garbage", wantFound: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value, found := ldap_db.ChildValue(testCase.dn, "uid", people)
			if found != testCase.wantFound || value != testCase.wantValue {
				t.Fatalf("ChildValue() = (%q, %v), want (%q, %v)", value, found, testCase.wantValue, testCase.wantFound)
			}
		})
	}
}
```

`backend/internal/db/ldap_db/client_test.go`:
```go
package ldap_db_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/db/ldap_db"
)

func TestUnreachableServerIsUnavailable(t *testing.T) {
	client := ldap_db.New(ldap_db.Config{URL: "ldap://127.0.0.1:1", BindDN: "cn=admin", BindPassword: "secret"})

	_, err := client.Search(t.Context(), ldap_db.SearchRequest{BaseDN: "dc=example,dc=com", Scope: ldap_db.ScopeBase})

	if !errors.Is(err, ldap_db.ErrUnavailable) {
		t.Fatalf("Search() error = %v, want ErrUnavailable", err)
	}
}

func TestVerifyPasswordRejectsEmptyPasswordWithoutNetwork(t *testing.T) {
	client := ldap_db.New(ldap_db.Config{URL: "ldap://127.0.0.1:1"})

	err := client.VerifyPassword(t.Context(), "uid=alice,ou=people,dc=example,dc=com", "")

	if !errors.Is(err, ldap_db.ErrInvalidCredentials) {
		t.Fatalf("VerifyPassword() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestEntryValuesIgnoresAttributeNameCase(t *testing.T) {
	entry := ldap_db.Entry{Attributes: map[string][]string{"memberOf": {"cn=admins"}}}

	if got := entry.First("memberof"); got != "cn=admins" {
		t.Fatalf("First() = %q, want %q", got, "cn=admins")
	}
	if got := entry.Values("missing"); got != nil {
		t.Fatalf("Values() = %v, want nil", got)
	}
}
```

`backend/internal/db/ldap_db/errors_test.go` (внутренний тест пакета: `translate` не экспортируется):
```go
package ldap_db

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestTranslateMapsResultCodes(t *testing.T) {
	err := translate(ldap.NewError(ldap.LDAPResultNoSuchObject, errors.New("no such object")))

	if !errors.Is(err, ErrNoSuchObject) {
		t.Fatalf("translate() = %v, want ErrNoSuchObject", err)
	}
}

func TestTranslateTreatsErrorWithoutResultCodeAsUnavailable(t *testing.T) {
	err := translate(errors.New("unable to read LDAP response packet: EOF"))

	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("translate() = %v, want ErrUnavailable", err)
	}
}

func TestServiceBindErrorSeparatesRefusalFromOutage(t *testing.T) {
	refused := serviceBindError(ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("invalid credentials")))
	dropped := serviceBindError(errors.New("unable to read LDAP response packet: EOF"))

	if !errors.Is(refused, ErrServiceBind) || errors.Is(refused, ErrInvalidCredentials) {
		t.Fatalf("refused bind = %v, want ErrServiceBind only", refused)
	}
	if !errors.Is(dropped, ErrUnavailable) {
		t.Fatalf("dropped connection = %v, want ErrUnavailable", dropped)
	}
}
```

- [ ] **Step 3: Запустить тесты и убедиться, что они падают**

Run: `cd backend && go test ./internal/db/ldap_db/`
Expected: FAIL — пакет `ldap_db` не содержит `FilterEquals`, `New` и т. д. (ошибка компиляции).

- [ ] **Step 4: Реализовать `errors.go`**

`backend/internal/db/ldap_db/errors.go`:
```go
package ldap_db

import (
	"errors"
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

var (
	ErrNoSuchObject         = errors.New("ldap: no such object")
	ErrAlreadyExists        = errors.New("ldap: entry already exists")
	ErrObjectClassViolation = errors.New("ldap: object class violation")
	ErrValueExists          = errors.New("ldap: attribute value already exists")
	ErrNoSuchValue          = errors.New("ldap: no such attribute value")
	ErrInvalidCredentials   = errors.New("ldap: invalid credentials")
	ErrUnavailable          = errors.New("ldap: directory unavailable")
	ErrServiceBind          = errors.New("ldap: service account bind failed")
	ErrInvalidDN            = errors.New("ldap: invalid dn")
)

var resultCodeErrors = map[uint16]error{
	ldap.LDAPResultNoSuchAttribute:        ErrNoSuchValue,
	ldap.LDAPResultAttributeOrValueExists: ErrValueExists,
	ldap.LDAPResultNoSuchObject:           ErrNoSuchObject,
	ldap.LDAPResultInvalidCredentials:     ErrInvalidCredentials,
	ldap.LDAPResultBusy:                   ErrUnavailable,
	ldap.LDAPResultUnavailable:            ErrUnavailable,
	ldap.LDAPResultObjectClassViolation:   ErrObjectClassViolation,
	ldap.LDAPResultEntryAlreadyExists:     ErrAlreadyExists,
	ldap.ErrorNetwork:                     ErrUnavailable,
}

// translate превращает ошибку go-ldap в sentinel этого пакета, чтобы слой репозиториев
// не зависел от кодов результата LDAP. Незнакомые коды проходят как есть.
func translate(err error) error {
	if err == nil {
		return nil
	}
	var ldapError *ldap.Error
	if !errors.As(err, &ldapError) {
		// Без кода результата — значит, ответа сервера не было: go-ldap так сообщает
		// об оборванном посреди запроса соединении («unable to read LDAP response packet»).
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	sentinel, known := resultCodeErrors[ldapError.ResultCode]
	if !known {
		return err
	}
	return fmt.Errorf("%w: %v", sentinel, err)
}
```

- [ ] **Step 5: Реализовать `names.go`**

`backend/internal/db/ldap_db/names.go`:
```go
package ldap_db

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// FilterEquals строит фильтр равенства. Значение приходит снаружи: без экранирования
// символы `*`, `(` и `)` изменили бы смысл фильтра (LDAP-инъекция).
func FilterEquals(attribute, value string) string {
	return "(" + attribute + "=" + ldap.EscapeFilter(value) + ")"
}

func RDN(attribute, value string) string {
	return attribute + "=" + ldap.EscapeDN(value)
}

func Join(rdn, parentDN string) string {
	return rdn + "," + parentDN
}

// IsWithin сообщает, совпадает ли dn с base или лежит под ним. Регистр не важен:
// для LDAP «DC=Example» и «dc=example» — одно и то же имя.
func IsWithin(dn, base string) (bool, error) {
	parsedDN, err := parseDN(dn)
	if err != nil {
		return false, err
	}
	parsedBase, err := parseDN(base)
	if err != nil {
		return false, err
	}
	if len(parsedDN.RDNs) == 0 {
		return false, nil
	}
	return parsedBase.EqualFold(parsedDN) || parsedBase.AncestorOfFold(parsedDN), nil
}

func FirstRDN(dn string) (string, error) {
	parsed, err := parseDN(dn)
	if err != nil {
		return "", err
	}
	if len(parsed.RDNs) == 0 {
		return "", nil
	}
	return parsed.RDNs[0].String(), nil
}

// ChildValue возвращает значение из RDN вида attribute=value, если dn — прямой потомок
// parentDN. Так из DN участника группы получают uid пользователя.
func ChildValue(dn, attribute, parentDN string) (string, bool) {
	parsedDN, err := parseDN(dn)
	if err != nil {
		return "", false
	}
	parsedParent, err := parseDN(parentDN)
	if err != nil {
		return "", false
	}
	isDirectChild := len(parsedDN.RDNs) == len(parsedParent.RDNs)+1 && parsedParent.AncestorOfFold(parsedDN)
	if !isDirectChild {
		return "", false
	}
	firstRDN := parsedDN.RDNs[0].Attributes
	if len(firstRDN) != 1 || !strings.EqualFold(firstRDN[0].Type, attribute) {
		return "", false
	}
	return firstRDN[0].Value, true
}

func parseDN(dn string) (*ldap.DN, error) {
	parsed, err := ldap.ParseDN(dn)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDN, err)
	}
	return parsed, nil
}
```

- [ ] **Step 6: Реализовать `client.go`**

`backend/internal/db/ldap_db/client.go`:
```go
// Package ldap_db — транспорт к LDAP-серверу (слой 4). Каждая операция открывает своё
// соединение и делает bind сервисным аккаунтом: так нет устаревших соединений и вопроса,
// под чьим именем сейчас выполнен bind. Для учебного проекта это важнее скорости пула.
package ldap_db

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

const (
	defaultTimeout  = 5 * time.Second
	anyObjectFilter = "(objectClass=*)"
)

type Config struct {
	URL          string
	BindDN       string
	BindPassword string
}

type Client struct {
	config Config
}

func New(config Config) *Client {
	return &Client{config: config}
}

type Scope int

const (
	ScopeBase Scope = iota
	ScopeOneLevel
	ScopeSubtree
)

type SearchRequest struct {
	BaseDN     string
	Scope      Scope
	Filter     string
	Attributes []string
}

type Entry struct {
	DN         string
	Attributes map[string][]string
}

// Values ищет атрибут без учёта регистра: сервер возвращает имя так, как оно записано
// в схеме (memberOf), а не так, как его запросили (memberof).
func (e Entry) Values(attribute string) []string {
	for name, values := range e.Attributes {
		if strings.EqualFold(name, attribute) {
			return values
		}
	}
	return nil
}

func (e Entry) First(attribute string) string {
	values := e.Values(attribute)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

type ChangeOperation int

const (
	ChangeAdd ChangeOperation = iota
	ChangeDelete
	ChangeReplace
)

type Change struct {
	Operation ChangeOperation
	Attribute string
	Values    []string
}

func (c *Client) Search(ctx context.Context, request SearchRequest) ([]Entry, error) {
	filter := request.Filter
	if filter == "" {
		filter = anyObjectFilter
	}
	var entries []Entry
	err := c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		result, err := connection.Search(ldap.NewSearchRequest(
			request.BaseDN, toLDAPScope(request.Scope), ldap.NeverDerefAliases, 0, 0, false,
			filter, request.Attributes, nil,
		))
		if err != nil {
			return err
		}
		entries = toEntries(result.Entries)
		return nil
	})
	return entries, err
}

func (c *Client) Add(ctx context.Context, dn string, attributes map[string][]string) error {
	request := ldap.NewAddRequest(dn, nil)
	for name, values := range attributes {
		request.Attribute(name, values)
	}
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		return connection.Add(request)
	})
}

func (c *Client) Modify(ctx context.Context, dn string, changes []Change) error {
	request := ldap.NewModifyRequest(dn, nil)
	for _, change := range changes {
		appendChange(request, change)
	}
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		return connection.Modify(request)
	})
}

func (c *Client) Delete(ctx context.Context, dn string) error {
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		return connection.Del(ldap.NewDelRequest(dn, nil))
	})
}

func (c *Client) Compare(ctx context.Context, dn, attribute, value string) (bool, error) {
	var matched bool
	err := c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		result, err := connection.Compare(dn, attribute, value)
		matched = result
		return err
	})
	return matched, err
}

// SetPassword использует расширенную операцию Password Modify (RFC 3062): сервер сам
// хэширует пароль. Запись атрибута userPassword напрямую сохранила бы его открытым текстом.
func (c *Client) SetPassword(ctx context.Context, dn, password string) error {
	return c.withServiceConnection(ctx, func(connection *ldap.Conn) error {
		_, err := connection.PasswordModify(ldap.NewPasswordModifyRequest(dn, "", password))
		return err
	})
}

// VerifyPassword проверяет пароль bind'ом от имени пользователя на отдельном соединении.
func (c *Client) VerifyPassword(ctx context.Context, dn, password string) error {
	if password == "" {
		// Bind с DN и пустым паролем по стандарту — анонимный вход, и многие серверы
		// отвечают на него «успехом». Такой ответ нельзя принять за проверку пароля.
		return ErrInvalidCredentials
	}
	connection, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	return translate(connection.Bind(dn, password))
}

func (c *Client) withServiceConnection(ctx context.Context, operation func(*ldap.Conn) error) error {
	connection, err := c.dial(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	if err := connection.Bind(c.config.BindDN, c.config.BindPassword); err != nil {
		return serviceBindError(err)
	}
	return translate(operation(connection))
}

// serviceBindError отделяет отказ сервисного bind от отказа пользовательского: неверный
// пароль сервисного аккаунта — ошибка конфигурации, а не «неверный логин» пользователя.
func serviceBindError(err error) error {
	translated := translate(err)
	if errors.Is(translated, ErrUnavailable) {
		return translated
	}
	return fmt.Errorf("%w: %v", ErrServiceBind, err)
}

func (c *Client) dial(ctx context.Context) (*ldap.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	timeout := timeoutFrom(ctx)
	connection, err := ldap.DialURL(c.config.URL, ldap.DialWithDialer(&net.Dialer{Timeout: timeout}))
	if err != nil {
		return nil, translate(err)
	}
	connection.SetTimeout(timeout)
	return connection, nil
}

func timeoutFrom(ctx context.Context) time.Duration {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		return defaultTimeout
	}
	return max(time.Until(deadline), time.Millisecond)
}

func appendChange(request *ldap.ModifyRequest, change Change) {
	switch change.Operation {
	case ChangeAdd:
		request.Add(change.Attribute, change.Values)
	case ChangeDelete:
		request.Delete(change.Attribute, change.Values)
	case ChangeReplace:
		request.Replace(change.Attribute, change.Values)
	}
}

func toLDAPScope(scope Scope) int {
	switch scope {
	case ScopeOneLevel:
		return ldap.ScopeSingleLevel
	case ScopeSubtree:
		return ldap.ScopeWholeSubtree
	default:
		return ldap.ScopeBaseObject
	}
}

func toEntries(source []*ldap.Entry) []Entry {
	entries := make([]Entry, 0, len(source))
	for _, entry := range source {
		attributes := make(map[string][]string, len(entry.Attributes))
		for _, attribute := range entry.Attributes {
			attributes[attribute.Name] = attribute.Values
		}
		entries = append(entries, Entry{DN: entry.DN, Attributes: attributes})
	}
	return entries
}
```

- [ ] **Step 7: Запустить юнит-тесты**

Run: `cd backend && go mod tidy && go test ./internal/db/ldap_db/`
Expected: PASS.

- [ ] **Step 8: Реализовать `ldapstand`**

`backend/internal/testsupport/ldapstand/ldapstand.go`:
```go
// Package ldapstand даёт интеграционным тестам LDAP из docker-compose.yml: клиент
// сервисного аккаунта и временную ветку дерева, которая удаляется после теста.
package ldapstand

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"testing"

	"ldap-admin/internal/db/ldap_db"
)

const (
	defaultURL          = "ldap://localhost:389"
	defaultBindDN       = "cn=admin,dc=example,dc=com"
	defaultBindPassword = "admin"
	defaultBaseDN       = "dc=example,dc=com"
	suffixBytes         = 4
)

type Stand struct {
	Client *ldap_db.Client
	Config ldap_db.Config
	// Base — временная ветка ou=test-<random> под base DN каталога.
	Base string
}

func Connect(t *testing.T) Stand {
	t.Helper()
	config := ldap_db.Config{
		URL:          valueOr("LDAP_ADMIN_TEST_LDAP_URL", defaultURL),
		BindDN:       valueOr("LDAP_ADMIN_TEST_LDAP_BIND_DN", defaultBindDN),
		BindPassword: valueOr("LDAP_ADMIN_TEST_LDAP_BIND_PASSWORD", defaultBindPassword),
	}
	directoryBase := valueOr("LDAP_ADMIN_TEST_LDAP_BASE_DN", defaultBaseDN)
	client := ldap_db.New(config)

	_, err := client.Search(t.Context(), ldap_db.SearchRequest{BaseDN: directoryBase, Scope: ldap_db.ScopeBase})
	if errors.Is(err, ldap_db.ErrUnavailable) {
		t.Skipf("LDAP is not reachable at %s: %v", config.URL, err)
	}
	if err != nil {
		t.Fatalf("probe LDAP: %v", err)
	}

	name := "test-" + randomSuffix(t)
	base := ldap_db.Join(ldap_db.RDN("ou", name), directoryBase)
	stand := Stand{Client: client, Config: config, Base: base}
	addEntry(t, client, base, map[string][]string{"objectClass": {"organizationalUnit"}, "ou": {name}})
	t.Cleanup(func() { removeSubtree(t, client, base) })
	return stand
}

func (s Stand) CreateOU(t *testing.T, name string) string {
	t.Helper()
	dn := ldap_db.Join(ldap_db.RDN("ou", name), s.Base)
	addEntry(t, s.Client, dn, map[string][]string{"objectClass": {"organizationalUnit"}, "ou": {name}})
	return dn
}

func (s Stand) CreatePerson(t *testing.T, parentDN, uid string) string {
	t.Helper()
	dn := ldap_db.Join(ldap_db.RDN("uid", uid), parentDN)
	addEntry(t, s.Client, dn, map[string][]string{
		"objectClass": {"inetOrgPerson"}, "uid": {uid}, "cn": {uid}, "sn": {uid},
	})
	return dn
}

func (s Stand) CreateGroup(t *testing.T, parentDN, name string, memberDNs ...string) string {
	t.Helper()
	dn := ldap_db.Join(ldap_db.RDN("cn", name), parentDN)
	addEntry(t, s.Client, dn, map[string][]string{
		"objectClass": {"groupOfUniqueNames"}, "cn": {name}, "uniqueMember": memberDNs,
	})
	return dn
}

func addEntry(t *testing.T, client *ldap_db.Client, dn string, attributes map[string][]string) {
	t.Helper()
	if err := client.Add(t.Context(), dn, attributes); err != nil {
		t.Fatalf("add %s: %v", dn, err)
	}
}

// removeSubtree удаляет ветку снизу вверх: LDAP не удаляет запись, у которой есть потомки.
// DN потомка всегда длиннее DN предка, поэтому сортировки по длине достаточно.
// t.Context() к моменту Cleanup уже отменён, поэтому контекст — фоновый.
func removeSubtree(t *testing.T, client *ldap_db.Client, base string) {
	ctx := context.Background()
	entries, err := client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: base, Scope: ldap_db.ScopeSubtree, Attributes: []string{"1.1"},
	})
	if err != nil {
		t.Logf("cleanup search %s: %v", base, err)
		return
	}
	slices.SortFunc(entries, func(left, right ldap_db.Entry) int {
		return cmp.Compare(len(right.DN), len(left.DN))
	})
	for _, entry := range entries {
		if err := client.Delete(ctx, entry.DN); err != nil {
			t.Logf("cleanup delete %s: %v", entry.DN, err)
		}
	}
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	buffer := make([]byte, suffixBytes)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(buffer)
}

func valueOr(name, fallback string) string {
	if value, present := os.LookupEnv(name); present && value != "" {
		return value
	}
	return fallback
}
```

- [ ] **Step 9: Написать интеграционные тесты поведения сервера**

Эти тесты фиксируют то, на что опирается дизайн (раздел 2.1 спецификации), включая два пункта, не проверенных вручную: `memberOf` при modify и Compare по `uniqueMember`.

`backend/internal/db/ldap_db/client_integration_test.go`:
```go
//go:build integration

package ldap_db_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/testsupport/ldapstand"
)

func TestAddSearchAndDelete(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	aliceDN := stand.CreatePerson(t, people, "alice")

	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: people, Scope: ldap_db.ScopeOneLevel,
		Filter: ldap_db.FilterEquals("uid", "alice"), Attributes: []string{"cn"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(entries) != 1 || entries[0].First("cn") != "alice" {
		t.Fatalf("Search() = %+v, want one entry with cn=alice", entries)
	}

	if err := stand.Client.Delete(t.Context(), aliceDN); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = stand.Client.Search(t.Context(), ldap_db.SearchRequest{BaseDN: aliceDN, Scope: ldap_db.ScopeBase})
	if !errors.Is(err, ldap_db.ErrNoSuchObject) {
		t.Fatalf("Search() after delete error = %v, want ErrNoSuchObject", err)
	}
}

func TestAddExistingEntryIsAlreadyExists(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")

	err := stand.Client.Add(t.Context(), people, map[string][]string{"objectClass": {"organizationalUnit"}, "ou": {"people"}})

	if !errors.Is(err, ldap_db.ErrAlreadyExists) {
		t.Fatalf("Add() error = %v, want ErrAlreadyExists", err)
	}
}

func TestMemberOfFollowsUniqueMemberChanges(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	modifyMember(t, stand, teamDN, ldap_db.ChangeAdd, bobDN)
	if !containsFold(memberOf(t, stand, bobDN), teamDN) {
		t.Fatalf("memberOf(bob) after add = %v, want %s", memberOf(t, stand, bobDN), teamDN)
	}

	modifyMember(t, stand, teamDN, ldap_db.ChangeDelete, bobDN)
	if got := memberOf(t, stand, bobDN); len(got) != 0 {
		t.Fatalf("memberOf(bob) after delete = %v, want empty", got)
	}
}

func TestRefintRemovesDeletedUserFromGroups(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN, bobDN)

	if err := stand.Client.Delete(t.Context(), bobDN); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	members := uniqueMembers(t, stand, teamDN)
	if len(members) != 1 || !strings.EqualFold(members[0], aliceDN) {
		t.Fatalf("uniqueMember after delete = %v, want only %s", members, aliceDN)
	}
}

func TestRemovingLastUniqueMemberIsObjectClassViolation(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	err := stand.Client.Modify(t.Context(), teamDN, []ldap_db.Change{
		{Operation: ldap_db.ChangeDelete, Attribute: "uniqueMember", Values: []string{aliceDN}},
	})

	if !errors.Is(err, ldap_db.ErrObjectClassViolation) {
		t.Fatalf("Modify() error = %v, want ErrObjectClassViolation", err)
	}
}

func TestAddingPresentValueAndDeletingAbsentValue(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	addErr := stand.Client.Modify(t.Context(), teamDN, []ldap_db.Change{
		{Operation: ldap_db.ChangeAdd, Attribute: "uniqueMember", Values: []string{aliceDN}},
	})
	deleteErr := stand.Client.Modify(t.Context(), teamDN, []ldap_db.Change{
		{Operation: ldap_db.ChangeDelete, Attribute: "uniqueMember", Values: []string{bobDN}},
	})

	if !errors.Is(addErr, ldap_db.ErrValueExists) {
		t.Fatalf("add present value error = %v, want ErrValueExists", addErr)
	}
	if !errors.Is(deleteErr, ldap_db.ErrNoSuchValue) {
		t.Fatalf("delete absent value error = %v, want ErrNoSuchValue", deleteErr)
	}
}

func TestCompareUniqueMember(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	aliceMatched, aliceErr := stand.Client.Compare(t.Context(), teamDN, "uniqueMember", aliceDN)
	bobMatched, bobErr := stand.Client.Compare(t.Context(), teamDN, "uniqueMember", bobDN)

	if aliceErr != nil || !aliceMatched {
		t.Fatalf("Compare(alice) = (%v, %v), want (true, nil)", aliceMatched, aliceErr)
	}
	if bobErr != nil || bobMatched {
		t.Fatalf("Compare(bob) = (%v, %v), want (false, nil)", bobMatched, bobErr)
	}
}

func TestSetPasswordStoresHashAndVerifies(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	aliceDN := stand.CreatePerson(t, people, "alice")

	if err := stand.Client.SetPassword(t.Context(), aliceDN, "correct-horse"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}

	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: aliceDN, Scope: ldap_db.ScopeBase, Attributes: []string{"userPassword"},
	})
	if err != nil || len(entries) != 1 {
		t.Fatalf("read userPassword: entries=%v err=%v", entries, err)
	}
	if !strings.HasPrefix(entries[0].First("userPassword"), "{SSHA}") {
		t.Fatalf("userPassword is not an SSHA hash")
	}
	if err := stand.Client.VerifyPassword(t.Context(), aliceDN, "correct-horse"); err != nil {
		t.Fatalf("VerifyPassword(correct) error = %v", err)
	}
	if err := stand.Client.VerifyPassword(t.Context(), aliceDN, "wrong-horse"); !errors.Is(err, ldap_db.ErrInvalidCredentials) {
		t.Fatalf("VerifyPassword(wrong) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestHasSubordinatesIsReturnedOnlyWhenRequested(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	stand.CreatePerson(t, people, "alice")
	stand.CreateOU(t, "groups")

	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: stand.Base, Scope: ldap_db.ScopeOneLevel, Attributes: []string{"hasSubordinates"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	flags := map[string]string{}
	for _, entry := range entries {
		rdn, _ := ldap_db.FirstRDN(entry.DN)
		flags[rdn] = entry.First("hasSubordinates")
	}
	if flags["ou=people"] != "TRUE" || flags["ou=groups"] != "FALSE" {
		t.Fatalf("hasSubordinates = %v, want people TRUE and groups FALSE", flags)
	}
}

func TestWrongServicePasswordIsServiceBindError(t *testing.T) {
	stand := ldapstand.Connect(t)
	config := stand.Config
	config.BindPassword = "definitely-wrong"

	_, err := ldap_db.New(config).Search(t.Context(), ldap_db.SearchRequest{BaseDN: stand.Base, Scope: ldap_db.ScopeBase})

	if !errors.Is(err, ldap_db.ErrServiceBind) {
		t.Fatalf("Search() error = %v, want ErrServiceBind", err)
	}
	if errors.Is(err, ldap_db.ErrInvalidCredentials) {
		t.Fatalf("service bind failure must not look like user invalid credentials")
	}
}

func modifyMember(t *testing.T, stand ldapstand.Stand, groupDN string, operation ldap_db.ChangeOperation, memberDN string) {
	t.Helper()
	err := stand.Client.Modify(t.Context(), groupDN, []ldap_db.Change{
		{Operation: operation, Attribute: "uniqueMember", Values: []string{memberDN}},
	})
	if err != nil {
		t.Fatalf("Modify() error = %v", err)
	}
}

func memberOf(t *testing.T, stand ldapstand.Stand, dn string) []string {
	t.Helper()
	return readAttribute(t, stand, dn, "memberOf")
}

func uniqueMembers(t *testing.T, stand ldapstand.Stand, dn string) []string {
	t.Helper()
	return readAttribute(t, stand, dn, "uniqueMember")
}

func readAttribute(t *testing.T, stand ldapstand.Stand, dn, attribute string) []string {
	t.Helper()
	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: dn, Scope: ldap_db.ScopeBase, Attributes: []string{attribute},
	})
	if err != nil || len(entries) != 1 {
		t.Fatalf("read %s of %s: entries=%v err=%v", attribute, dn, entries, err)
	}
	return entries[0].Values(attribute)
}

func containsFold(values []string, target string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return strings.EqualFold(value, target) })
}
```

- [ ] **Step 10: Запустить интеграционные тесты против контейнера**

Run: `cd backend && make test-integration`
Expected: PASS всех тестов `ldap_db`. Если `TestMemberOfFollowsUniqueMemberChanges` или `TestCompareUniqueMember` падают — **остановиться и сообщить**: на этих предположениях построены задачи 6 и 8.

- [ ] **Step 11: Проверить vet**

Run: `cd backend && make vet`
Expected: без замечаний.

- [ ] **Step 12: Commit**

```bash
git add backend/go.mod backend/go.sum backend/Makefile backend/internal/db backend/internal/testsupport
git commit -m "Add LDAP transport with server behaviour tests

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 3: Домен — `directory` и `user`

**Files:**
- Create: `backend/internal/domain/directory/directory.go`
- Create: `backend/internal/domain/user/user.go`
- Create: `backend/internal/domain/user/errors.go`
- Test: `backend/internal/domain/user/user_test.go`

**Interfaces:**
- Consumes: ничего.
- Produces (пакет `ldap-admin/internal/domain/directory`):
  - `type DN string`
  - `type Node struct { DN DN; RDN string; ObjectClasses []string; HasChildren bool }`
  - `type Entry struct { DN DN; Attributes, OperationalAttributes map[string][]string }`
  - `ErrNotFound`, `ErrOutsideBase`, `ErrInvalidDN`, `ErrUnavailable`
- Produces (пакет `ldap-admin/internal/domain/user`):
  - `type UID string`; `func ParseUID(raw string) (UID, error)`
  - `type User struct { UID UID; CommonName, Surname string; Emails []string }`
  - `func New(uid UID, commonName, surname string, emails []string) (User, error)` — `Emails` никогда не `nil`
  - `const MinPasswordLength = 8`; `func ValidatePassword(password string) error`
  - `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalid`, `ErrSelfDelete`

- [ ] **Step 1: Создать `directory.go`** (логики нет, тестировать нечего)

`backend/internal/domain/directory/directory.go`:
```go
// Package directory — словарь браузера дерева каталога. DN здесь — непрозрачная строка:
// разбор синтаксиса DN (экранирование, регистр) — знание источника, оно в слое транспорта.
package directory

import "errors"

type DN string

type Node struct {
	DN            DN
	RDN           string
	ObjectClasses []string
	HasChildren   bool
}

// Entry делит атрибуты на обычные и служебные: служебные (memberOf, entryUUID, ...)
// ведёт сам сервер, и это стоит видеть.
type Entry struct {
	DN                    DN
	Attributes            map[string][]string
	OperationalAttributes map[string][]string
}

var (
	ErrNotFound    = errors.New("entry not found")
	ErrOutsideBase = errors.New("dn is outside the directory base")
	ErrInvalidDN   = errors.New("invalid dn")
	// ErrUnavailable — каталог не отвечает. Общая для всех репозиториев поверх LDAP.
	ErrUnavailable = errors.New("directory unavailable")
)
```

- [ ] **Step 2: Написать падающие тесты `user`**

`backend/internal/domain/user/user_test.go`:
```go
package user_test

import (
	"errors"
	"strings"
	"testing"

	"ldap-admin/internal/domain/user"
)

func TestParseUID(t *testing.T) {
	valid := []string{"alice", "a", "john.doe", "user_1", "x-y", "a" + strings.Repeat("b", 63)}
	for _, raw := range valid {
		if _, err := user.ParseUID(raw); err != nil {
			t.Errorf("ParseUID(%q) error = %v, want nil", raw, err)
		}
	}
	invalid := []string{"", "Alice", "1alice", "a b", "a*)(uid=*", "uid,ou=x", "a" + strings.Repeat("b", 64)}
	for _, raw := range invalid {
		if _, err := user.ParseUID(raw); !errors.Is(err, user.ErrInvalid) {
			t.Errorf("ParseUID(%q) error = %v, want ErrInvalid", raw, err)
		}
	}
}

func TestNewTrimsNamesAndKeepsDNSpecialCharacters(t *testing.T) {
	created, err := user.New("jdoe", "  Doe, John  ", " Doe ", []string{" jdoe@example.com "})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if created.CommonName != "Doe, John" || created.Surname != "Doe" {
		t.Fatalf("New() names = %q / %q", created.CommonName, created.Surname)
	}
	if len(created.Emails) != 1 || created.Emails[0] != "jdoe@example.com" {
		t.Fatalf("New() emails = %v", created.Emails)
	}
}

func TestNewWithoutEmailsReturnsEmptySlice(t *testing.T) {
	created, err := user.New("jdoe", "John", "Doe", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if created.Emails == nil || len(created.Emails) != 0 {
		t.Fatalf("New() emails = %#v, want empty non-nil slice", created.Emails)
	}
}

func TestNewRejectsInvalidData(t *testing.T) {
	cases := []struct {
		name       string
		uid        user.UID
		commonName string
		surname    string
		emails     []string
	}{
		{name: "invalid uid", uid: "Bad", commonName: "John", surname: "Doe"},
		{name: "blank common name", uid: "jdoe", commonName: "   ", surname: "Doe"},
		{name: "blank surname", uid: "jdoe", commonName: "John", surname: ""},
		{name: "too long surname", uid: "jdoe", commonName: "John", surname: strings.Repeat("я", 257)},
		{name: "not an email", uid: "jdoe", commonName: "John", surname: "Doe", emails: []string{"nope"}},
		{name: "email with display name", uid: "jdoe", commonName: "John", surname: "Doe", emails: []string{"John <j@example.com>"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := user.New(testCase.uid, testCase.commonName, testCase.surname, testCase.emails)
			if !errors.Is(err, user.ErrInvalid) {
				t.Fatalf("New() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	if err := user.ValidatePassword("12345678"); err != nil {
		t.Fatalf("ValidatePassword(8 chars) error = %v", err)
	}
	if err := user.ValidatePassword("пароль12"); err != nil {
		t.Fatalf("ValidatePassword(8 runes) error = %v", err)
	}
	for _, password := range []string{"", "1234567"} {
		if err := user.ValidatePassword(password); !errors.Is(err, user.ErrInvalid) {
			t.Errorf("ValidatePassword(%q) error = %v, want ErrInvalid", password, err)
		}
	}
}
```

- [ ] **Step 3: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/domain/user/`
Expected: FAIL — пакет `user` не существует.

- [ ] **Step 4: Реализовать `user`**

`backend/internal/domain/user/errors.go`:
```go
package user

import "errors"

var (
	ErrNotFound      = errors.New("user not found")
	ErrAlreadyExists = errors.New("user already exists")
	ErrInvalid       = errors.New("invalid user data")
	ErrSelfDelete    = errors.New("you cannot delete yourself")
)
```

`backend/internal/domain/user/user.go`:
```go
package user

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MinPasswordLength = 8
	maxNameLength     = 256
)

// uidPattern ограничивает uid безопасным подмножеством: такой uid можно подставить
// в DN и фильтр без сюрпризов, и он однозначен без учёта регистра.
var uidPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)

type UID string

func ParseUID(raw string) (UID, error) {
	if !uidPattern.MatchString(raw) {
		return "", fmt.Errorf("%w: uid must start with a lowercase letter and contain only a-z, 0-9, '.', '_', '-' (up to 64 characters)", ErrInvalid)
	}
	return UID(raw), nil
}

type User struct {
	UID        UID
	CommonName string
	Surname    string
	Emails     []string
}

func New(uid UID, commonName, surname string, emails []string) (User, error) {
	if _, err := ParseUID(string(uid)); err != nil {
		return User{}, err
	}
	commonName = strings.TrimSpace(commonName)
	surname = strings.TrimSpace(surname)
	if err := validateName("common name", commonName); err != nil {
		return User{}, err
	}
	if err := validateName("surname", surname); err != nil {
		return User{}, err
	}
	normalizedEmails, err := normalizeEmails(emails)
	if err != nil {
		return User{}, err
	}
	return User{UID: uid, CommonName: commonName, Surname: surname, Emails: normalizedEmails}, nil
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalid, MinPasswordLength)
	}
	return nil
}

func validateName(field, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalid, field)
	}
	if utf8.RuneCountInString(value) > maxNameLength {
		return fmt.Errorf("%w: %s is longer than %d characters", ErrInvalid, field, maxNameLength)
	}
	return nil
}

func normalizeEmails(emails []string) ([]string, error) {
	normalized := make([]string, 0, len(emails))
	for _, email := range emails {
		trimmed := strings.TrimSpace(email)
		address, err := mail.ParseAddress(trimmed)
		if err != nil || address.Address != trimmed {
			return nil, fmt.Errorf("%w: %q is not a valid email address", ErrInvalid, trimmed)
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}
```

- [ ] **Step 5: Запустить тесты**

Run: `cd backend && go test ./internal/domain/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/domain/directory backend/internal/domain/user
git commit -m "Add directory and user domain

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Домен — `group` и `session`

**Files:**
- Create: `backend/internal/domain/group/group.go`
- Create: `backend/internal/domain/group/errors.go`
- Create: `backend/internal/domain/session/session.go`
- Test: `backend/internal/domain/group/group_test.go`
- Test: `backend/internal/domain/session/session_test.go`

**Interfaces:**
- Consumes: `directory.DN`, `user.UID` (задача 3).
- Produces (пакет `ldap-admin/internal/domain/group`):
  - `type Name string`; `func ParseName(raw string) (Name, error)`
  - `type Member struct { DN directory.DN; UID user.UID }` — у участника из `ou=people` задан `UID`, у участника вне её — только `DN`
  - `type Group struct { Name Name; Description string; Members []Member }`
  - `func New(name Name, description string, members []Member) (Group, error)`
  - `func (Group) HasMember(uid user.UID) bool`, `func (Group) IsSoleMember(uid user.UID) bool`, `func (Group) CheckRemoval(uid user.UID) error`
  - `ErrNotFound`, `ErrAlreadyExists`, `ErrInvalid`, `ErrLastMember`, `ErrNotMember`, `ErrAlreadyMember`, `ErrProtected`
  - `type SoleMemberError struct { Groups []Name }` (используется как `*SoleMemberError`)
- Produces (пакет `ldap-admin/internal/domain/session`):
  - `type Session struct { ID string; UID user.UID; CommonName string; ExpiresAt time.Time }`; `func (Session) IsExpired(now time.Time) bool`
  - `type Clock interface { Now() time.Time }`; `type IDGenerator interface { NewID() (string, error) }`
  - `ErrInvalidCredentials`, `ErrNotAdmin`, `ErrNotFound`

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/domain/group/group_test.go`:
```go
package group_test

import (
	"errors"
	"strings"
	"testing"

	"ldap-admin/internal/domain/group"
)

func TestParseName(t *testing.T) {
	if _, err := group.ParseName("dev-team"); err != nil {
		t.Fatalf("ParseName(dev-team) error = %v", err)
	}
	for _, raw := range []string{"", "Admins", "a,b", "a b"} {
		if _, err := group.ParseName(raw); !errors.Is(err, group.ErrInvalid) {
			t.Errorf("ParseName(%q) error = %v, want ErrInvalid", raw, err)
		}
	}
}

func TestNewRequiresAtLeastOneMember(t *testing.T) {
	_, err := group.New("team", "", nil)
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewRejectsDuplicateMember(t *testing.T) {
	_, err := group.New("team", "", []group.Member{{UID: "alice"}, {UID: "alice"}})
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewRejectsMemberWithoutIdentity(t *testing.T) {
	_, err := group.New("team", "", []group.Member{{}})
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewRejectsTooLongDescription(t *testing.T) {
	_, err := group.New("team", strings.Repeat("d", 1025), []group.Member{{UID: "alice"}})
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewAcceptsMemberOutsidePeople(t *testing.T) {
	created, err := group.New("team", "  Team  ", []group.Member{{DN: "cn=admin,dc=example,dc=com"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if created.Description != "Team" {
		t.Fatalf("Description = %q, want trimmed", created.Description)
	}
}

func TestCheckRemoval(t *testing.T) {
	pair, _ := group.New("team", "", []group.Member{{UID: "alice"}, {UID: "bob"}})
	solo, _ := group.New("solo", "", []group.Member{{UID: "alice"}})

	if err := pair.CheckRemoval("bob"); err != nil {
		t.Fatalf("CheckRemoval(bob) error = %v", err)
	}
	if err := pair.CheckRemoval("carol"); !errors.Is(err, group.ErrNotMember) {
		t.Fatalf("CheckRemoval(carol) error = %v, want ErrNotMember", err)
	}
	if err := solo.CheckRemoval("alice"); !errors.Is(err, group.ErrLastMember) {
		t.Fatalf("CheckRemoval(last) error = %v, want ErrLastMember", err)
	}
}

func TestIsSoleMember(t *testing.T) {
	pair, _ := group.New("team", "", []group.Member{{UID: "alice"}, {UID: "bob"}})
	solo, _ := group.New("solo", "", []group.Member{{UID: "alice"}})

	if pair.IsSoleMember("alice") || !solo.IsSoleMember("alice") || solo.IsSoleMember("bob") {
		t.Fatalf("IsSoleMember gives wrong answers")
	}
}

func TestSoleMemberErrorNamesGroups(t *testing.T) {
	err := &group.SoleMemberError{Groups: []group.Name{"solo", "duo"}}
	if !strings.Contains(err.Error(), "solo, duo") {
		t.Fatalf("Error() = %q, want group names", err.Error())
	}
}
```

`backend/internal/domain/session/session_test.go`:
```go
package session_test

import (
	"testing"
	"time"

	"ldap-admin/internal/domain/session"
)

func TestIsExpired(t *testing.T) {
	expiresAt := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	current := session.Session{ExpiresAt: expiresAt}

	if current.IsExpired(expiresAt.Add(-time.Second)) {
		t.Fatalf("session expired a second before its deadline")
	}
	if !current.IsExpired(expiresAt) {
		t.Fatalf("session must be expired exactly at its deadline")
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/domain/...`
Expected: FAIL — пакетов `group` и `session` нет.

- [ ] **Step 3: Реализовать `group`**

`backend/internal/domain/group/errors.go`:
```go
package group

import (
	"errors"
	"strings"
)

var (
	ErrNotFound      = errors.New("group not found")
	ErrAlreadyExists = errors.New("group already exists")
	ErrInvalid       = errors.New("invalid group data")
	ErrLastMember    = errors.New("cannot remove the last member of a group")
	ErrNotMember     = errors.New("user is not a member of the group")
	ErrAlreadyMember = errors.New("user is already a member of the group")
	ErrProtected     = errors.New("this group is protected")
)

// SoleMemberError — пользователь единственный участник этих групп. Удалять его нельзя:
// сервер удалил бы запись, а в группах осталась бы ссылка на несуществующий DN.
type SoleMemberError struct {
	Groups []Name
}

func (e *SoleMemberError) Error() string {
	names := make([]string, 0, len(e.Groups))
	for _, name := range e.Groups {
		names = append(names, string(name))
	}
	return "user is the only member of groups: " + strings.Join(names, ", ")
}
```

`backend/internal/domain/group/group.go`:
```go
package group

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/user"
)

const maxDescriptionLength = 1024

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)

type Name string

func ParseName(raw string) (Name, error) {
	if !namePattern.MatchString(raw) {
		return "", fmt.Errorf("%w: group name must start with a lowercase letter and contain only a-z, 0-9, '.', '_', '-' (up to 64 characters)", ErrInvalid)
	}
	return Name(raw), nil
}

type Member struct {
	DN  directory.DN
	UID user.UID
}

type Group struct {
	Name        Name
	Description string
	Members     []Member
}

// New требует хотя бы одного участника: в groupOfUniqueNames атрибут uniqueMember
// обязателен, сервер не примет группу без него.
func New(name Name, description string, members []Member) (Group, error) {
	if _, err := ParseName(string(name)); err != nil {
		return Group{}, err
	}
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Group{}, fmt.Errorf("%w: description is longer than %d characters", ErrInvalid, maxDescriptionLength)
	}
	if len(members) == 0 {
		return Group{}, fmt.Errorf("%w: a group needs at least one member", ErrInvalid)
	}
	if err := validateMembers(members); err != nil {
		return Group{}, err
	}
	return Group{Name: name, Description: description, Members: slices.Clone(members)}, nil
}

func (g Group) HasMember(uid user.UID) bool {
	return slices.ContainsFunc(g.Members, func(member Member) bool { return member.UID == uid })
}

func (g Group) IsSoleMember(uid user.UID) bool {
	return len(g.Members) == 1 && g.HasMember(uid)
}

func (g Group) CheckRemoval(uid user.UID) error {
	if !g.HasMember(uid) {
		return ErrNotMember
	}
	if len(g.Members) == 1 {
		return ErrLastMember
	}
	return nil
}

func validateMembers(members []Member) error {
	seen := make(map[Member]bool, len(members))
	for _, member := range members {
		if member.UID == "" && member.DN == "" {
			return fmt.Errorf("%w: a member needs a uid or a dn", ErrInvalid)
		}
		if seen[member] {
			return fmt.Errorf("%w: member %s is listed twice", ErrInvalid, describe(member))
		}
		seen[member] = true
	}
	return nil
}

func describe(member Member) string {
	if member.UID != "" {
		return string(member.UID)
	}
	return string(member.DN)
}
```

- [ ] **Step 4: Реализовать `session`**

`backend/internal/domain/session/session.go`:
```go
package session

import (
	"errors"
	"time"

	"ldap-admin/internal/domain/user"
)

var (
	ErrInvalidCredentials = errors.New("invalid uid or password")
	ErrNotAdmin           = errors.New("user is not a member of the admins group")
	ErrNotFound           = errors.New("session not found or expired")
)

type Session struct {
	ID         string
	UID        user.UID
	CommonName string
	ExpiresAt  time.Time
}

func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() (string, error)
}
```

- [ ] **Step 5: Запустить тесты**

Run: `cd backend && go test ./internal/domain/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/domain/group backend/internal/domain/session
git commit -m "Add group and session domain

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 5: Раскладка дерева и `user_repo`

**Files:**
- Create: `backend/internal/repos/tree_layout/layout.go`
- Create: `backend/internal/repos/user_repo/repo.go`
- Test: `backend/internal/repos/tree_layout/layout_test.go`
- Test: `backend/internal/repos/user_repo/repo_integration_test.go`

**Interfaces:**
- Consumes: `ldap_db.*` (задача 2), `user.*`, `directory.ErrUnavailable` (задача 3), `group.Name`, `session.ErrInvalidCredentials` (задача 4), `ldapstand` (задача 2).
- Produces (пакет `ldap-admin/internal/repos/tree_layout`):
  - `func New(baseDN string) Layout`
  - `func (Layout) BaseDN() string`, `PeopleDN() string`, `GroupsDN() string`
  - `func (Layout) UserDN(uid user.UID) string`, `GroupDN(name group.Name) string`
  - `func (Layout) UIDOf(dn string) (user.UID, bool)` — uid, если DN лежит прямо в `ou=people`
- Produces (пакет `ldap-admin/internal/repos/user_repo`):
  - `func New(client *ldap_db.Client, layout tree_layout.Layout) *Repo`
  - `List(ctx) ([]user.User, error)`, `Get(ctx, uid user.UID) (user.User, error)`, `Create(ctx, user.User) error`, `Update(ctx, user.User) error`, `SetPassword(ctx, uid user.UID, password string) error`, `Delete(ctx, uid user.UID) error`, `Verify(ctx, uid user.UID, password string) error`

- [ ] **Step 1: Написать падающий тест раскладки**

`backend/internal/repos/tree_layout/layout_test.go`:
```go
package tree_layout_test

import (
	"testing"

	"ldap-admin/internal/repos/tree_layout"
)

func TestLayoutBuildsDNs(t *testing.T) {
	layout := tree_layout.New("dc=example,dc=com")

	if got := layout.PeopleDN(); got != "ou=people,dc=example,dc=com" {
		t.Errorf("PeopleDN() = %q", got)
	}
	if got := layout.GroupsDN(); got != "ou=groups,dc=example,dc=com" {
		t.Errorf("GroupsDN() = %q", got)
	}
	if got := layout.UserDN("alice"); got != "uid=alice,ou=people,dc=example,dc=com" {
		t.Errorf("UserDN() = %q", got)
	}
	if got := layout.GroupDN("admins"); got != "cn=admins,ou=groups,dc=example,dc=com" {
		t.Errorf("GroupDN() = %q", got)
	}
}

func TestUIDOf(t *testing.T) {
	layout := tree_layout.New("dc=example,dc=com")

	if uid, found := layout.UIDOf("UID=alice,OU=People,dc=example,dc=com"); !found || uid != "alice" {
		t.Errorf("UIDOf(person) = (%q, %v), want (alice, true)", uid, found)
	}
	if _, found := layout.UIDOf("cn=admin,dc=example,dc=com"); found {
		t.Errorf("UIDOf(cn=admin) found a uid")
	}
	if _, found := layout.UIDOf("uid=Bob,ou=people,dc=example,dc=com"); found {
		t.Errorf("UIDOf(uid=Bob) accepted a uid that breaks domain rules")
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падает**

Run: `cd backend && go test ./internal/repos/tree_layout/`
Expected: FAIL — пакета нет.

- [ ] **Step 3: Реализовать раскладку**

`backend/internal/repos/tree_layout/layout.go`:
```go
// Package tree_layout знает, где в дереве лежат пользователи и группы, и строит их DN.
// Это единственное место, где зашита раскладка каталога.
package tree_layout

import (
	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type Layout struct {
	baseDN   string
	peopleDN string
	groupsDN string
}

func New(baseDN string) Layout {
	return Layout{
		baseDN:   baseDN,
		peopleDN: ldap_db.Join(ldap_db.RDN("ou", "people"), baseDN),
		groupsDN: ldap_db.Join(ldap_db.RDN("ou", "groups"), baseDN),
	}
}

func (l Layout) BaseDN() string   { return l.baseDN }
func (l Layout) PeopleDN() string { return l.peopleDN }
func (l Layout) GroupsDN() string { return l.groupsDN }

func (l Layout) UserDN(uid user.UID) string {
	return ldap_db.Join(ldap_db.RDN("uid", string(uid)), l.peopleDN)
}

func (l Layout) GroupDN(name group.Name) string {
	return ldap_db.Join(ldap_db.RDN("cn", string(name)), l.groupsDN)
}

func (l Layout) UIDOf(dn string) (user.UID, bool) {
	value, found := ldap_db.ChildValue(dn, "uid", l.peopleDN)
	if !found {
		return "", false
	}
	uid, err := user.ParseUID(value)
	if err != nil {
		return "", false
	}
	return uid, true
}
```

- [ ] **Step 4: Запустить тест раскладки**

Run: `cd backend && go test ./internal/repos/tree_layout/`
Expected: PASS.

- [ ] **Step 5: Написать падающие интеграционные тесты `user_repo`**

`backend/internal/repos/user_repo/repo_integration_test.go`:
```go
//go:build integration

package user_repo_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/tree_layout"
	"ldap-admin/internal/repos/user_repo"
	"ldap-admin/internal/testsupport/ldapstand"
)

func newRepo(t *testing.T) (*user_repo.Repo, ldapstand.Stand) {
	t.Helper()
	stand := ldapstand.Connect(t)
	stand.CreateOU(t, "people")
	stand.CreateOU(t, "groups")
	return user_repo.New(stand.Client, tree_layout.New(stand.Base)), stand
}

func mustUser(t *testing.T, uid user.UID, commonName string, emails ...string) user.User {
	t.Helper()
	created, err := user.New(uid, commonName, "Doe", emails)
	if err != nil {
		t.Fatalf("user.New() error = %v", err)
	}
	return created
}

func TestCreateGetUpdateDelete(t *testing.T) {
	repo, _ := newRepo(t)
	john := mustUser(t, "jdoe", "Doe, John", "jdoe@example.com")

	if err := repo.Create(t.Context(), john); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	stored, err := repo.Get(t.Context(), "jdoe")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.CommonName != "Doe, John" || len(stored.Emails) != 1 {
		t.Fatalf("Get() = %+v", stored)
	}

	if err := repo.Update(t.Context(), mustUser(t, "jdoe", "John Doe")); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, _ := repo.Get(t.Context(), "jdoe")
	if updated.CommonName != "John Doe" || len(updated.Emails) != 0 {
		t.Fatalf("Get() after update = %+v, want new cn and no mail", updated)
	}

	if err := repo.Delete(t.Context(), "jdoe"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get(t.Context(), "jdoe"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestCreateDuplicateIsAlreadyExists(t *testing.T) {
	repo, _ := newRepo(t)
	john := mustUser(t, "jdoe", "John")
	if err := repo.Create(t.Context(), john); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.Create(t.Context(), john); !errors.Is(err, user.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestMissingUserIsNotFound(t *testing.T) {
	repo, _ := newRepo(t)

	if _, err := repo.Get(t.Context(), "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if err := repo.Update(t.Context(), mustUser(t, "ghost", "Ghost")); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(t.Context(), "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestSetPasswordAndVerify(t *testing.T) {
	repo, _ := newRepo(t)
	if err := repo.Create(t.Context(), mustUser(t, "jdoe", "John")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.SetPassword(t.Context(), "jdoe", "correct-horse"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}

	if err := repo.Verify(t.Context(), "jdoe", "correct-horse"); err != nil {
		t.Errorf("Verify(correct) error = %v", err)
	}
	if err := repo.Verify(t.Context(), "jdoe", "wrong-horse"); !errors.Is(err, session.ErrInvalidCredentials) {
		t.Errorf("Verify(wrong) error = %v, want ErrInvalidCredentials", err)
	}
	if err := repo.Verify(t.Context(), "ghost", "whatever-1"); !errors.Is(err, session.ErrInvalidCredentials) {
		t.Errorf("Verify(missing user) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestListIsSortedAndSkipsEntriesBreakingDomainRules(t *testing.T) {
	repo, stand := newRepo(t)
	for _, uid := range []user.UID{"zed", "amy"} {
		if err := repo.Create(t.Context(), mustUser(t, uid, string(uid))); err != nil {
			t.Fatalf("Create(%s) error = %v", uid, err)
		}
	}
	stand.CreatePerson(t, "ou=people,"+stand.Base, "Bob")

	users, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 2 || users[0].UID != "amy" || users[1].UID != "zed" {
		t.Fatalf("List() = %+v, want [amy zed]", users)
	}
}

func TestListWithoutPeopleBranchIsNotNotFound(t *testing.T) {
	stand := ldapstand.Connect(t)
	repo := user_repo.New(stand.Client, tree_layout.New(stand.Base))

	_, err := repo.List(t.Context())

	if err == nil || errors.Is(err, user.ErrNotFound) {
		t.Fatalf("List() error = %v, want a configuration error", err)
	}
}
```

- [ ] **Step 6: Запустить и убедиться, что падают**

Run: `cd backend && go test -tags integration ./internal/repos/user_repo/`
Expected: FAIL — пакета `user_repo` нет.

- [ ] **Step 7: Реализовать `user_repo`**

`backend/internal/repos/user_repo/repo.go`:
```go
// Package user_repo хранит пользователей как записи inetOrgPerson в ou=people.
package user_repo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/tree_layout"
)

var userAttributes = []string{"uid", "cn", "sn", "mail"}

type Repo struct {
	client *ldap_db.Client
	layout tree_layout.Layout
}

func New(client *ldap_db.Client, layout tree_layout.Layout) *Repo {
	return &Repo{client: client, layout: layout}
}

func (r *Repo) List(ctx context.Context) ([]user.User, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN:     r.layout.PeopleDN(),
		Scope:      ldap_db.ScopeOneLevel,
		Filter:     ldap_db.FilterEquals("objectClass", "inetOrgPerson"),
		Attributes: userAttributes,
	})
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return nil, fmt.Errorf("people branch %s is missing, apply seed.ldif: %w", r.layout.PeopleDN(), err)
	}
	if err != nil {
		return nil, mapError(err)
	}
	users := make([]user.User, 0, len(entries))
	for _, entry := range entries {
		// Записи, заведённые в обход админки и не проходящие доменные правила,
		// в список не попадают — их видно в браузере дерева.
		converted, err := toUser(entry)
		if err != nil {
			continue
		}
		users = append(users, converted)
	}
	slices.SortFunc(users, func(left, right user.User) int {
		return strings.Compare(string(left.UID), string(right.UID))
	})
	return users, nil
}

func (r *Repo) Get(ctx context.Context, uid user.UID) (user.User, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: r.layout.UserDN(uid), Scope: ldap_db.ScopeBase, Attributes: userAttributes,
	})
	if err != nil {
		return user.User{}, mapError(err)
	}
	if len(entries) == 0 {
		return user.User{}, user.ErrNotFound
	}
	return toUser(entries[0])
}

func (r *Repo) Create(ctx context.Context, created user.User) error {
	attributes := map[string][]string{
		"objectClass": {"inetOrgPerson"},
		"uid":         {string(created.UID)},
		"cn":          {created.CommonName},
		"sn":          {created.Surname},
	}
	if len(created.Emails) > 0 {
		attributes["mail"] = created.Emails
	}
	return mapError(r.client.Add(ctx, r.layout.UserDN(created.UID), attributes))
}

// Update заменяет атрибуты целиком (modify replace). Пустой список mail удаляет атрибут.
func (r *Repo) Update(ctx context.Context, updated user.User) error {
	return mapError(r.client.Modify(ctx, r.layout.UserDN(updated.UID), []ldap_db.Change{
		{Operation: ldap_db.ChangeReplace, Attribute: "cn", Values: []string{updated.CommonName}},
		{Operation: ldap_db.ChangeReplace, Attribute: "sn", Values: []string{updated.Surname}},
		{Operation: ldap_db.ChangeReplace, Attribute: "mail", Values: updated.Emails},
	}))
}

func (r *Repo) SetPassword(ctx context.Context, uid user.UID, password string) error {
	return mapError(r.client.SetPassword(ctx, r.layout.UserDN(uid), password))
}

func (r *Repo) Delete(ctx context.Context, uid user.UID) error {
	return mapError(r.client.Delete(ctx, r.layout.UserDN(uid)))
}

// Verify проверяет пароль bind'ом пользователя. Сервер отвечает одинаково на неверный
// пароль и несуществующий DN — и это правильно: ответ не подсказывает, есть ли такой uid.
func (r *Repo) Verify(ctx context.Context, uid user.UID, password string) error {
	err := r.client.VerifyPassword(ctx, r.layout.UserDN(uid), password)
	if errors.Is(err, ldap_db.ErrInvalidCredentials) {
		return session.ErrInvalidCredentials
	}
	return mapError(err)
}

func toUser(entry ldap_db.Entry) (user.User, error) {
	uid, err := user.ParseUID(entry.First("uid"))
	if err != nil {
		return user.User{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	converted, err := user.New(uid, entry.First("cn"), entry.First("sn"), entry.Values("mail"))
	if err != nil {
		return user.User{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	return converted, nil
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ldap_db.ErrNoSuchObject):
		return user.ErrNotFound
	case errors.Is(err, ldap_db.ErrAlreadyExists):
		return user.ErrAlreadyExists
	case errors.Is(err, ldap_db.ErrObjectClassViolation):
		return fmt.Errorf("%w: rejected by the directory schema", user.ErrInvalid)
	case errors.Is(err, ldap_db.ErrUnavailable):
		return fmt.Errorf("%w: %w", directory.ErrUnavailable, err)
	default:
		return err
	}
}
```

`toUser` намеренно оборачивает доменную ошибку через `%v`, а не `%w`: запись, сломанная в каталоге, — не ошибка ввода клиента, и наружу она должна уйти как 500, а не 400.

- [ ] **Step 8: Запустить тесты**

Run: `cd backend && go test ./... && go test -tags integration ./internal/repos/user_repo/`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/repos/tree_layout backend/internal/repos/user_repo
git commit -m "Add tree layout and user repository

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `group_repo`

**Files:**
- Create: `backend/internal/repos/group_repo/repo.go`
- Test: `backend/internal/repos/group_repo/repo_integration_test.go`

**Interfaces:**
- Consumes: `ldap_db.*`, `tree_layout.Layout` (задача 5), `group.*`, `user.*`, `directory.*`.
- Produces (пакет `ldap-admin/internal/repos/group_repo`):
  - `func New(client *ldap_db.Client, layout tree_layout.Layout) *Repo`
  - `List(ctx) ([]group.Group, error)`, `Get(ctx, name group.Name) (group.Group, error)`, `Create(ctx, group.Group) error`, `Delete(ctx, name group.Name) error`
  - `AddMember(ctx, name group.Name, uid user.UID) error`, `RemoveMember(ctx, name group.Name, uid user.UID) error`
  - `GroupsOf(ctx, uid user.UID) ([]group.Group, error)` — по атрибуту `memberOf` пользователя
  - `IsMember(ctx, name group.Name, uid user.UID) (bool, error)` — операция Compare

- [ ] **Step 1: Написать падающие интеграционные тесты**

`backend/internal/repos/group_repo/repo_integration_test.go`:
```go
//go:build integration

package group_repo_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/group_repo"
	"ldap-admin/internal/repos/tree_layout"
	"ldap-admin/internal/testsupport/ldapstand"
)

type fixture struct {
	repo   *group_repo.Repo
	stand  ldapstand.Stand
	layout tree_layout.Layout
}

func newFixture(t *testing.T, uids ...string) fixture {
	t.Helper()
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	stand.CreateOU(t, "groups")
	for _, uid := range uids {
		stand.CreatePerson(t, people, uid)
	}
	layout := tree_layout.New(stand.Base)
	return fixture{repo: group_repo.New(stand.Client, layout), stand: stand, layout: layout}
}

func (f fixture) createGroup(t *testing.T, name group.Name, uids ...user.UID) {
	t.Helper()
	members := make([]group.Member, 0, len(uids))
	for _, uid := range uids {
		members = append(members, group.Member{UID: uid})
	}
	created, err := group.New(name, "Test group", members)
	if err != nil {
		t.Fatalf("group.New() error = %v", err)
	}
	if err := f.repo.Create(t.Context(), created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestCreateGetListDelete(t *testing.T) {
	f := newFixture(t, "alice")
	f.createGroup(t, "team", "alice")

	stored, err := f.repo.Get(t.Context(), "team")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Description != "Test group" || len(stored.Members) != 1 || stored.Members[0].UID != "alice" {
		t.Fatalf("Get() = %+v", stored)
	}
	if stored.Members[0].DN == "" {
		t.Fatalf("member DN is empty")
	}

	groups, err := f.repo.List(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("List() = %v, %v", groups, err)
	}

	if err := f.repo.Delete(t.Context(), "team"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := f.repo.Get(t.Context(), "team"); !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestCreateDuplicateIsAlreadyExists(t *testing.T) {
	f := newFixture(t, "alice")
	f.createGroup(t, "team", "alice")

	duplicate, _ := group.New("team", "", []group.Member{{UID: "alice"}})
	if err := f.repo.Create(t.Context(), duplicate); !errors.Is(err, group.ErrAlreadyExists) {
		t.Fatalf("Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestMembershipChangesAreVisibleThroughMemberOf(t *testing.T) {
	f := newFixture(t, "alice", "bob")
	f.createGroup(t, "team", "alice")

	if err := f.repo.AddMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}
	groups, err := f.repo.GroupsOf(t.Context(), "bob")
	if err != nil || len(groups) != 1 || groups[0].Name != "team" {
		t.Fatalf("GroupsOf(bob) = %v, %v; want [team]", groups, err)
	}

	if err := f.repo.RemoveMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("RemoveMember() error = %v", err)
	}
	groups, err = f.repo.GroupsOf(t.Context(), "bob")
	if err != nil || len(groups) != 0 {
		t.Fatalf("GroupsOf(bob) after removal = %v, %v; want none", groups, err)
	}
}

func TestMembershipErrors(t *testing.T) {
	f := newFixture(t, "alice", "bob")
	f.createGroup(t, "team", "alice")

	if err := f.repo.AddMember(t.Context(), "team", "alice"); !errors.Is(err, group.ErrAlreadyMember) {
		t.Errorf("AddMember(existing) error = %v, want ErrAlreadyMember", err)
	}
	if err := f.repo.RemoveMember(t.Context(), "team", "bob"); !errors.Is(err, group.ErrNotMember) {
		t.Errorf("RemoveMember(absent) error = %v, want ErrNotMember", err)
	}
	if err := f.repo.RemoveMember(t.Context(), "team", "alice"); !errors.Is(err, group.ErrLastMember) {
		t.Errorf("RemoveMember(last) error = %v, want ErrLastMember", err)
	}
	if err := f.repo.AddMember(t.Context(), "ghost", "alice"); !errors.Is(err, group.ErrNotFound) {
		t.Errorf("AddMember(missing group) error = %v, want ErrNotFound", err)
	}
}

func TestIsMember(t *testing.T) {
	f := newFixture(t, "alice", "bob")
	f.createGroup(t, "admins", "alice")

	cases := []struct {
		name group.Name
		uid  user.UID
		want bool
	}{
		{name: "admins", uid: "alice", want: true},
		{name: "admins", uid: "bob", want: false},
		{name: "ghost", uid: "alice", want: false},
	}
	for _, testCase := range cases {
		got, err := f.repo.IsMember(t.Context(), testCase.name, testCase.uid)
		if err != nil || got != testCase.want {
			t.Errorf("IsMember(%s, %s) = (%v, %v), want (%v, nil)", testCase.name, testCase.uid, got, err, testCase.want)
		}
	}
}

func TestMemberOutsidePeopleHasNoUID(t *testing.T) {
	f := newFixture(t)
	f.stand.CreateGroup(t, f.layout.GroupsDN(), "service", f.stand.Base)

	stored, err := f.repo.Get(t.Context(), "service")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(stored.Members) != 1 || stored.Members[0].UID != "" || stored.Members[0].DN == "" {
		t.Fatalf("Get() members = %+v, want one member with DN only", stored.Members)
	}
}

func TestListSkipsGroupsBreakingDomainRules(t *testing.T) {
	f := newFixture(t, "alice")
	aliceDN := f.layout.UserDN("alice")
	f.stand.CreateGroup(t, f.layout.GroupsDN(), "Mixed Case", aliceDN)
	f.createGroup(t, "team", "alice")

	groups, err := f.repo.List(t.Context())
	if err != nil || len(groups) != 1 || groups[0].Name != "team" {
		t.Fatalf("List() = %v, %v; want only team", groups, err)
	}
}

func TestGroupsOfMissingUserIsUserNotFound(t *testing.T) {
	f := newFixture(t)

	if _, err := f.repo.GroupsOf(t.Context(), "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("GroupsOf() error = %v, want user.ErrNotFound", err)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test -tags integration ./internal/repos/group_repo/`
Expected: FAIL — пакета `group_repo` нет.

- [ ] **Step 3: Реализовать `group_repo`**

`backend/internal/repos/group_repo/repo.go`:
```go
// Package group_repo хранит группы как записи groupOfUniqueNames в ou=groups.
// Выбран именно этот класс: оверлей memberof образа osixia настроен на него и ведёт
// у участников атрибут memberOf.
package group_repo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/tree_layout"
)

const membersAttribute = "uniqueMember"

var groupAttributes = []string{"cn", "description", membersAttribute}

type Repo struct {
	client *ldap_db.Client
	layout tree_layout.Layout
}

func New(client *ldap_db.Client, layout tree_layout.Layout) *Repo {
	return &Repo{client: client, layout: layout}
}

func (r *Repo) List(ctx context.Context) ([]group.Group, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN:     r.layout.GroupsDN(),
		Scope:      ldap_db.ScopeOneLevel,
		Filter:     ldap_db.FilterEquals("objectClass", "groupOfUniqueNames"),
		Attributes: groupAttributes,
	})
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return nil, fmt.Errorf("groups branch %s is missing, apply seed.ldif: %w", r.layout.GroupsDN(), err)
	}
	if err != nil {
		return nil, mapError(err)
	}
	groups := make([]group.Group, 0, len(entries))
	for _, entry := range entries {
		converted, err := r.toGroup(entry)
		if err != nil {
			continue
		}
		groups = append(groups, converted)
	}
	slices.SortFunc(groups, func(left, right group.Group) int {
		return strings.Compare(string(left.Name), string(right.Name))
	})
	return groups, nil
}

func (r *Repo) Get(ctx context.Context, name group.Name) (group.Group, error) {
	return r.getByDN(ctx, r.layout.GroupDN(name))
}

func (r *Repo) Create(ctx context.Context, created group.Group) error {
	memberDNs := make([]string, 0, len(created.Members))
	for _, member := range created.Members {
		memberDNs = append(memberDNs, r.memberDN(member))
	}
	attributes := map[string][]string{
		"objectClass":    {"groupOfUniqueNames"},
		"cn":             {string(created.Name)},
		membersAttribute: memberDNs,
	}
	if created.Description != "" {
		attributes["description"] = []string{created.Description}
	}
	return mapError(r.client.Add(ctx, r.layout.GroupDN(created.Name), attributes))
}

func (r *Repo) Delete(ctx context.Context, name group.Name) error {
	return mapError(r.client.Delete(ctx, r.layout.GroupDN(name)))
}

// AddMember добавляет одно значение (modify add), не трогая остальных участников.
func (r *Repo) AddMember(ctx context.Context, name group.Name, uid user.UID) error {
	err := r.client.Modify(ctx, r.layout.GroupDN(name), []ldap_db.Change{
		{Operation: ldap_db.ChangeAdd, Attribute: membersAttribute, Values: []string{r.layout.UserDN(uid)}},
	})
	if errors.Is(err, ldap_db.ErrValueExists) {
		return group.ErrAlreadyMember
	}
	return mapError(err)
}

func (r *Repo) RemoveMember(ctx context.Context, name group.Name, uid user.UID) error {
	err := r.client.Modify(ctx, r.layout.GroupDN(name), []ldap_db.Change{
		{Operation: ldap_db.ChangeDelete, Attribute: membersAttribute, Values: []string{r.layout.UserDN(uid)}},
	})
	switch {
	case errors.Is(err, ldap_db.ErrNoSuchValue):
		return group.ErrNotMember
	case errors.Is(err, ldap_db.ErrObjectClassViolation):
		return group.ErrLastMember
	default:
		return mapError(err)
	}
}

// GroupsOf читает memberOf пользователя: этот служебный атрибут ведёт оверлей memberof,
// и в обычном поиске он не возвращается — его надо запросить по имени.
func (r *Repo) GroupsOf(ctx context.Context, uid user.UID) ([]group.Group, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: r.layout.UserDN(uid), Scope: ldap_db.ScopeBase, Attributes: []string{"memberOf"},
	})
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return nil, user.ErrNotFound
	}
	if err != nil {
		return nil, mapError(err)
	}
	if len(entries) == 0 {
		return nil, user.ErrNotFound
	}
	return r.groupsByDN(ctx, entries[0].Values("memberOf"))
}

// IsMember использует операцию Compare: сервер отвечает «да/нет», не возвращая записей.
// Если группы нет, участников у неё тоже нет.
func (r *Repo) IsMember(ctx context.Context, name group.Name, uid user.UID) (bool, error) {
	matched, err := r.client.Compare(ctx, r.layout.GroupDN(name), membersAttribute, r.layout.UserDN(uid))
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return false, nil
	}
	return matched, mapError(err)
}

func (r *Repo) groupsByDN(ctx context.Context, groupDNs []string) ([]group.Group, error) {
	groups := make([]group.Group, 0, len(groupDNs))
	for _, groupDN := range groupDNs {
		found, err := r.getByDN(ctx, groupDN)
		if errors.Is(err, group.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		groups = append(groups, found)
	}
	return groups, nil
}

func (r *Repo) getByDN(ctx context.Context, dn string) (group.Group, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: dn, Scope: ldap_db.ScopeBase, Attributes: groupAttributes,
	})
	if err != nil {
		return group.Group{}, mapError(err)
	}
	if len(entries) == 0 {
		return group.Group{}, group.ErrNotFound
	}
	return r.toGroup(entries[0])
}

func (r *Repo) toGroup(entry ldap_db.Entry) (group.Group, error) {
	name, err := group.ParseName(entry.First("cn"))
	if err != nil {
		return group.Group{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	memberDNs := entry.Values(membersAttribute)
	members := make([]group.Member, 0, len(memberDNs))
	for _, memberDN := range memberDNs {
		uid, _ := r.layout.UIDOf(memberDN)
		members = append(members, group.Member{DN: directory.DN(memberDN), UID: uid})
	}
	converted, err := group.New(name, entry.First("description"), members)
	if err != nil {
		return group.Group{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	return converted, nil
}

func (r *Repo) memberDN(member group.Member) string {
	if member.UID != "" {
		return r.layout.UserDN(member.UID)
	}
	return string(member.DN)
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ldap_db.ErrNoSuchObject):
		return group.ErrNotFound
	case errors.Is(err, ldap_db.ErrAlreadyExists):
		return group.ErrAlreadyExists
	case errors.Is(err, ldap_db.ErrObjectClassViolation):
		return fmt.Errorf("%w: rejected by the directory schema", group.ErrInvalid)
	case errors.Is(err, ldap_db.ErrUnavailable):
		return fmt.Errorf("%w: %w", directory.ErrUnavailable, err)
	default:
		return err
	}
}
```

Уточнение к `toGroup`: если в каталоге один и тот же участник записан дважды в разном регистре DN, `group.New` откажет и группа пропадёт из списка. Для учебного каталога это допустимо; такая группа видна в браузере дерева.

- [ ] **Step 4: Запустить тесты**

Run: `cd backend && go test ./... && go test -tags integration ./internal/repos/group_repo/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repos/group_repo
git commit -m "Add group repository on groupOfUniqueNames

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: `directory_repo` и `session_repo`

**Files:**
- Create: `backend/internal/repos/directory_repo/repo.go`
- Create: `backend/internal/repos/session_repo/repo.go`
- Test: `backend/internal/repos/directory_repo/repo_integration_test.go`
- Test: `backend/internal/repos/session_repo/repo_test.go`

**Interfaces:**
- Consumes: `ldap_db.*`, `directory.*`, `session.*`, `ldapstand`.
- Produces (пакет `ldap-admin/internal/repos/directory_repo`):
  - `func New(client *ldap_db.Client, baseDN string) *Repo`
  - `Children(ctx, dn *directory.DN) ([]directory.Node, error)` — `nil` = base DN
  - `Entry(ctx, dn *directory.DN) (directory.Entry, error)` — `nil` = base DN
- Produces (пакет `ldap-admin/internal/repos/session_repo`):
  - `func New() *Repo`; `Save(ctx, session.Session) error`, `Find(ctx, id string) (session.Session, error)`, `Delete(ctx, id string) error`

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/repos/session_repo/repo_test.go`:
```go
package session_repo_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/repos/session_repo"
)

func TestSaveFindDelete(t *testing.T) {
	repo := session_repo.New()
	stored := session.Session{ID: "abc", UID: "alice"}

	if err := repo.Save(t.Context(), stored); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	found, err := repo.Find(t.Context(), "abc")
	if err != nil || found.UID != "alice" {
		t.Fatalf("Find() = %+v, %v", found, err)
	}
	if err := repo.Delete(t.Context(), "abc"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Find(t.Context(), "abc"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Find() after delete error = %v, want ErrNotFound", err)
	}
}
```

`backend/internal/repos/directory_repo/repo_integration_test.go`:
```go
//go:build integration

package directory_repo_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/repos/directory_repo"
	"ldap-admin/internal/testsupport/ldapstand"
)

func dnPointer(value string) *directory.DN {
	dn := directory.DN(value)
	return &dn
}

func TestChildrenOfBase(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	stand.CreatePerson(t, people, "alice")
	stand.CreateOU(t, "groups")
	repo := directory_repo.New(stand.Client, stand.Base)

	nodes, err := repo.Children(t.Context(), nil)
	if err != nil {
		t.Fatalf("Children() error = %v", err)
	}
	if len(nodes) != 2 || nodes[0].RDN != "ou=groups" || nodes[1].RDN != "ou=people" {
		t.Fatalf("Children() = %+v, want groups then people", nodes)
	}
	if nodes[0].HasChildren || !nodes[1].HasChildren {
		t.Fatalf("HasChildren flags = %v/%v, want false/true", nodes[0].HasChildren, nodes[1].HasChildren)
	}
	if len(nodes[1].ObjectClasses) == 0 {
		t.Fatalf("ObjectClasses are empty")
	}
}

func TestEntrySplitsAttributesAndHidesPassword(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	aliceDN := stand.CreatePerson(t, people, "alice")
	if err := stand.Client.SetPassword(t.Context(), aliceDN, "correct-horse"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	repo := directory_repo.New(stand.Client, stand.Base)

	entry, err := repo.Entry(t.Context(), dnPointer(aliceDN))
	if err != nil {
		t.Fatalf("Entry() error = %v", err)
	}
	if len(entry.Attributes["cn"]) == 0 {
		t.Errorf("Attributes have no cn: %v", entry.Attributes)
	}
	for name := range entry.Attributes {
		if name == "userPassword" || name == "userpassword" {
			t.Errorf("userPassword leaked into Attributes")
		}
	}
	if len(entry.OperationalAttributes["entryUUID"]) == 0 || len(entry.OperationalAttributes["structuralObjectClass"]) == 0 {
		t.Errorf("OperationalAttributes = %v, want entryUUID and structuralObjectClass", entry.OperationalAttributes)
	}
}

func TestEntryWithoutDNIsBase(t *testing.T) {
	stand := ldapstand.Connect(t)
	repo := directory_repo.New(stand.Client, stand.Base)

	entry, err := repo.Entry(t.Context(), nil)
	if err != nil || string(entry.DN) != stand.Base {
		t.Fatalf("Entry(nil) = %v, %v; want base %s", entry.DN, err, stand.Base)
	}
}

func TestRejectedDNs(t *testing.T) {
	stand := ldapstand.Connect(t)
	repo := directory_repo.New(stand.Client, stand.Base)

	if _, err := repo.Entry(t.Context(), dnPointer("cn=config")); !errors.Is(err, directory.ErrOutsideBase) {
		t.Errorf("Entry(cn=config) error = %v, want ErrOutsideBase", err)
	}
	if _, err := repo.Children(t.Context(), dnPointer("not a dn")); !errors.Is(err, directory.ErrInvalidDN) {
		t.Errorf("Children(not a dn) error = %v, want ErrInvalidDN", err)
	}
	if _, err := repo.Entry(t.Context(), dnPointer("ou=missing,"+stand.Base)); !errors.Is(err, directory.ErrNotFound) {
		t.Errorf("Entry(missing) error = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/repos/session_repo/ && go test -tags integration ./internal/repos/directory_repo/`
Expected: FAIL — пакетов нет.

- [ ] **Step 3: Реализовать `session_repo`**

`backend/internal/repos/session_repo/repo.go`:
```go
// Package session_repo хранит сессии в памяти процесса: перезапуск бэкенда
// разлогинивает всех, для учебного проекта это принято.
package session_repo

import (
	"context"
	"sync"

	"ldap-admin/internal/domain/session"
)

type Repo struct {
	mutex    sync.Mutex
	sessions map[string]session.Session
}

func New() *Repo {
	return &Repo{sessions: make(map[string]session.Session)}
}

func (r *Repo) Save(_ context.Context, stored session.Session) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.sessions[stored.ID] = stored
	return nil
}

func (r *Repo) Find(_ context.Context, id string) (session.Session, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	found, exists := r.sessions[id]
	if !exists {
		return session.Session{}, session.ErrNotFound
	}
	return found, nil
}

func (r *Repo) Delete(_ context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.sessions, id)
	return nil
}
```

- [ ] **Step 4: Реализовать `directory_repo`**

`backend/internal/repos/directory_repo/repo.go`:
```go
// Package directory_repo читает произвольные записи дерева для браузера каталога.
package directory_repo

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/directory"
)

const (
	// "*" — все обычные атрибуты, "+" — все служебные (RFC 3673). Две выборки вместо
	// одной нужны, чтобы знать, какие атрибуты ведёт сервер.
	allUserAttributes        = "*"
	allOperationalAttributes = "+"
	passwordAttribute        = "userPassword"
)

type Repo struct {
	client *ldap_db.Client
	baseDN string
}

func New(client *ldap_db.Client, baseDN string) *Repo {
	return &Repo{client: client, baseDN: baseDN}
}

func (r *Repo) Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error) {
	parent, err := r.resolve(dn)
	if err != nil {
		return nil, err
	}
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: parent, Scope: ldap_db.ScopeOneLevel, Attributes: []string{"objectClass", "hasSubordinates"},
	})
	if err != nil {
		return nil, mapError(err)
	}
	nodes := make([]directory.Node, 0, len(entries))
	for _, entry := range entries {
		node, err := toNode(entry)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	slices.SortFunc(nodes, func(left, right directory.Node) int { return strings.Compare(left.RDN, right.RDN) })
	return nodes, nil
}

func (r *Repo) Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error) {
	target, err := r.resolve(dn)
	if err != nil {
		return directory.Entry{}, err
	}
	attributes, err := r.readAttributes(ctx, target, allUserAttributes)
	if err != nil {
		return directory.Entry{}, err
	}
	operational, err := r.readAttributes(ctx, target, allOperationalAttributes)
	if err != nil {
		return directory.Entry{}, err
	}
	maps.DeleteFunc(attributes, func(name string, _ []string) bool {
		return strings.EqualFold(name, passwordAttribute)
	})
	return directory.Entry{DN: directory.DN(target), Attributes: attributes, OperationalAttributes: operational}, nil
}

func (r *Repo) readAttributes(ctx context.Context, dn, selector string) (map[string][]string, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: dn, Scope: ldap_db.ScopeBase, Attributes: []string{selector},
	})
	if err != nil {
		return nil, mapError(err)
	}
	if len(entries) == 0 {
		return nil, directory.ErrNotFound
	}
	return entries[0].Attributes, nil
}

// resolve не выпускает браузер за пределы каталога приложения.
func (r *Repo) resolve(dn *directory.DN) (string, error) {
	if dn == nil {
		return r.baseDN, nil
	}
	within, err := ldap_db.IsWithin(string(*dn), r.baseDN)
	if errors.Is(err, ldap_db.ErrInvalidDN) {
		return "", directory.ErrInvalidDN
	}
	if err != nil {
		return "", err
	}
	if !within {
		return "", directory.ErrOutsideBase
	}
	return string(*dn), nil
}

func toNode(entry ldap_db.Entry) (directory.Node, error) {
	rdn, err := ldap_db.FirstRDN(entry.DN)
	if err != nil {
		return directory.Node{}, fmt.Errorf("entry %s: %v", entry.DN, err)
	}
	return directory.Node{
		DN:            directory.DN(entry.DN),
		RDN:           rdn,
		ObjectClasses: entry.Values("objectClass"),
		HasChildren:   strings.EqualFold(entry.First("hasSubordinates"), "TRUE"),
	}, nil
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ldap_db.ErrNoSuchObject):
		return directory.ErrNotFound
	case errors.Is(err, ldap_db.ErrUnavailable):
		return fmt.Errorf("%w: %w", directory.ErrUnavailable, err)
	default:
		return err
	}
}
```

- [ ] **Step 5: Запустить тесты**

Run: `cd backend && go test ./... && go test -tags integration ./internal/repos/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/repos/directory_repo backend/internal/repos/session_repo
git commit -m "Add directory browser and in-memory session repositories

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 8: `auth_service`

**Files:**
- Create: `backend/internal/services/auth_service/service.go`
- Test: `backend/internal/services/auth_service/service_test.go`

**Interfaces:**
- Consumes: `user.*`, `group.Name`, `session.*`, `directory.ErrUnavailable` (задачи 3–4).
- Produces (пакет `ldap-admin/internal/services/auth_service`):
  - Порты: `PasswordVerifier { Verify(ctx, uid user.UID, password string) error }`, `AdminChecker { IsMember(ctx, name group.Name, uid user.UID) (bool, error) }`, `UserReader { Get(ctx, uid user.UID) (user.User, error) }`, `SessionStore { Save(ctx, session.Session) error; Find(ctx, id string) (session.Session, error); Delete(ctx, id string) error }`
  - `func New(passwords PasswordVerifier, admins AdminChecker, users UserReader, sessions SessionStore, clock session.Clock, ids session.IDGenerator, adminsGroup group.Name, sessionTTL time.Duration) *Service`
  - `Login(ctx, rawUID, password string) (session.Session, error)`, `Authenticate(ctx, id string) (session.Session, error)`, `Logout(ctx, id string) error`
  - Реализации портов: `user_repo.Repo` — `PasswordVerifier` и `UserReader`; `group_repo.Repo` — `AdminChecker`; `session_repo.Repo` — `SessionStore`.

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/services/auth_service/service_test.go`:
```go
package auth_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/services/auth_service"
)

const sessionTTL = 8 * time.Hour

var startTime = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

type fakePasswords struct {
	err   error
	calls int
}

func (f *fakePasswords) Verify(_ context.Context, _ user.UID, _ string) error {
	f.calls++
	return f.err
}

type fakeAdmins struct {
	members map[user.UID]bool
	err     error
}

func (f *fakeAdmins) IsMember(_ context.Context, _ group.Name, uid user.UID) (bool, error) {
	return f.members[uid], f.err
}

type fakeUsers map[user.UID]user.User

func (f fakeUsers) Get(_ context.Context, uid user.UID) (user.User, error) {
	found, exists := f[uid]
	if !exists {
		return user.User{}, user.ErrNotFound
	}
	return found, nil
}

type fakeSessions map[string]session.Session

func (f fakeSessions) Save(_ context.Context, stored session.Session) error {
	f[stored.ID] = stored
	return nil
}

func (f fakeSessions) Find(_ context.Context, id string) (session.Session, error) {
	found, exists := f[id]
	if !exists {
		return session.Session{}, session.ErrNotFound
	}
	return found, nil
}

func (f fakeSessions) Delete(_ context.Context, id string) error {
	delete(f, id)
	return nil
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

type sequenceIDs struct{ issued int }

func (g *sequenceIDs) NewID() (string, error) {
	g.issued++
	return fmt.Sprintf("session-%d", g.issued), nil
}

type fixture struct {
	service   *auth_service.Service
	passwords *fakePasswords
	admins    *fakeAdmins
	sessions  fakeSessions
	clock     *fakeClock
}

func newFixture() fixture {
	passwords := &fakePasswords{}
	admins := &fakeAdmins{members: map[user.UID]bool{"alice": true}}
	users := fakeUsers{
		"alice": {UID: "alice", CommonName: "Alice Admin"},
		"bob":   {UID: "bob", CommonName: "Bob"},
	}
	sessions := fakeSessions{}
	clock := &fakeClock{now: startTime}
	service := auth_service.New(passwords, admins, users, sessions, clock, &sequenceIDs{}, "admins", sessionTTL)
	return fixture{service: service, passwords: passwords, admins: admins, sessions: sessions, clock: clock}
}

func TestLoginStartsSessionForAdmin(t *testing.T) {
	f := newFixture()

	started, err := f.service.Login(t.Context(), "alice", "alice-secret")

	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	want := session.Session{ID: "session-1", UID: "alice", CommonName: "Alice Admin", ExpiresAt: startTime.Add(sessionTTL)}
	if started != want {
		t.Fatalf("Login() = %+v, want %+v", started, want)
	}
	if _, saved := f.sessions["session-1"]; !saved {
		t.Fatalf("session was not saved")
	}
}

func TestLoginRejectsMalformedInputWithoutDirectory(t *testing.T) {
	cases := []struct {
		name     string
		uid      string
		password string
	}{
		{name: "uid in another case", uid: "Alice", password: "alice-secret"},
		{name: "filter injection", uid: "a*)(uid=*", password: "alice-secret"},
		{name: "empty password", uid: "alice", password: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture()

			_, err := f.service.Login(t.Context(), testCase.uid, testCase.password)

			if !errors.Is(err, session.ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
			}
			if f.passwords.calls != 0 {
				t.Fatalf("directory was called %d times", f.passwords.calls)
			}
		})
	}
}

func TestLoginWithWrongPassword(t *testing.T) {
	f := newFixture()
	f.passwords.err = session.ErrInvalidCredentials

	_, err := f.service.Login(t.Context(), "alice", "wrong-password")

	if !errors.Is(err, session.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginOfNonAdmin(t *testing.T) {
	f := newFixture()

	_, err := f.service.Login(t.Context(), "bob", "bob-secret")

	if !errors.Is(err, session.ErrNotAdmin) {
		t.Fatalf("Login() error = %v, want ErrNotAdmin", err)
	}
	if len(f.sessions) != 0 {
		t.Fatalf("session saved for a non-admin")
	}
}

func TestLoginPropagatesDirectoryFailure(t *testing.T) {
	f := newFixture()
	f.admins.err = directory.ErrUnavailable

	_, err := f.service.Login(t.Context(), "alice", "alice-secret")

	if !errors.Is(err, directory.ErrUnavailable) {
		t.Fatalf("Login() error = %v, want ErrUnavailable", err)
	}
}

func TestAuthenticate(t *testing.T) {
	f := newFixture()
	started, _ := f.service.Login(t.Context(), "alice", "alice-secret")

	found, err := f.service.Authenticate(t.Context(), started.ID)
	if err != nil || found.UID != "alice" {
		t.Fatalf("Authenticate() = %+v, %v", found, err)
	}

	f.clock.now = started.ExpiresAt
	if _, err := f.service.Authenticate(t.Context(), started.ID); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Authenticate(expired) error = %v, want ErrNotFound", err)
	}
	if _, kept := f.sessions[started.ID]; kept {
		t.Fatalf("expired session was not deleted")
	}
	if _, err := f.service.Authenticate(t.Context(), "missing"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Authenticate(missing) error = %v, want ErrNotFound", err)
	}
}

func TestLogoutDeletesSession(t *testing.T) {
	f := newFixture()
	started, _ := f.service.Login(t.Context(), "alice", "alice-secret")

	if err := f.service.Logout(t.Context(), started.ID); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if len(f.sessions) != 0 {
		t.Fatalf("session is still stored")
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/services/auth_service/`
Expected: FAIL — пакета нет.

- [ ] **Step 3: Реализовать сервис**

`backend/internal/services/auth_service/service.go`:
```go
// Package auth_service — вход в админку: пароль проверяется bind'ом в LDAP, доступ —
// членством в группе админов, дальше работает сессия.
package auth_service

import (
	"context"
	"fmt"
	"time"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

type PasswordVerifier interface {
	Verify(ctx context.Context, uid user.UID, password string) error
}

type AdminChecker interface {
	IsMember(ctx context.Context, name group.Name, uid user.UID) (bool, error)
}

type UserReader interface {
	Get(ctx context.Context, uid user.UID) (user.User, error)
}

type SessionStore interface {
	Save(ctx context.Context, stored session.Session) error
	Find(ctx context.Context, id string) (session.Session, error)
	Delete(ctx context.Context, id string) error
}

type Service struct {
	passwords   PasswordVerifier
	admins      AdminChecker
	users       UserReader
	sessions    SessionStore
	clock       session.Clock
	ids         session.IDGenerator
	adminsGroup group.Name
	sessionTTL  time.Duration
}

func New(
	passwords PasswordVerifier,
	admins AdminChecker,
	users UserReader,
	sessions SessionStore,
	clock session.Clock,
	ids session.IDGenerator,
	adminsGroup group.Name,
	sessionTTL time.Duration,
) *Service {
	return &Service{
		passwords:   passwords,
		admins:      admins,
		users:       users,
		sessions:    sessions,
		clock:       clock,
		ids:         ids,
		adminsGroup: adminsGroup,
		sessionTTL:  sessionTTL,
	}
}

func (s *Service) Login(ctx context.Context, rawUID, password string) (session.Session, error) {
	uid, err := user.ParseUID(rawUID)
	// Ответ одинаков для любой причины отказа: он не должен подсказывать, что именно не так.
	if err != nil || password == "" {
		return session.Session{}, session.ErrInvalidCredentials
	}
	if err := s.passwords.Verify(ctx, uid, password); err != nil {
		return session.Session{}, err
	}
	isAdmin, err := s.admins.IsMember(ctx, s.adminsGroup, uid)
	if err != nil {
		return session.Session{}, err
	}
	if !isAdmin {
		return session.Session{}, session.ErrNotAdmin
	}
	account, err := s.users.Get(ctx, uid)
	if err != nil {
		return session.Session{}, err
	}
	return s.start(ctx, account)
}

func (s *Service) Authenticate(ctx context.Context, id string) (session.Session, error) {
	found, err := s.sessions.Find(ctx, id)
	if err != nil {
		return session.Session{}, err
	}
	if !found.IsExpired(s.clock.Now()) {
		return found, nil
	}
	if err := s.sessions.Delete(ctx, id); err != nil {
		return session.Session{}, err
	}
	return session.Session{}, session.ErrNotFound
}

func (s *Service) Logout(ctx context.Context, id string) error {
	return s.sessions.Delete(ctx, id)
}

func (s *Service) start(ctx context.Context, account user.User) (session.Session, error) {
	id, err := s.ids.NewID()
	if err != nil {
		return session.Session{}, fmt.Errorf("generate session id: %w", err)
	}
	started := session.Session{
		ID:         id,
		UID:        account.UID,
		CommonName: account.CommonName,
		ExpiresAt:  s.clock.Now().Add(s.sessionTTL),
	}
	if err := s.sessions.Save(ctx, started); err != nil {
		return session.Session{}, err
	}
	return started, nil
}
```

- [ ] **Step 4: Запустить тесты**

Run: `cd backend && go test ./internal/services/auth_service/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/auth_service
git commit -m "Add auth service with LDAP bind login and sessions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: `user_service`

**Files:**
- Create: `backend/internal/services/user_service/service.go`
- Test: `backend/internal/services/user_service/service_test.go`

**Interfaces:**
- Consumes: `user.*`, `group.*` (задачи 3–4).
- Produces (пакет `ldap-admin/internal/services/user_service`):
  - Порты: `UserStore { List(ctx) ([]user.User, error); Get(ctx, uid) (user.User, error); Create(ctx, user.User) error; Update(ctx, user.User) error; SetPassword(ctx, uid user.UID, password string) error; Delete(ctx, uid user.UID) error }`, `GroupReader { GroupsOf(ctx, uid user.UID) ([]group.Group, error) }`
  - `func New(users UserStore, groups GroupReader) *Service`
  - `List(ctx) ([]user.User, error)`, `Get(ctx, uid user.UID) (user.User, []group.Name, error)`, `Create(ctx, created user.User, password string) error`, `Update(ctx, updated user.User) error`, `SetPassword(ctx, uid user.UID, password string) error`, `Delete(ctx, actor, uid user.UID) error`
  - Реализации портов: `user_repo.Repo` — `UserStore`, `group_repo.Repo` — `GroupReader`.

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/services/user_service/service_test.go`:
```go
package user_service_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/services/user_service"
)

var errDirectoryHiccup = errors.New("directory hiccup")

type fakeStore struct {
	users          map[user.UID]user.User
	passwords      map[user.UID]string
	setPasswordErr error
	deleted        []user.UID
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[user.UID]user.User{}, passwords: map[user.UID]string{}}
}

func (f *fakeStore) List(_ context.Context) ([]user.User, error) {
	return slices.Collect(maps.Values(f.users)), nil
}

func (f *fakeStore) Get(_ context.Context, uid user.UID) (user.User, error) {
	found, exists := f.users[uid]
	if !exists {
		return user.User{}, user.ErrNotFound
	}
	return found, nil
}

func (f *fakeStore) Create(_ context.Context, created user.User) error {
	if _, exists := f.users[created.UID]; exists {
		return user.ErrAlreadyExists
	}
	f.users[created.UID] = created
	return nil
}

func (f *fakeStore) Update(_ context.Context, updated user.User) error {
	f.users[updated.UID] = updated
	return nil
}

func (f *fakeStore) SetPassword(_ context.Context, uid user.UID, password string) error {
	if f.setPasswordErr != nil {
		return f.setPasswordErr
	}
	f.passwords[uid] = password
	return nil
}

func (f *fakeStore) Delete(_ context.Context, uid user.UID) error {
	f.deleted = append(f.deleted, uid)
	delete(f.users, uid)
	return nil
}

type fakeGroups map[user.UID][]group.Group

func (f fakeGroups) GroupsOf(_ context.Context, uid user.UID) ([]group.Group, error) {
	return f[uid], nil
}

func mustGroup(t *testing.T, name group.Name, uids ...user.UID) group.Group {
	t.Helper()
	members := make([]group.Member, 0, len(uids))
	for _, uid := range uids {
		members = append(members, group.Member{UID: uid})
	}
	created, err := group.New(name, "", members)
	if err != nil {
		t.Fatalf("group.New() error = %v", err)
	}
	return created
}

var john = user.User{UID: "jdoe", CommonName: "John Doe", Surname: "Doe", Emails: []string{}}

func TestCreateSetsPassword(t *testing.T) {
	store := newFakeStore()
	service := user_service.New(store, fakeGroups{})

	if err := service.Create(t.Context(), john, "correct-horse"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if store.passwords["jdoe"] != "correct-horse" {
		t.Fatalf("password was not set")
	}
}

func TestCreateRejectsShortPasswordBeforeWriting(t *testing.T) {
	store := newFakeStore()
	service := user_service.New(store, fakeGroups{})

	err := service.Create(t.Context(), john, "short")

	if !errors.Is(err, user.ErrInvalid) {
		t.Fatalf("Create() error = %v, want ErrInvalid", err)
	}
	if len(store.users) != 0 {
		t.Fatalf("user was written despite invalid password")
	}
}

func TestCreateRollsBackWhenPasswordFails(t *testing.T) {
	store := newFakeStore()
	store.setPasswordErr = errDirectoryHiccup
	service := user_service.New(store, fakeGroups{})

	err := service.Create(t.Context(), john, "correct-horse")

	if !errors.Is(err, errDirectoryHiccup) {
		t.Fatalf("Create() error = %v, want the password error", err)
	}
	if !slices.Equal(store.deleted, []user.UID{"jdoe"}) || len(store.users) != 0 {
		t.Fatalf("user without password was not rolled back: deleted=%v users=%v", store.deleted, store.users)
	}
}

func TestSetPasswordValidatesLength(t *testing.T) {
	service := user_service.New(newFakeStore(), fakeGroups{})

	if err := service.SetPassword(t.Context(), "jdoe", "short"); !errors.Is(err, user.ErrInvalid) {
		t.Fatalf("SetPassword() error = %v, want ErrInvalid", err)
	}
}

func TestGetReturnsGroupNames(t *testing.T) {
	store := newFakeStore()
	store.users["jdoe"] = john
	groups := fakeGroups{"jdoe": {mustGroup(t, "team", "jdoe", "alice")}}
	service := user_service.New(store, groups)

	found, names, err := service.Get(t.Context(), "jdoe")

	if err != nil || found.UID != "jdoe" || !slices.Equal(names, []group.Name{"team"}) {
		t.Fatalf("Get() = %+v, %v, %v", found, names, err)
	}
}

func TestDeleteSelfIsRejected(t *testing.T) {
	store := newFakeStore()
	store.users["alice"] = user.User{UID: "alice"}
	service := user_service.New(store, fakeGroups{})

	err := service.Delete(t.Context(), "alice", "alice")

	if !errors.Is(err, user.ErrSelfDelete) || len(store.deleted) != 0 {
		t.Fatalf("Delete(self) error = %v, deleted = %v", err, store.deleted)
	}
}

func TestDeleteSoleMemberIsRejectedWithGroupNames(t *testing.T) {
	store := newFakeStore()
	store.users["jdoe"] = john
	groups := fakeGroups{"jdoe": {mustGroup(t, "solo", "jdoe"), mustGroup(t, "team", "jdoe", "alice")}}
	service := user_service.New(store, groups)

	err := service.Delete(t.Context(), "alice", "jdoe")

	var soleMember *group.SoleMemberError
	if !errors.As(err, &soleMember) || !slices.Equal(soleMember.Groups, []group.Name{"solo"}) {
		t.Fatalf("Delete() error = %v, want SoleMemberError{solo}", err)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("user was deleted despite sole membership")
	}
}

func TestDeleteRemovesUserSharingGroups(t *testing.T) {
	store := newFakeStore()
	store.users["jdoe"] = john
	groups := fakeGroups{"jdoe": {mustGroup(t, "team", "jdoe", "alice")}}
	service := user_service.New(store, groups)

	if err := service.Delete(t.Context(), "alice", "jdoe"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !slices.Equal(store.deleted, []user.UID{"jdoe"}) {
		t.Fatalf("deleted = %v, want [jdoe]", store.deleted)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/services/user_service/`
Expected: FAIL — пакета нет.

- [ ] **Step 3: Реализовать сервис**

`backend/internal/services/user_service/service.go`:
```go
package user_service

import (
	"context"
	"errors"
	"fmt"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type UserStore interface {
	List(ctx context.Context) ([]user.User, error)
	Get(ctx context.Context, uid user.UID) (user.User, error)
	Create(ctx context.Context, created user.User) error
	Update(ctx context.Context, updated user.User) error
	SetPassword(ctx context.Context, uid user.UID, password string) error
	Delete(ctx context.Context, uid user.UID) error
}

type GroupReader interface {
	GroupsOf(ctx context.Context, uid user.UID) ([]group.Group, error)
}

type Service struct {
	users  UserStore
	groups GroupReader
}

func New(users UserStore, groups GroupReader) *Service {
	return &Service{users: users, groups: groups}
}

func (s *Service) List(ctx context.Context) ([]user.User, error) {
	return s.users.List(ctx)
}

func (s *Service) Get(ctx context.Context, uid user.UID) (user.User, []group.Name, error) {
	found, err := s.users.Get(ctx, uid)
	if err != nil {
		return user.User{}, nil, err
	}
	groups, err := s.groups.GroupsOf(ctx, uid)
	if err != nil {
		return user.User{}, nil, err
	}
	names := make([]group.Name, 0, len(groups))
	for _, membership := range groups {
		names = append(names, membership.Name)
	}
	return found, names, nil
}

// Create — две операции LDAP без общей транзакции: запись и пароль. Если пароль не
// задался, запись удаляется, чтобы в каталоге не остался пользователь без пароля.
func (s *Service) Create(ctx context.Context, created user.User, password string) error {
	if err := user.ValidatePassword(password); err != nil {
		return err
	}
	if err := s.users.Create(ctx, created); err != nil {
		return err
	}
	if err := s.users.SetPassword(ctx, created.UID, password); err != nil {
		rollbackErr := s.users.Delete(ctx, created.UID)
		return errors.Join(fmt.Errorf("set password: %w", err), rollbackErr)
	}
	return nil
}

func (s *Service) Update(ctx context.Context, updated user.User) error {
	return s.users.Update(ctx, updated)
}

func (s *Service) SetPassword(ctx context.Context, uid user.UID, password string) error {
	if err := user.ValidatePassword(password); err != nil {
		return err
	}
	return s.users.SetPassword(ctx, uid, password)
}

// Delete не чистит группы сам: это делает оверлей refint. Но если пользователь —
// единственный участник группы, refint молча оставит в ней ссылку на удалённую запись,
// поэтому такой случай отсекается заранее.
func (s *Service) Delete(ctx context.Context, actor, uid user.UID) error {
	if actor == uid {
		return user.ErrSelfDelete
	}
	groups, err := s.groups.GroupsOf(ctx, uid)
	if err != nil {
		return err
	}
	if blocking := soleMemberships(groups, uid); len(blocking) > 0 {
		return &group.SoleMemberError{Groups: blocking}
	}
	return s.users.Delete(ctx, uid)
}

func soleMemberships(groups []group.Group, uid user.UID) []group.Name {
	var names []group.Name
	for _, membership := range groups {
		if membership.IsSoleMember(uid) {
			names = append(names, membership.Name)
		}
	}
	return names
}
```

- [ ] **Step 4: Запустить тесты**

Run: `cd backend && go test ./internal/services/user_service/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/user_service
git commit -m "Add user service with safe delete and password rollback

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: `group_service` и `directory_service`

**Files:**
- Create: `backend/internal/services/group_service/service.go`
- Create: `backend/internal/services/directory_service/service.go`
- Test: `backend/internal/services/group_service/service_test.go`
- Test: `backend/internal/services/directory_service/service_test.go`

**Interfaces:**
- Consumes: `user.*`, `group.*`, `directory.*`.
- Produces (пакет `ldap-admin/internal/services/group_service`):
  - Порты: `GroupStore { List(ctx) ([]group.Group, error); Get(ctx, name group.Name) (group.Group, error); Create(ctx, group.Group) error; Delete(ctx, name group.Name) error; AddMember(ctx, name group.Name, uid user.UID) error; RemoveMember(ctx, name group.Name, uid user.UID) error }`, `UserReader { Get(ctx, uid user.UID) (user.User, error) }`
  - `func New(groups GroupStore, users UserReader, adminsGroup group.Name) *Service`
  - `List`, `Get(ctx, name) (group.Group, error)`, `Create(ctx, created group.Group) (group.Group, error)`, `Delete(ctx, name) error`, `AddMember(ctx, name, uid) error`, `RemoveMember(ctx, name, uid) error`
- Produces (пакет `ldap-admin/internal/services/directory_service`):
  - Порт: `Browser { Children(ctx, dn *directory.DN) ([]directory.Node, error); Entry(ctx, dn *directory.DN) (directory.Entry, error) }`
  - `func New(browser Browser) *Service`; `Children`, `Entry` с теми же сигнатурами
- Реализации портов: `group_repo.Repo` — `GroupStore`, `user_repo.Repo` — `UserReader`, `directory_repo.Repo` — `Browser`.

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/services/group_service/service_test.go`:
```go
package group_service_test

import (
	"context"
	"errors"
	"testing"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/services/group_service"
)

type fakeStore struct {
	groups  map[group.Name]group.Group
	removed []user.UID
	added   []user.UID
	deleted []group.Name
}

func (f *fakeStore) List(_ context.Context) ([]group.Group, error) { return nil, nil }

func (f *fakeStore) Get(_ context.Context, name group.Name) (group.Group, error) {
	found, exists := f.groups[name]
	if !exists {
		return group.Group{}, group.ErrNotFound
	}
	return found, nil
}

func (f *fakeStore) Create(_ context.Context, created group.Group) error {
	f.groups[created.Name] = created
	return nil
}

func (f *fakeStore) Delete(_ context.Context, name group.Name) error {
	f.deleted = append(f.deleted, name)
	return nil
}

func (f *fakeStore) AddMember(_ context.Context, _ group.Name, uid user.UID) error {
	f.added = append(f.added, uid)
	return nil
}

func (f *fakeStore) RemoveMember(_ context.Context, _ group.Name, uid user.UID) error {
	f.removed = append(f.removed, uid)
	return nil
}

type fakeUsers map[user.UID]bool

func (f fakeUsers) Get(_ context.Context, uid user.UID) (user.User, error) {
	if !f[uid] {
		return user.User{}, user.ErrNotFound
	}
	return user.User{UID: uid}, nil
}

func newService(t *testing.T, groups ...group.Group) (*group_service.Service, *fakeStore) {
	t.Helper()
	store := &fakeStore{groups: map[group.Name]group.Group{}}
	for _, existing := range groups {
		store.groups[existing.Name] = existing
	}
	users := fakeUsers{"alice": true, "bob": true}
	return group_service.New(store, users, "admins"), store
}

func mustGroup(t *testing.T, name group.Name, uids ...user.UID) group.Group {
	t.Helper()
	members := make([]group.Member, 0, len(uids))
	for _, uid := range uids {
		members = append(members, group.Member{UID: uid})
	}
	created, err := group.New(name, "", members)
	if err != nil {
		t.Fatalf("group.New() error = %v", err)
	}
	return created
}

func TestCreateChecksMembersExist(t *testing.T) {
	service, store := newService(t)

	_, err := service.Create(t.Context(), mustGroup(t, "team", "alice", "ghost"))

	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("Create() error = %v, want user.ErrNotFound", err)
	}
	if len(store.groups) != 0 {
		t.Fatalf("group was written despite missing member")
	}
}

func TestCreateReturnsStoredGroup(t *testing.T) {
	service, _ := newService(t)

	created, err := service.Create(t.Context(), mustGroup(t, "team", "alice"))

	if err != nil || created.Name != "team" {
		t.Fatalf("Create() = %+v, %v", created, err)
	}
}

func TestDeleteAdminsGroupIsProtected(t *testing.T) {
	service, store := newService(t, mustGroup(t, "admins", "alice"))

	if err := service.Delete(t.Context(), "admins"); !errors.Is(err, group.ErrProtected) {
		t.Fatalf("Delete(admins) error = %v, want ErrProtected", err)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("protected group was deleted")
	}
}

func TestAddMemberRequiresExistingUser(t *testing.T) {
	service, store := newService(t, mustGroup(t, "team", "alice"))

	if err := service.AddMember(t.Context(), "team", "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("AddMember(ghost) error = %v, want user.ErrNotFound", err)
	}
	if err := service.AddMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("AddMember(bob) error = %v", err)
	}
	if len(store.added) != 1 || store.added[0] != "bob" {
		t.Fatalf("added = %v, want [bob]", store.added)
	}
}

func TestRemoveMemberChecksDomainRulesFirst(t *testing.T) {
	service, store := newService(t, mustGroup(t, "solo", "alice"), mustGroup(t, "team", "alice", "bob"))

	if err := service.RemoveMember(t.Context(), "solo", "alice"); !errors.Is(err, group.ErrLastMember) {
		t.Errorf("RemoveMember(last) error = %v, want ErrLastMember", err)
	}
	if err := service.RemoveMember(t.Context(), "team", "carol"); !errors.Is(err, group.ErrNotMember) {
		t.Errorf("RemoveMember(stranger) error = %v, want ErrNotMember", err)
	}
	if err := service.RemoveMember(t.Context(), "ghost", "alice"); !errors.Is(err, group.ErrNotFound) {
		t.Errorf("RemoveMember(missing group) error = %v, want ErrNotFound", err)
	}
	if len(store.removed) != 0 {
		t.Fatalf("store was called for rejected removals: %v", store.removed)
	}
	if err := service.RemoveMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("RemoveMember(bob) error = %v", err)
	}
}
```

`backend/internal/services/directory_service/service_test.go`:
```go
package directory_service_test

import (
	"context"
	"testing"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/services/directory_service"
)

type recordingBrowser struct {
	requested []*directory.DN
}

func (b *recordingBrowser) Children(_ context.Context, dn *directory.DN) ([]directory.Node, error) {
	b.requested = append(b.requested, dn)
	return []directory.Node{{DN: "ou=people,dc=example,dc=com", RDN: "ou=people"}}, nil
}

func (b *recordingBrowser) Entry(_ context.Context, dn *directory.DN) (directory.Entry, error) {
	b.requested = append(b.requested, dn)
	return directory.Entry{DN: "dc=example,dc=com"}, nil
}

func TestServicePassesRequestedDNThrough(t *testing.T) {
	browser := &recordingBrowser{}
	service := directory_service.New(browser)
	people := directory.DN("ou=people,dc=example,dc=com")

	nodes, err := service.Children(t.Context(), nil)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("Children() = %v, %v", nodes, err)
	}
	if _, err := service.Entry(t.Context(), &people); err != nil {
		t.Fatalf("Entry() error = %v", err)
	}
	if browser.requested[0] != nil || *browser.requested[1] != people {
		t.Fatalf("requested = %v, want [nil, people]", browser.requested)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/services/...`
Expected: FAIL — пакетов `group_service` и `directory_service` нет.

- [ ] **Step 3: Реализовать `group_service`**

`backend/internal/services/group_service/service.go`:
```go
package group_service

import (
	"context"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type GroupStore interface {
	List(ctx context.Context) ([]group.Group, error)
	Get(ctx context.Context, name group.Name) (group.Group, error)
	Create(ctx context.Context, created group.Group) error
	Delete(ctx context.Context, name group.Name) error
	AddMember(ctx context.Context, name group.Name, uid user.UID) error
	RemoveMember(ctx context.Context, name group.Name, uid user.UID) error
}

type UserReader interface {
	Get(ctx context.Context, uid user.UID) (user.User, error)
}

type Service struct {
	groups      GroupStore
	users       UserReader
	adminsGroup group.Name
}

func New(groups GroupStore, users UserReader, adminsGroup group.Name) *Service {
	return &Service{groups: groups, users: users, adminsGroup: adminsGroup}
}

func (s *Service) List(ctx context.Context) ([]group.Group, error) {
	return s.groups.List(ctx)
}

func (s *Service) Get(ctx context.Context, name group.Name) (group.Group, error) {
	return s.groups.Get(ctx, name)
}

// Create проверяет участников заранее: атрибут uniqueMember примет DN и несуществующей
// записи — синтаксис проверяет только формат DN. Возвращает группу, перечитанную из
// каталога: DN участников знает только репозиторий.
func (s *Service) Create(ctx context.Context, created group.Group) (group.Group, error) {
	if err := s.requireUsers(ctx, created.Members); err != nil {
		return group.Group{}, err
	}
	if err := s.groups.Create(ctx, created); err != nil {
		return group.Group{}, err
	}
	return s.groups.Get(ctx, created.Name)
}

// Delete не даёт удалить группу админов: без неё в админку не войдёт никто.
func (s *Service) Delete(ctx context.Context, name group.Name) error {
	if name == s.adminsGroup {
		return group.ErrProtected
	}
	return s.groups.Delete(ctx, name)
}

func (s *Service) AddMember(ctx context.Context, name group.Name, uid user.UID) error {
	if _, err := s.users.Get(ctx, uid); err != nil {
		return err
	}
	return s.groups.AddMember(ctx, name, uid)
}

func (s *Service) RemoveMember(ctx context.Context, name group.Name, uid user.UID) error {
	current, err := s.groups.Get(ctx, name)
	if err != nil {
		return err
	}
	if err := current.CheckRemoval(uid); err != nil {
		return err
	}
	return s.groups.RemoveMember(ctx, name, uid)
}

func (s *Service) requireUsers(ctx context.Context, members []group.Member) error {
	for _, member := range members {
		if member.UID == "" {
			continue
		}
		if _, err := s.users.Get(ctx, member.UID); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: Реализовать `directory_service`**

`backend/internal/services/directory_service/service.go`:
```go
// Package directory_service — просмотр дерева. Бизнес-правил у просмотра нет, сервис
// тонкий и существует ради единообразия слоёв.
package directory_service

import (
	"context"

	"ldap-admin/internal/domain/directory"
)

type Browser interface {
	Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error)
	Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error)
}

type Service struct {
	browser Browser
}

func New(browser Browser) *Service {
	return &Service{browser: browser}
}

func (s *Service) Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error) {
	return s.browser.Children(ctx, dn)
}

func (s *Service) Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error) {
	return s.browser.Entry(ctx, dn)
}
```

- [ ] **Step 5: Запустить тесты**

Run: `cd backend && go test ./internal/services/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/services/group_service backend/internal/services/directory_service
git commit -m "Add group and directory services

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 11: HTTP — JSON, ошибки, сессия, вход

**Files:**
- Create: `backend/internal/api/httpjson/httpjson.go`
- Create: `backend/internal/api/sessionctx/sessionctx.go`
- Create: `backend/internal/api/auth/handler.go`
- Test: `backend/internal/api/httpjson/httpjson_test.go`
- Test: `backend/internal/api/auth/handler_test.go`

**Interfaces:**
- Consumes: доменные ошибки всех пакетов (задачи 3–4); сигнатуры `auth_service.Service` (задача 8).
- Produces (пакет `ldap-admin/internal/api/httpjson`):
  - `type ErrorBody struct { Error string \`json:"error"\`; Message string \`json:"message"\`; Groups []string \`json:"groups,omitempty"\` }`
  - `func Write(w http.ResponseWriter, status int, body any)`
  - `func Decode(w http.ResponseWriter, r *http.Request, target any) error` — неизвестные поля и битый JSON → `ErrMalformedBody`
  - `func WriteError(w http.ResponseWriter, logger *slog.Logger, err error)` — таблица раздела 5.5 спецификации
  - `var ErrMalformedBody`
- Produces (пакет `ldap-admin/internal/api/sessionctx`): `func With(ctx, session.Session) context.Context`, `func From(ctx) (session.Session, bool)`
- Produces (пакет `ldap-admin/internal/api/auth`):
  - `const CookieName = "ldap_admin_session"`
  - `type Service interface { Login(ctx, uid, password string) (session.Session, error); Authenticate(ctx, id string) (session.Session, error); Logout(ctx, id string) error }`
  - `func New(service Service, logger *slog.Logger) *Handler`
  - `(*Handler).Login`, `Logout`, `Me` — `http.HandlerFunc`; `(*Handler).RequireSession(next http.Handler) http.Handler`

- [ ] **Step 1: Написать падающие тесты `httpjson`**

`backend/internal/api/httpjson/httpjson_test.go`:
```go
package httpjson_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func writeError(t *testing.T, err error) (int, httpjson.ErrorBody) {
	t.Helper()
	recorder := httptest.NewRecorder()
	httpjson.WriteError(recorder, silentLogger, err)
	var body httpjson.ErrorBody
	if decodeErr := json.NewDecoder(recorder.Body).Decode(&body); decodeErr != nil {
		t.Fatalf("decode body: %v", decodeErr)
	}
	return recorder.Code, body
}

func TestWriteErrorMapsDomainErrors(t *testing.T) {
	driverFailure := errors.New("dial tcp 127.0.0.1:389: connection refused")
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		message string
	}{
		{"invalid input keeps details", fmt.Errorf("%w: surname is required", user.ErrInvalid), 400, "invalid_input", "invalid user data: surname is required"},
		{"dn outside base", directory.ErrOutsideBase, 400, "invalid_input", "dn is outside the directory base"},
		{"malformed dn", directory.ErrInvalidDN, 400, "invalid_input", "invalid dn"},
		{"no session", session.ErrNotFound, 401, "unauthenticated", "session not found or expired"},
		{"bad credentials", session.ErrInvalidCredentials, 401, "invalid_credentials", "invalid uid or password"},
		{"not admin", session.ErrNotAdmin, 403, "not_admin", "user is not a member of the admins group"},
		{"user missing", user.ErrNotFound, 404, "not_found", "user not found"},
		{"not a member", group.ErrNotMember, 404, "not_found", "user is not a member of the group"},
		{"already member", group.ErrAlreadyMember, 409, "already_exists", "user is already a member of the group"},
		{"last member", group.ErrLastMember, 409, "last_member", "cannot remove the last member of a group"},
		{"protected group", group.ErrProtected, 409, "protected", "this group is protected"},
		{"self delete", user.ErrSelfDelete, 409, "self_delete", "you cannot delete yourself"},
		{"directory down hides driver text", fmt.Errorf("%w: %w", directory.ErrUnavailable, driverFailure), 503, "directory_unavailable", "directory unavailable"},
		{"unknown error hides details", errors.New("ldap: service account bind failed: details"), 500, "internal", "internal server error"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			status, body := writeError(t, testCase.err)
			if status != testCase.status || body.Error != testCase.code || body.Message != testCase.message {
				t.Fatalf("WriteError() = %d %+v, want %d %s %q", status, body, testCase.status, testCase.code, testCase.message)
			}
		})
	}
}

func TestWriteErrorListsSoleMemberGroups(t *testing.T) {
	status, body := writeError(t, &group.SoleMemberError{Groups: []group.Name{"solo"}})

	if status != http.StatusConflict || body.Error != "sole_member" || !slices.Equal(body.Groups, []string{"solo"}) {
		t.Fatalf("WriteError() = %d %+v", status, body)
	}
}

func TestDecodeRejectsUnknownFieldsAndBrokenJSON(t *testing.T) {
	for _, payload := range []string{`{"uid":"a","extra":1}`, `{"uid":`} {
		request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
		var target struct {
			UID string `json:"uid"`
		}
		err := httpjson.Decode(httptest.NewRecorder(), request, &target)
		if !errors.Is(err, httpjson.ErrMalformedBody) {
			t.Errorf("Decode(%s) error = %v, want ErrMalformedBody", payload, err)
		}
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/api/httpjson/`
Expected: FAIL — пакета нет.

- [ ] **Step 3: Реализовать `httpjson` и `sessionctx`**

`backend/internal/api/httpjson/httpjson.go`:
```go
// Package httpjson пишет JSON-ответы и переводит доменные ошибки в HTTP-коды.
package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

const maxBodyBytes = 1 << 20

var ErrMalformedBody = errors.New("malformed request body")

type ErrorBody struct {
	Error   string   `json:"error"`
	Message string   `json:"message"`
	Groups  []string `json:"groups,omitempty"`
}

type errorRule struct {
	target error
	status int
	code   string
	// detailed — в ответ уходит полный текст ошибки, а не только текст sentinel.
	// Так только для ошибок ввода: их текст собирает домен, деталей драйвера в нём нет.
	detailed bool
}

var errorRules = []errorRule{
	{target: user.ErrInvalid, status: http.StatusBadRequest, code: "invalid_input", detailed: true},
	{target: group.ErrInvalid, status: http.StatusBadRequest, code: "invalid_input", detailed: true},
	{target: ErrMalformedBody, status: http.StatusBadRequest, code: "invalid_input", detailed: true},
	{target: directory.ErrOutsideBase, status: http.StatusBadRequest, code: "invalid_input"},
	{target: directory.ErrInvalidDN, status: http.StatusBadRequest, code: "invalid_input"},
	{target: session.ErrNotFound, status: http.StatusUnauthorized, code: "unauthenticated"},
	{target: session.ErrInvalidCredentials, status: http.StatusUnauthorized, code: "invalid_credentials"},
	{target: session.ErrNotAdmin, status: http.StatusForbidden, code: "not_admin"},
	{target: user.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
	{target: group.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
	{target: directory.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
	{target: group.ErrNotMember, status: http.StatusNotFound, code: "not_found"},
	{target: user.ErrAlreadyExists, status: http.StatusConflict, code: "already_exists"},
	{target: group.ErrAlreadyExists, status: http.StatusConflict, code: "already_exists"},
	{target: group.ErrAlreadyMember, status: http.StatusConflict, code: "already_exists"},
	{target: group.ErrLastMember, status: http.StatusConflict, code: "last_member"},
	{target: group.ErrProtected, status: http.StatusConflict, code: "protected"},
	{target: user.ErrSelfDelete, status: http.StatusConflict, code: "self_delete"},
	{target: directory.ErrUnavailable, status: http.StatusServiceUnavailable, code: "directory_unavailable"},
}

func Write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

func Decode(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", ErrMalformedBody, err)
	}
	return nil
}

func WriteError(w http.ResponseWriter, logger *slog.Logger, err error) {
	var soleMember *group.SoleMemberError
	if errors.As(err, &soleMember) {
		Write(w, http.StatusConflict, ErrorBody{Error: "sole_member", Message: err.Error(), Groups: groupNames(soleMember.Groups)})
		return
	}
	for _, rule := range errorRules {
		if !errors.Is(err, rule.target) {
			continue
		}
		if rule.status >= http.StatusInternalServerError {
			logger.Error("request failed", "error", err)
		}
		Write(w, rule.status, ErrorBody{Error: rule.code, Message: messageFor(rule, err)})
		return
	}
	logger.Error("unexpected error", "error", err)
	Write(w, http.StatusInternalServerError, ErrorBody{Error: "internal", Message: "internal server error"})
}

func messageFor(rule errorRule, err error) string {
	if rule.detailed {
		return err.Error()
	}
	return rule.target.Error()
}

func groupNames(names []group.Name) []string {
	converted := make([]string, 0, len(names))
	for _, name := range names {
		converted = append(converted, string(name))
	}
	return converted
}
```

`backend/internal/api/sessionctx/sessionctx.go`:
```go
// Package sessionctx переносит сессию вошедшего пользователя через context запроса:
// её кладёт middleware проверки сессии, читают хендлеры.
package sessionctx

import (
	"context"

	"ldap-admin/internal/domain/session"
)

type contextKey struct{}

func With(ctx context.Context, current session.Session) context.Context {
	return context.WithValue(ctx, contextKey{}, current)
}

func From(ctx context.Context) (session.Session, bool) {
	current, found := ctx.Value(contextKey{}).(session.Session)
	return current, found
}
```

- [ ] **Step 4: Запустить тесты `httpjson`**

Run: `cd backend && go test ./internal/api/httpjson/`
Expected: PASS.

- [ ] **Step 5: Написать падающие тесты хендлера входа**

`backend/internal/api/auth/handler_test.go`:
```go
package auth_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"ldap-admin/internal/api/auth"
	"ldap-admin/internal/api/sessionctx"
	"ldap-admin/internal/domain/session"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

var aliceSession = session.Session{
	ID: "session-1", UID: "alice", CommonName: "Alice Admin",
	ExpiresAt: time.Date(2026, 10, 7, 20, 0, 0, 0, time.UTC),
}

type fakeService struct {
	loginErr  error
	sessions  map[string]session.Session
	loggedOut []string
}

func (f *fakeService) Login(_ context.Context, _, _ string) (session.Session, error) {
	if f.loginErr != nil {
		return session.Session{}, f.loginErr
	}
	return aliceSession, nil
}

func (f *fakeService) Authenticate(_ context.Context, id string) (session.Session, error) {
	found, exists := f.sessions[id]
	if !exists {
		return session.Session{}, session.ErrNotFound
	}
	return found, nil
}

func (f *fakeService) Logout(_ context.Context, id string) error {
	f.loggedOut = append(f.loggedOut, id)
	return nil
}

func postLogin(handler *auth.Handler, payload string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler.Login(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(payload)))
	return recorder
}

func TestLoginSetsSessionCookie(t *testing.T) {
	handler := auth.New(&fakeService{}, silentLogger)

	recorder := postLogin(handler, `{"uid":"alice","password":"alice-secret"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	var body map[string]string
	_ = json.NewDecoder(recorder.Body).Decode(&body)
	if body["uid"] != "alice" || body["cn"] != "Alice Admin" {
		t.Fatalf("body = %v", body)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v, want one", cookies)
	}
	cookie := cookies[0]
	if cookie.Name != auth.CookieName || cookie.Value != "session-1" || !cookie.HttpOnly ||
		cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api" {
		t.Fatalf("cookie = %+v", cookie)
	}
}

func TestLoginErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		payload string
		status  int
	}{
		{name: "invalid credentials", err: session.ErrInvalidCredentials, payload: `{"uid":"alice","password":"x"}`, status: http.StatusUnauthorized},
		{name: "not admin", err: session.ErrNotAdmin, payload: `{"uid":"bob","password":"bob-secret"}`, status: http.StatusForbidden},
		{name: "malformed json", payload: `{"uid":`, status: http.StatusBadRequest},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			handler := auth.New(&fakeService{loginErr: testCase.err}, silentLogger)

			recorder := postLogin(handler, testCase.payload)

			if recorder.Code != testCase.status || len(recorder.Result().Cookies()) != 0 {
				t.Fatalf("status = %d, cookies = %v", recorder.Code, recorder.Result().Cookies())
			}
		})
	}
}

func TestRequireSessionRejectsMissingOrUnknownCookie(t *testing.T) {
	handler := auth.New(&fakeService{sessions: map[string]session.Session{}}, silentLogger)
	protected := handler.RequireSession(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatalf("protected handler was called")
	}))

	for _, cookieValue := range []string{"", "forged"} {
		request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		if cookieValue != "" {
			request.AddCookie(&http.Cookie{Name: auth.CookieName, Value: cookieValue})
		}
		recorder := httptest.NewRecorder()
		protected.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("cookie %q: status = %d, want 401", cookieValue, recorder.Code)
		}
	}
}

func TestRequireSessionPassesSessionToHandler(t *testing.T) {
	handler := auth.New(&fakeService{sessions: map[string]session.Session{"session-1": aliceSession}}, silentLogger)
	var seen session.Session
	protected := handler.RequireSession(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, _ = sessionctx.From(r.Context())
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	request.AddCookie(&http.Cookie{Name: auth.CookieName, Value: "session-1"})

	protected.ServeHTTP(httptest.NewRecorder(), request)

	if seen.UID != "alice" {
		t.Fatalf("session in context = %+v", seen)
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	service := &fakeService{}
	handler := auth.New(service, silentLogger)
	request := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	request = request.WithContext(sessionctx.With(request.Context(), aliceSession))
	recorder := httptest.NewRecorder()

	handler.Logout(recorder, request)

	if recorder.Code != http.StatusNoContent || len(service.loggedOut) != 1 || service.loggedOut[0] != "session-1" {
		t.Fatalf("status = %d, loggedOut = %v", recorder.Code, service.loggedOut)
	}
	cookies := recorder.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("cookies = %+v, want an expired cookie", cookies)
	}
}

func TestMeReturnsCurrentUser(t *testing.T) {
	handler := auth.New(&fakeService{}, silentLogger)
	request := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	request = request.WithContext(sessionctx.With(request.Context(), aliceSession))
	recorder := httptest.NewRecorder()

	handler.Me(recorder, request)

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"cn":"Alice Admin"`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}
```

- [ ] **Step 6: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/api/auth/`
Expected: FAIL — пакета `auth` нет.

- [ ] **Step 7: Реализовать хендлер входа**

`backend/internal/api/auth/handler.go`:
```go
// Package auth — HTTP-вход: выдача cookie сессии и middleware, пускающий дальше
// только с действующей сессией.
package auth

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/api/sessionctx"
	"ldap-admin/internal/domain/session"
)

const (
	CookieName = "ldap_admin_session"
	cookiePath = "/api"
)

type Service interface {
	Login(ctx context.Context, uid, password string) (session.Session, error)
	Authenticate(ctx context.Context, id string) (session.Session, error)
	Logout(ctx context.Context, id string) error
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type loginRequest struct {
	UID      string `json:"uid"`
	Password string `json:"password"`
}

type currentUserResponse struct {
	UID        string `json:"uid"`
	CommonName string `json:"cn"`
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request loginRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	started, err := h.service.Login(r.Context(), request.UID, request.Password)
	if err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	http.SetCookie(w, sessionCookie(started))
	httpjson.Write(w, http.StatusOK, toCurrentUser(started))
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	current, found := sessionctx.From(r.Context())
	if !found {
		httpjson.WriteError(w, h.logger, session.ErrNotFound)
		return
	}
	if err := h.service.Logout(r.Context(), current.ID); err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	http.SetCookie(w, expiredCookie())
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	current, found := sessionctx.From(r.Context())
	if !found {
		httpjson.WriteError(w, h.logger, session.ErrNotFound)
		return
	}
	httpjson.Write(w, http.StatusOK, toCurrentUser(current))
}

func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(CookieName)
		if err != nil {
			httpjson.WriteError(w, h.logger, session.ErrNotFound)
			return
		}
		current, err := h.service.Authenticate(r.Context(), cookie.Value)
		if err != nil {
			httpjson.WriteError(w, h.logger, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(sessionctx.With(r.Context(), current)))
	})
}

// sessionCookie недоступна скриптам страницы (HttpOnly) и не уходит с запросами
// с чужих сайтов (SameSite=Strict) — этого достаточно вместо отдельной CSRF-защиты.
func sessionCookie(started session.Session) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    started.ID,
		Path:     cookiePath,
		Expires:  started.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func expiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Path:     cookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func toCurrentUser(current session.Session) currentUserResponse {
	return currentUserResponse{UID: string(current.UID), CommonName: current.CommonName}
}
```

- [ ] **Step 8: Запустить тесты**

Run: `cd backend && go test ./internal/api/...`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add backend/internal/api/httpjson backend/internal/api/sessionctx backend/internal/api/auth
git commit -m "Add HTTP error mapping and session login handlers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: HTTP — пользователи

**Files:**
- Create: `backend/internal/api/users/handler.go`
- Test: `backend/internal/api/users/handler_test.go`

**Interfaces:**
- Consumes: `httpjson`, `sessionctx` (задача 11); сигнатуры `user_service.Service` (задача 9).
- Produces (пакет `ldap-admin/internal/api/users`):
  - `type Service interface { List(ctx) ([]user.User, error); Get(ctx, uid user.UID) (user.User, []group.Name, error); Create(ctx, user.User, password string) error; Update(ctx, user.User) error; SetPassword(ctx, uid user.UID, password string) error; Delete(ctx, actor, uid user.UID) error }`
  - `func New(service Service, logger *slog.Logger) *Handler`
  - `List`, `Get`, `Create`, `Update`, `SetPassword`, `Delete` — `http.HandlerFunc`; путь задаёт параметр `{uid}`
  - JSON: пользователь `{uid, cn, sn, mail[]}`, карточка — то же плюс `groups[]`

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/api/users/handler_test.go`:
```go
package users_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ldap-admin/internal/api/sessionctx"
	"ldap-admin/internal/api/users"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeService struct {
	users       []user.User
	created     []user.User
	createErr   error
	deleteErr   error
	deleteCalls [][2]user.UID
}

func (f *fakeService) List(_ context.Context) ([]user.User, error) { return f.users, nil }

func (f *fakeService) Get(_ context.Context, uid user.UID) (user.User, []group.Name, error) {
	for _, stored := range f.users {
		if stored.UID == uid {
			return stored, []group.Name{"admins"}, nil
		}
	}
	return user.User{}, nil, user.ErrNotFound
}

func (f *fakeService) Create(_ context.Context, created user.User, _ string) error {
	f.created = append(f.created, created)
	return f.createErr
}

func (f *fakeService) Update(_ context.Context, _ user.User) error { return nil }

func (f *fakeService) SetPassword(_ context.Context, _ user.UID, _ string) error { return nil }

func (f *fakeService) Delete(_ context.Context, actor, uid user.UID) error {
	f.deleteCalls = append(f.deleteCalls, [2]user.UID{actor, uid})
	return f.deleteErr
}

func serve(handler http.HandlerFunc, method, uid, payload string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/api/users", strings.NewReader(payload))
	request.SetPathValue("uid", uid)
	request = request.WithContext(sessionctx.With(request.Context(), session.Session{UID: "alice"}))
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

func TestListReturnsEmptyArrayNotNull(t *testing.T) {
	handler := users.New(&fakeService{}, silentLogger)

	recorder := serve(handler.List, http.MethodGet, "", "")

	if recorder.Code != http.StatusOK || strings.TrimSpace(recorder.Body.String()) != "[]" {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}

func TestGetReturnsGroups(t *testing.T) {
	service := &fakeService{users: []user.User{{UID: "jdoe", CommonName: "John", Surname: "Doe", Emails: []string{}}}}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Get, http.MethodGet, "jdoe", "")

	var body map[string]any
	_ = json.NewDecoder(recorder.Body).Decode(&body)
	if recorder.Code != http.StatusOK || body["uid"] != "jdoe" || body["groups"] == nil {
		t.Fatalf("status = %d, body = %v", recorder.Code, body)
	}
	if missing := serve(handler.Get, http.MethodGet, "ghost", ""); missing.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d, want 404", missing.Code)
	}
}

func TestCreate(t *testing.T) {
	service := &fakeService{}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "",
		`{"uid":"jdoe","cn":"Doe, John","sn":"Doe","mail":["jdoe@example.com"],"password":"correct-horse"}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	if len(service.created) != 1 || service.created[0].CommonName != "Doe, John" {
		t.Fatalf("created = %+v", service.created)
	}
}

func TestCreateRejectsInvalidInputBeforeService(t *testing.T) {
	service := &fakeService{}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "", `{"uid":"John","cn":"John","sn":"Doe","password":"correct-horse"}`)

	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "invalid_input") {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
	if len(service.created) != 0 {
		t.Fatalf("service was called with invalid input")
	}
}

func TestCreateDuplicateIsConflict(t *testing.T) {
	handler := users.New(&fakeService{createErr: user.ErrAlreadyExists}, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "", `{"uid":"jdoe","cn":"John","sn":"Doe","password":"correct-horse"}`)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
}

func TestDeletePassesActorFromSession(t *testing.T) {
	service := &fakeService{}
	handler := users.New(service, silentLogger)

	recorder := serve(handler.Delete, http.MethodDelete, "jdoe", "")

	if recorder.Code != http.StatusNoContent || len(service.deleteCalls) != 1 || service.deleteCalls[0] != [2]user.UID{"alice", "jdoe"} {
		t.Fatalf("status = %d, calls = %v", recorder.Code, service.deleteCalls)
	}
}

func TestDeleteSoleMemberListsGroups(t *testing.T) {
	handler := users.New(&fakeService{deleteErr: &group.SoleMemberError{Groups: []group.Name{"solo"}}}, silentLogger)

	recorder := serve(handler.Delete, http.MethodDelete, "jdoe", "")

	if recorder.Code != http.StatusConflict || !strings.Contains(recorder.Body.String(), `"groups":["solo"]`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/api/users/`
Expected: FAIL — пакета нет.

- [ ] **Step 3: Реализовать хендлер**

`backend/internal/api/users/handler.go`:
```go
package users

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/api/sessionctx"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

type Service interface {
	List(ctx context.Context) ([]user.User, error)
	Get(ctx context.Context, uid user.UID) (user.User, []group.Name, error)
	Create(ctx context.Context, created user.User, password string) error
	Update(ctx context.Context, updated user.User) error
	SetPassword(ctx context.Context, uid user.UID, password string) error
	Delete(ctx context.Context, actor, uid user.UID) error
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type userResponse struct {
	UID        string   `json:"uid"`
	CommonName string   `json:"cn"`
	Surname    string   `json:"sn"`
	Emails     []string `json:"mail"`
}

type userDetailsResponse struct {
	userResponse
	Groups []string `json:"groups"`
}

type createUserRequest struct {
	UID        string   `json:"uid"`
	CommonName string   `json:"cn"`
	Surname    string   `json:"sn"`
	Emails     []string `json:"mail"`
	Password   string   `json:"password"`
}

type updateUserRequest struct {
	CommonName string   `json:"cn"`
	Surname    string   `json:"sn"`
	Emails     []string `json:"mail"`
}

type passwordRequest struct {
	Password string `json:"password"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	found, err := h.service.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	response := make([]userResponse, 0, len(found))
	for _, account := range found {
		response = append(response, toUserResponse(account))
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	found, groups, err := h.service.Get(r.Context(), uid)
	if err != nil {
		h.fail(w, err)
		return
	}
	names := make([]string, 0, len(groups))
	for _, name := range groups {
		names = append(names, string(name))
	}
	httpjson.Write(w, http.StatusOK, userDetailsResponse{userResponse: toUserResponse(found), Groups: names})
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createUserRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	created, err := buildUser(request.UID, request.CommonName, request.Surname, request.Emails)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.Create(r.Context(), created, request.Password); err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, toUserResponse(created))
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	var request updateUserRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	updated, err := buildUser(r.PathValue("uid"), request.CommonName, request.Surname, request.Emails)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.Update(r.Context(), updated); err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toUserResponse(updated))
}

func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	var request passwordRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.SetPassword(r.Context(), uid, request.Password); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	actor, found := sessionctx.From(r.Context())
	if !found {
		h.fail(w, session.ErrNotFound)
		return
	}
	if err := h.service.Delete(r.Context(), actor.UID, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	httpjson.WriteError(w, h.logger, err)
}

func buildUser(rawUID, commonName, surname string, emails []string) (user.User, error) {
	uid, err := user.ParseUID(rawUID)
	if err != nil {
		return user.User{}, err
	}
	return user.New(uid, commonName, surname, emails)
}

func toUserResponse(account user.User) userResponse {
	emails := account.Emails
	if emails == nil {
		emails = []string{}
	}
	return userResponse{UID: string(account.UID), CommonName: account.CommonName, Surname: account.Surname, Emails: emails}
}
```

- [ ] **Step 4: Запустить тесты**

Run: `cd backend && go test ./internal/api/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/api/users
git commit -m "Add user HTTP handlers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: HTTP — группы и дерево

**Files:**
- Create: `backend/internal/api/groups/handler.go`
- Create: `backend/internal/api/directory/handler.go`
- Test: `backend/internal/api/groups/handler_test.go`
- Test: `backend/internal/api/directory/handler_test.go`

**Interfaces:**
- Consumes: `httpjson` (задача 11); сигнатуры `group_service.Service`, `directory_service.Service` (задача 10).
- Produces (пакет `ldap-admin/internal/api/groups`):
  - `type Service interface { List(ctx) ([]group.Group, error); Get(ctx, name group.Name) (group.Group, error); Create(ctx, group.Group) (group.Group, error); Delete(ctx, name group.Name) error; AddMember(ctx, name group.Name, uid user.UID) error; RemoveMember(ctx, name group.Name, uid user.UID) error }`
  - `func New(service Service, logger *slog.Logger) *Handler`; `List`, `Get`, `Create`, `Delete`, `AddMember`, `RemoveMember`; параметры пути `{cn}` и `{uid}`
  - JSON: сводка `{cn, description, memberCount}`, группа `{cn, description, members: [{dn, uid}]}`
- Produces (пакет `ldap-admin/internal/api/directory`):
  - `type Service interface { Children(ctx, dn *directory.DN) ([]directory.Node, error); Entry(ctx, dn *directory.DN) (directory.Entry, error) }`
  - `func New(service Service, logger *slog.Logger) *Handler`; `Children`, `Entry`; query-параметр `dn` (его отсутствие = base DN)
  - JSON: узел `{dn, rdn, objectClass[], hasChildren}`, запись `{dn, attributes, operationalAttributes}`

- [ ] **Step 1: Написать падающие тесты**

`backend/internal/api/groups/handler_test.go`:
```go
package groups_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ldap-admin/internal/api/groups"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeService struct {
	created   []group.Group
	removeErr error
	added     []user.UID
}

func (f *fakeService) List(_ context.Context) ([]group.Group, error) {
	return []group.Group{{Name: "admins", Members: []group.Member{{UID: "alice"}}}}, nil
}

func (f *fakeService) Get(_ context.Context, name group.Name) (group.Group, error) {
	return group.Group{Name: name, Members: []group.Member{{DN: "uid=alice,ou=people,dc=example,dc=com", UID: "alice"}}}, nil
}

func (f *fakeService) Create(_ context.Context, created group.Group) (group.Group, error) {
	f.created = append(f.created, created)
	return created, nil
}

func (f *fakeService) Delete(_ context.Context, name group.Name) error {
	if name == "admins" {
		return group.ErrProtected
	}
	return nil
}

func (f *fakeService) AddMember(_ context.Context, _ group.Name, uid user.UID) error {
	f.added = append(f.added, uid)
	return nil
}

func (f *fakeService) RemoveMember(_ context.Context, _ group.Name, _ user.UID) error {
	return f.removeErr
}

func serve(handler http.HandlerFunc, method, name, uid, payload string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, "/api/groups", strings.NewReader(payload))
	request.SetPathValue("cn", name)
	request.SetPathValue("uid", uid)
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

func TestListReturnsMemberCount(t *testing.T) {
	handler := groups.New(&fakeService{}, silentLogger)

	recorder := serve(handler.List, http.MethodGet, "", "", "")

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"memberCount":1`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}

func TestGetReturnsMembersWithDN(t *testing.T) {
	handler := groups.New(&fakeService{}, silentLogger)

	recorder := serve(handler.Get, http.MethodGet, "admins", "", "")

	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"dn":"uid=alice,ou=people,dc=example,dc=com"`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body)
	}
}

func TestCreate(t *testing.T) {
	service := &fakeService{}
	handler := groups.New(service, silentLogger)

	recorder := serve(handler.Create, http.MethodPost, "", "", `{"cn":"team","description":"Team","members":["alice","bob"]}`)

	if recorder.Code != http.StatusCreated || len(service.created) != 1 || len(service.created[0].Members) != 2 {
		t.Fatalf("status = %d, created = %+v", recorder.Code, service.created)
	}
}

func TestCreateRejectsInvalidGroups(t *testing.T) {
	payloads := []string{
		`{"cn":"team","members":[]}`,
		`{"cn":"team","members":["alice","alice"]}`,
		`{"cn":"Team","members":["alice"]}`,
		`{"cn":"team","members":["Alice"]}`,
	}
	for _, payload := range payloads {
		service := &fakeService{}
		recorder := serve(groups.New(service, silentLogger).Create, http.MethodPost, "", "", payload)
		if recorder.Code != http.StatusBadRequest || len(service.created) != 0 {
			t.Errorf("payload %s: status = %d, created = %v", payload, recorder.Code, service.created)
		}
	}
}

func TestDeleteProtectedGroup(t *testing.T) {
	handler := groups.New(&fakeService{}, silentLogger)

	if recorder := serve(handler.Delete, http.MethodDelete, "admins", "", ""); recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", recorder.Code)
	}
	if recorder := serve(handler.Delete, http.MethodDelete, "team", "", ""); recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
}

func TestMembers(t *testing.T) {
	service := &fakeService{removeErr: group.ErrLastMember}
	handler := groups.New(service, silentLogger)

	added := serve(handler.AddMember, http.MethodPost, "team", "", `{"uid":"bob"}`)
	removed := serve(handler.RemoveMember, http.MethodDelete, "team", "alice", "")

	if added.Code != http.StatusNoContent || len(service.added) != 1 || service.added[0] != "bob" {
		t.Errorf("add: status = %d, added = %v", added.Code, service.added)
	}
	if removed.Code != http.StatusConflict || !strings.Contains(removed.Body.String(), "last_member") {
		t.Errorf("remove: status = %d, body = %s", removed.Code, removed.Body)
	}
}
```

`backend/internal/api/directory/handler_test.go`:
```go
package directory_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apidirectory "ldap-admin/internal/api/directory"
	"ldap-admin/internal/domain/directory"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type fakeService struct {
	requested []*directory.DN
}

func (f *fakeService) Children(_ context.Context, dn *directory.DN) ([]directory.Node, error) {
	f.requested = append(f.requested, dn)
	if dn != nil && *dn == "cn=config" {
		return nil, directory.ErrOutsideBase
	}
	return []directory.Node{{DN: "ou=people,dc=example,dc=com", RDN: "ou=people", HasChildren: true}}, nil
}

func (f *fakeService) Entry(_ context.Context, dn *directory.DN) (directory.Entry, error) {
	f.requested = append(f.requested, dn)
	return directory.Entry{
		DN:                    "dc=example,dc=com",
		Attributes:            map[string][]string{"o": {"Example"}},
		OperationalAttributes: map[string][]string{"entryUUID": {"1b6d"}},
	}, nil
}

func get(handler http.HandlerFunc, target string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func TestChildrenWithoutDNAsksForBase(t *testing.T) {
	service := &fakeService{}
	handler := apidirectory.New(service, silentLogger)

	recorder := get(handler.Children, "/api/directory/children")

	if recorder.Code != http.StatusOK || service.requested[0] != nil {
		t.Fatalf("status = %d, requested = %v", recorder.Code, service.requested)
	}
	if !strings.Contains(recorder.Body.String(), `"objectClass":[]`) || !strings.Contains(recorder.Body.String(), `"hasChildren":true`) {
		t.Fatalf("body = %s", recorder.Body)
	}
}

func TestChildrenPassesEscapedDN(t *testing.T) {
	service := &fakeService{}
	handler := apidirectory.New(service, silentLogger)

	get(handler.Children, "/api/directory/children?dn=ou%3Dpeople%2Cdc%3Dexample%2Cdc%3Dcom")

	if service.requested[0] == nil || *service.requested[0] != "ou=people,dc=example,dc=com" {
		t.Fatalf("requested = %v", service.requested)
	}
}

func TestChildrenOutsideBaseIsBadRequest(t *testing.T) {
	handler := apidirectory.New(&fakeService{}, silentLogger)

	if recorder := get(handler.Children, "/api/directory/children?dn=cn%3Dconfig"); recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", recorder.Code)
	}
}

func TestEntrySplitsAttributes(t *testing.T) {
	handler := apidirectory.New(&fakeService{}, silentLogger)

	recorder := get(handler.Entry, "/api/directory/entry")

	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.Contains(body, `"attributes":{"o":["Example"]}`) ||
		!strings.Contains(body, `"operationalAttributes":{"entryUUID":["1b6d"]}`) {
		t.Fatalf("status = %d, body = %s", recorder.Code, body)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/api/...`
Expected: FAIL — пакетов `groups` и `directory` нет.

- [ ] **Step 3: Реализовать хендлер групп**

`backend/internal/api/groups/handler.go`:
```go
package groups

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type Service interface {
	List(ctx context.Context) ([]group.Group, error)
	Get(ctx context.Context, name group.Name) (group.Group, error)
	Create(ctx context.Context, created group.Group) (group.Group, error)
	Delete(ctx context.Context, name group.Name) error
	AddMember(ctx context.Context, name group.Name, uid user.UID) error
	RemoveMember(ctx context.Context, name group.Name, uid user.UID) error
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type groupSummaryResponse struct {
	Name        string `json:"cn"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}

type memberResponse struct {
	DN  string `json:"dn"`
	UID string `json:"uid"`
}

type groupResponse struct {
	Name        string           `json:"cn"`
	Description string           `json:"description"`
	Members     []memberResponse `json:"members"`
}

type createGroupRequest struct {
	Name        string   `json:"cn"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

type memberRequest struct {
	UID string `json:"uid"`
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	found, err := h.service.List(r.Context())
	if err != nil {
		h.fail(w, err)
		return
	}
	response := make([]groupSummaryResponse, 0, len(found))
	for _, stored := range found {
		response = append(response, groupSummaryResponse{
			Name: string(stored.Name), Description: stored.Description, MemberCount: len(stored.Members),
		})
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	found, err := h.service.Get(r.Context(), name)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusOK, toGroupResponse(found))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var request createGroupRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	draft, err := buildGroup(request)
	if err != nil {
		h.fail(w, err)
		return
	}
	created, err := h.service.Create(r.Context(), draft)
	if err != nil {
		h.fail(w, err)
		return
	}
	httpjson.Write(w, http.StatusCreated, toGroupResponse(created))
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.Delete(r.Context(), name); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	var request memberRequest
	if err := httpjson.Decode(w, r, &request); err != nil {
		h.fail(w, err)
		return
	}
	uid, err := user.ParseUID(request.UID)
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.AddMember(r.Context(), name, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	name, err := group.ParseName(r.PathValue("cn"))
	if err != nil {
		h.fail(w, err)
		return
	}
	uid, err := user.ParseUID(r.PathValue("uid"))
	if err != nil {
		h.fail(w, err)
		return
	}
	if err := h.service.RemoveMember(r.Context(), name, uid); err != nil {
		h.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) fail(w http.ResponseWriter, err error) {
	httpjson.WriteError(w, h.logger, err)
}

func buildGroup(request createGroupRequest) (group.Group, error) {
	name, err := group.ParseName(request.Name)
	if err != nil {
		return group.Group{}, err
	}
	members := make([]group.Member, 0, len(request.Members))
	for _, rawUID := range request.Members {
		uid, err := user.ParseUID(rawUID)
		if err != nil {
			return group.Group{}, err
		}
		members = append(members, group.Member{UID: uid})
	}
	return group.New(name, request.Description, members)
}

func toGroupResponse(stored group.Group) groupResponse {
	members := make([]memberResponse, 0, len(stored.Members))
	for _, member := range stored.Members {
		members = append(members, memberResponse{DN: string(member.DN), UID: string(member.UID)})
	}
	return groupResponse{Name: string(stored.Name), Description: stored.Description, Members: members}
}
```

`user.ParseUID` возвращает `user.ErrInvalid`, а он в таблице `httpjson` уже даёт 400 — отдельной обработки не нужно.

- [ ] **Step 4: Реализовать хендлер дерева**

`backend/internal/api/directory/handler.go`:
```go
// Package directory — HTTP-просмотр дерева каталога. DN передаётся query-параметром:
// в нём есть запятые и знаки «=», в пути URL он был бы неудобен.
package directory

import (
	"context"
	"log/slog"
	"net/http"

	"ldap-admin/internal/api/httpjson"
	"ldap-admin/internal/domain/directory"
)

const dnParameter = "dn"

type Service interface {
	Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error)
	Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error)
}

type Handler struct {
	service Service
	logger  *slog.Logger
}

func New(service Service, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

type nodeResponse struct {
	DN            string   `json:"dn"`
	RDN           string   `json:"rdn"`
	ObjectClasses []string `json:"objectClass"`
	HasChildren   bool     `json:"hasChildren"`
}

type entryResponse struct {
	DN                    string              `json:"dn"`
	Attributes            map[string][]string `json:"attributes"`
	OperationalAttributes map[string][]string `json:"operationalAttributes"`
}

func (h *Handler) Children(w http.ResponseWriter, r *http.Request) {
	nodes, err := h.service.Children(r.Context(), requestedDN(r))
	if err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	response := make([]nodeResponse, 0, len(nodes))
	for _, node := range nodes {
		response = append(response, nodeResponse{
			DN:            string(node.DN),
			RDN:           node.RDN,
			ObjectClasses: append([]string{}, node.ObjectClasses...),
			HasChildren:   node.HasChildren,
		})
	}
	httpjson.Write(w, http.StatusOK, response)
}

func (h *Handler) Entry(w http.ResponseWriter, r *http.Request) {
	entry, err := h.service.Entry(r.Context(), requestedDN(r))
	if err != nil {
		httpjson.WriteError(w, h.logger, err)
		return
	}
	httpjson.Write(w, http.StatusOK, entryResponse{
		DN:                    string(entry.DN),
		Attributes:            entry.Attributes,
		OperationalAttributes: entry.OperationalAttributes,
	})
}

// requestedDN различает «параметра нет» (nil — корень каталога) и «параметр пустой»:
// пустой DN — это ввод клиента, и его проверит репозиторий.
func requestedDN(r *http.Request) *directory.DN {
	query := r.URL.Query()
	if !query.Has(dnParameter) {
		return nil
	}
	dn := directory.DN(query.Get(dnParameter))
	return &dn
}
```

- [ ] **Step 5: Запустить тесты**

Run: `cd backend && go test ./internal/api/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/api/groups backend/internal/api/directory
git commit -m "Add group and directory HTTP handlers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 14: Сборка бэкенда — конфиг, маршруты, composition root

**Files:**
- Create: `backend/internal/config/config.go`
- Create: `backend/internal/server/routes.go`
- Create: `backend/internal/server/server.go`
- Create: `backend/internal/app/app.go`
- Create: `backend/internal/app/runtime.go`
- Create: `backend/cmd/ldap-admin/main.go`
- Create: `backend/.env.example`
- Test: `backend/internal/config/config_test.go`
- Test: `backend/internal/server/routes_test.go`
- Test: `backend/internal/app/runtime_test.go`

**Interfaces:**
- Consumes: всё из задач 2–13.
- Produces:
  - `config.Load(lookup func(string) (string, bool)) (config.Config, error)`; `type Config struct { HTTPAddress, LDAPURL, BindDN, BindPassword, BaseDN, AdminsGroup string; SessionTTL time.Duration }`
  - `server.Handlers{ Auth *auth.Handler; Users *users.Handler; Groups *groups.Handler; Directory *directory.Handler }`; `server.NewHandler(Handlers, *slog.Logger) http.Handler`; `server.Run(ctx, address string, handler http.Handler, logger *slog.Logger) error`
  - `app.Main()`; бинарь `go run ./cmd/ldap-admin` слушает `LDAP_ADMIN_HTTP_ADDRESS`
  - Полная карта маршрутов (раздел 5 спецификации) — её использует фронтенд в задачах 15–18.

- [ ] **Step 1: Написать падающие тесты конфига и генератора ID**

`backend/internal/config/config_test.go`:
```go
package config_test

import (
	"testing"
	"time"

	"ldap-admin/internal/config"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	loaded, err := config.Load(lookupFrom(map[string]string{"LDAP_ADMIN_LDAP_BIND_PASSWORD": "admin"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := config.Config{
		HTTPAddress:  ":8080",
		LDAPURL:      "ldap://localhost:389",
		BindDN:       "cn=admin,dc=example,dc=com",
		BindPassword: "admin",
		BaseDN:       "dc=example,dc=com",
		AdminsGroup:  "admins",
		SessionTTL:   8 * time.Hour,
	}
	if loaded != want {
		t.Fatalf("Load() = %+v, want %+v", loaded, want)
	}
}

func TestLoadRequiresBindPassword(t *testing.T) {
	if _, err := config.Load(lookupFrom(map[string]string{})); err == nil {
		t.Fatalf("Load() without bind password succeeded")
	}
}

func TestLoadRejectsBadSessionTTL(t *testing.T) {
	for _, ttl := range []string{"soon", "0s", "-1h"} {
		_, err := config.Load(lookupFrom(map[string]string{
			"LDAP_ADMIN_LDAP_BIND_PASSWORD": "admin",
			"LDAP_ADMIN_SESSION_TTL":        ttl,
		}))
		if err == nil {
			t.Errorf("Load() accepted session TTL %q", ttl)
		}
	}
}
```

`backend/internal/app/runtime_test.go`:
```go
package app

import "testing"

func TestRandomIDGeneratorProducesDistinctURLSafeIDs(t *testing.T) {
	generator := randomIDGenerator{}

	first, firstErr := generator.NewID()
	second, secondErr := generator.NewID()

	if firstErr != nil || secondErr != nil {
		t.Fatalf("NewID() errors = %v, %v", firstErr, secondErr)
	}
	// 32 байта в base64url без выравнивания — 43 символа.
	if len(first) != 43 || first == second {
		t.Fatalf("NewID() = %q, %q", first, second)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd backend && go test ./internal/config/ ./internal/app/`
Expected: FAIL — пакетов нет.

- [ ] **Step 3: Реализовать конфиг**

`backend/internal/config/config.go`:
```go
// Package config читает настройки из переменных окружения LDAP_ADMIN_*.
// Конфиг грузится один раз в composition root и дальше передаётся параметрами.
package config

import (
	"errors"
	"fmt"
	"time"
)

const envPrefix = "LDAP_ADMIN_"

type Config struct {
	HTTPAddress  string
	LDAPURL      string
	BindDN       string
	BindPassword string
	BaseDN       string
	AdminsGroup  string
	SessionTTL   time.Duration
}

func Load(lookup func(string) (string, bool)) (Config, error) {
	bindPassword, _ := lookup(envPrefix + "LDAP_BIND_PASSWORD")
	if bindPassword == "" {
		return Config{}, errors.New(envPrefix + "LDAP_BIND_PASSWORD is required")
	}
	rawTTL := valueOr(lookup, "SESSION_TTL", "8h")
	sessionTTL, err := time.ParseDuration(rawTTL)
	if err != nil || sessionTTL <= 0 {
		return Config{}, fmt.Errorf("%sSESSION_TTL must be a positive duration, got %q", envPrefix, rawTTL)
	}
	return Config{
		HTTPAddress:  valueOr(lookup, "HTTP_ADDRESS", ":8080"),
		LDAPURL:      valueOr(lookup, "LDAP_URL", "ldap://localhost:389"),
		BindDN:       valueOr(lookup, "LDAP_BIND_DN", "cn=admin,dc=example,dc=com"),
		BindPassword: bindPassword,
		BaseDN:       valueOr(lookup, "LDAP_BASE_DN", "dc=example,dc=com"),
		AdminsGroup:  valueOr(lookup, "ADMINS_GROUP", "admins"),
		SessionTTL:   sessionTTL,
	}, nil
}

func valueOr(lookup func(string) (string, bool), name, fallback string) string {
	if value, present := lookup(envPrefix + name); present && value != "" {
		return value
	}
	return fallback
}
```

- [ ] **Step 4: Реализовать `runtime.go`**

`backend/internal/app/runtime.go`:
```go
package app

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

const sessionIDBytes = 32

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now().UTC()
}

type randomIDGenerator struct{}

func (randomIDGenerator) NewID() (string, error) {
	buffer := make([]byte, sessionIDBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
```

- [ ] **Step 5: Запустить тесты конфига и runtime**

Run: `cd backend && go test ./internal/config/ ./internal/app/`
Expected: PASS.

- [ ] **Step 6: Написать падающий тест маршрутов**

Тест собирает настоящие хендлеры поверх фейкового сервиса входа и проверяет, что защита навешена на все маршруты, кроме входа.

`backend/internal/server/routes_test.go`:
```go
package server_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ldap-admin/internal/api/auth"
	"ldap-admin/internal/api/directory"
	"ldap-admin/internal/api/groups"
	"ldap-admin/internal/api/users"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/server"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type rejectingAuth struct{}

func (rejectingAuth) Login(context.Context, string, string) (session.Session, error) {
	return session.Session{}, session.ErrInvalidCredentials
}

func (rejectingAuth) Authenticate(context.Context, string) (session.Session, error) {
	return session.Session{}, session.ErrNotFound
}

func (rejectingAuth) Logout(context.Context, string) error { return nil }

func TestEveryRouteExceptLoginRequiresSession(t *testing.T) {
	handler := server.NewHandler(server.Handlers{
		Auth:      auth.New(rejectingAuth{}, silentLogger),
		Users:     users.New(nil, silentLogger),
		Groups:    groups.New(nil, silentLogger),
		Directory: directory.New(nil, silentLogger),
	}, silentLogger)
	protected := []string{
		"POST /api/auth/logout", "GET /api/auth/me",
		"GET /api/users", "GET /api/users/alice", "POST /api/users", "PUT /api/users/alice",
		"PUT /api/users/alice/password", "DELETE /api/users/alice",
		"GET /api/groups", "GET /api/groups/admins", "POST /api/groups", "DELETE /api/groups/team",
		"POST /api/groups/team/members", "DELETE /api/groups/team/members/alice",
		"GET /api/directory/children", "GET /api/directory/entry",
	}
	for _, route := range protected {
		method, path, _ := strings.Cut(route, " ")
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", route, recorder.Code)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"uid":"alice","password":"wrong-pass"}`)))
	if recorder.Code != http.StatusUnauthorized || !strings.Contains(recorder.Body.String(), "invalid_credentials") {
		t.Errorf("login: status = %d, body = %s", recorder.Code, recorder.Body)
	}
}
```

Сервисы пользователей, групп и дерева здесь `nil`: до них запрос не доходит, middleware отвечает раньше. Если тест упадёт с nil pointer — значит, маршрут оказался без защиты.

- [ ] **Step 7: Запустить и убедиться, что падает**

Run: `cd backend && go test ./internal/server/`
Expected: FAIL — пакета `server` нет.

- [ ] **Step 8: Реализовать маршруты и сервер**

`backend/internal/server/routes.go`:
```go
package server

import (
	"log/slog"
	"net/http"
	"time"

	"ldap-admin/internal/api/auth"
	"ldap-admin/internal/api/directory"
	"ldap-admin/internal/api/groups"
	"ldap-admin/internal/api/users"
)

type Handlers struct {
	Auth      *auth.Handler
	Users     *users.Handler
	Groups    *groups.Handler
	Directory *directory.Handler
}

type route struct {
	pattern string
	handler http.HandlerFunc
}

func NewHandler(handlers Handlers, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/auth/login", handlers.Auth.Login)
	for _, protected := range protectedRoutes(handlers) {
		mux.Handle(protected.pattern, handlers.Auth.RequireSession(protected.handler))
	}
	return logRequests(logger, mux)
}

func protectedRoutes(handlers Handlers) []route {
	return []route{
		{"POST /api/auth/logout", handlers.Auth.Logout},
		{"GET /api/auth/me", handlers.Auth.Me},
		{"GET /api/users", handlers.Users.List},
		{"POST /api/users", handlers.Users.Create},
		{"GET /api/users/{uid}", handlers.Users.Get},
		{"PUT /api/users/{uid}", handlers.Users.Update},
		{"PUT /api/users/{uid}/password", handlers.Users.SetPassword},
		{"DELETE /api/users/{uid}", handlers.Users.Delete},
		{"GET /api/groups", handlers.Groups.List},
		{"POST /api/groups", handlers.Groups.Create},
		{"GET /api/groups/{cn}", handlers.Groups.Get},
		{"DELETE /api/groups/{cn}", handlers.Groups.Delete},
		{"POST /api/groups/{cn}/members", handlers.Groups.AddMember},
		{"DELETE /api/groups/{cn}/members/{uid}", handlers.Groups.RemoveMember},
		{"GET /api/directory/children", handlers.Directory.Children},
		{"GET /api/directory/entry", handlers.Directory.Entry},
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		logger.Info("request",
			"method", r.Method, "path", r.URL.Path, "status", recorder.status, "duration", time.Since(started))
	})
}
```

`backend/internal/server/server.go`:
```go
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 30 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Run обслуживает HTTP до отмены ctx, затем даёт текущим запросам завершиться.
func Run(ctx context.Context, address string, handler http.Handler, logger *slog.Logger) error {
	httpServer := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
	}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- httpServer.ListenAndServe() }()
	logger.Info("listening", "address", address)

	select {
	case err := <-serveErrors:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownContext, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownContext); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
```

- [ ] **Step 9: Запустить тест маршрутов**

Run: `cd backend && go test ./internal/server/`
Expected: PASS.

- [ ] **Step 10: Реализовать composition root и точку входа**

`backend/internal/app/app.go`:
```go
// Package app — composition root: читает конфиг и руками собирает слои
// транспорт → репозитории → сервисы → хендлеры.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"ldap-admin/internal/api/auth"
	"ldap-admin/internal/api/directory"
	"ldap-admin/internal/api/groups"
	"ldap-admin/internal/api/users"
	"ldap-admin/internal/config"
	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/repos/directory_repo"
	"ldap-admin/internal/repos/group_repo"
	"ldap-admin/internal/repos/session_repo"
	"ldap-admin/internal/repos/tree_layout"
	"ldap-admin/internal/repos/user_repo"
	"ldap-admin/internal/server"
	"ldap-admin/internal/services/auth_service"
	"ldap-admin/internal/services/directory_service"
	"ldap-admin/internal/services/group_service"
	"ldap-admin/internal/services/user_service"
)

func Main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("ldap-admin stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	loaded, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	adminsGroup, err := group.ParseName(loaded.AdminsGroup)
	if err != nil {
		return fmt.Errorf("LDAP_ADMIN_ADMINS_GROUP: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client := ldap_db.New(ldap_db.Config{URL: loaded.LDAPURL, BindDN: loaded.BindDN, BindPassword: loaded.BindPassword})
	layout := tree_layout.New(loaded.BaseDN)
	userRepo := user_repo.New(client, layout)
	groupRepo := group_repo.New(client, layout)
	directoryRepo := directory_repo.New(client, loaded.BaseDN)
	sessionRepo := session_repo.New()

	authService := auth_service.New(
		userRepo, groupRepo, userRepo, sessionRepo,
		systemClock{}, randomIDGenerator{}, adminsGroup, loaded.SessionTTL,
	)
	userService := user_service.New(userRepo, groupRepo)
	groupService := group_service.New(groupRepo, userRepo, adminsGroup)
	directoryService := directory_service.New(directoryRepo)

	handler := server.NewHandler(server.Handlers{
		Auth:      auth.New(authService, logger),
		Users:     users.New(userService, logger),
		Groups:    groups.New(groupService, logger),
		Directory: directory.New(directoryService, logger),
	}, logger)
	return server.Run(ctx, loaded.HTTPAddress, handler, logger)
}
```

`backend/cmd/ldap-admin/main.go`:
```go
package main

import "ldap-admin/internal/app"

func main() {
	app.Main()
}
```

`backend/.env.example`:
```dotenv
LDAP_ADMIN_HTTP_ADDRESS=:8080
LDAP_ADMIN_LDAP_URL=ldap://localhost:389
LDAP_ADMIN_LDAP_BIND_DN=cn=admin,dc=example,dc=com
LDAP_ADMIN_LDAP_BIND_PASSWORD=admin
LDAP_ADMIN_LDAP_BASE_DN=dc=example,dc=com
LDAP_ADMIN_ADMINS_GROUP=admins
LDAP_ADMIN_SESSION_TTL=8h
```

- [ ] **Step 11: Прогнать все проверки бэкенда**

Run: `cd backend && make build && make vet && make test && make test-integration`
Expected: всё без ошибок; интеграционные тесты проходят против контейнера.

- [ ] **Step 12: Смоук-тест живого бэкенда**

Run (одним блоком, из `backend/`):
```bash
cp -n .env.example .env
go build -o /tmp/ldap-admin-smoke ./cmd/ldap-admin
(set -a; . ./.env; set +a; exec /tmp/ldap-admin-smoke) > /tmp/ldap-admin-smoke.log 2>&1 &
SERVER_PID=$!
sleep 2
JAR=$(mktemp)
curl -s -c "$JAR" -X POST localhost:8080/api/auth/login -d '{"uid":"alice","password":"alice-secret"}'; echo
curl -s -b "$JAR" localhost:8080/api/users; echo
curl -s -b "$JAR" localhost:8080/api/groups/admins; echo
curl -s -b "$JAR" 'localhost:8080/api/directory/children'; echo
curl -s -b "$JAR" 'localhost:8080/api/directory/children?dn=cn%3Dconfig'; echo
curl -s localhost:8080/api/users; echo
kill $SERVER_PID
```
Expected, по строкам:
1. `{"uid":"alice","cn":"Alice Admin"}`
2. массив с одним пользователем `alice`
3. группа `admins` с участником `{"dn":"uid=alice,ou=people,dc=example,dc=com","uid":"alice"}`
4. ровно два узла — `ou=groups` и `ou=people`, оба с `"hasChildren":true` (`cn=admin` — служебный rootdn сервера, записи в дереве у него нет)
5. `{"error":"invalid_input","message":"dn is outside the directory base"}`
6. `{"error":"unauthenticated",...}`

Если `.env` уже существовал, `cp -n` его не перезапишет. После `kill` в `/tmp/ldap-admin-smoke.log` должна быть строка `shutting down` — это проверка graceful shutdown.

- [ ] **Step 13: Commit**

```bash
git add backend/internal/config backend/internal/server backend/internal/app backend/cmd backend/.env.example
git commit -m "Wire backend composition root and HTTP routes

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 15: Фронтенд — каркас, слой API, вход

**Files:**
- Create: `frontend/package.json`, `frontend/tsconfig.json`, `frontend/env.d.ts`, `frontend/vite.config.ts`, `frontend/index.html`
- Create: `frontend/src/main.ts`, `frontend/src/App.vue`, `frontend/src/libs/vuetify.ts`
- Create: `frontend/src/common/services/api/{api-error,http-client,models,dtos,api.service}.ts`
- Create: `frontend/src/common/services/api/repositories/auth.repository.ts`
- Create: `frontend/src/stores/notifier.store.ts`
- Create: `frontend/src/components/AppNotifier.vue`, `frontend/src/components/ConfirmDialog.vue`
- Create: `frontend/src/router/{index,guards,types}.ts`
- Create: `frontend/src/layouts/DefaultLayout.vue`, `frontend/src/views/HomeView.vue`
- Create: `frontend/src/features/auth/{index.ts,stores/auth.store.ts,helpers/login-error.ts,helpers/redirect-target.ts,views/LoginView.vue}`
- Test: `frontend/src/common/services/api/__tests__/api-error.spec.ts`
- Test: `frontend/src/common/services/api/__tests__/auth.repository.spec.ts`
- Test: `frontend/src/features/auth/__tests__/auth.store.spec.ts`
- Test: `frontend/src/features/auth/__tests__/helpers.spec.ts`
- Test: `frontend/src/router/__tests__/guards.spec.ts`

**Interfaces:**
- Consumes: HTTP API бэкенда (задача 14), запущенный на `localhost:8080`.
- Produces:
  - `@/common/services/api/models` — все модели фронтенда (ниже), их используют задачи 16–18.
  - `@/common/services/api/dtos` — все формы ответов бэкенда.
  - `ApiError` (`status`, `code`, `message`, `groups`), `toApiError(error: unknown): ApiError`, `errorMessage(error: unknown): string`, `isApiError`.
  - `createHttpClient(): AxiosInstance`, `setUnauthorizedHandler(handler: () => void)`, `rejectWithApiError(error: unknown): Promise<never>`.
  - `apiService` — объект с репозиториями; в этой задаче только `auth`. Задачи 16–18 добавляют `directory`, `users`, `groups`.
  - `useNotifierStore()` с `success(message: string)` и `error(caught: unknown)`.
  - `ConfirmDialog` — `v-model` (boolean), props `title`, `text`, `confirmLabel?`, `loading?`, событие `confirm`.
  - `useAuthStore()` (из `@/features/auth`): `user`, `checked`, `isSignedIn`, `fetchCurrentUser()`, `signIn(uid, password)`, `signOut()`, `reset()`.
  - Маршруты `login` (публичный) и `home`; массив `NAVIGATION` в `DefaultLayout.vue`, который расширяют задачи 16–18.

- [ ] **Step 1: Создать `package.json` и установить зависимости**

`frontend/package.json`:
```json
{
  "name": "ldap-admin-frontend",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "vue-tsc --noEmit && vite build",
    "typecheck": "vue-tsc --noEmit",
    "test": "vitest",
    "test:run": "vitest run"
  }
}
```

Версии — те же мажоры, что работают вместе в matrix-customer-frontend.

Run:
```bash
cd frontend
npm install vue@^3.5 vue-router@^5.1 pinia@^3.0 vuetify@^4.1 axios@^1.18 @mdi/font@^7.4
npm install -D vite@^8.1 @vitejs/plugin-vue@^6.0 vite-plugin-vuetify@^2.1 typescript@~6.0 vue-tsc@^3.3 @vue/tsconfig@^0.9 vitest@^4.1 happy-dom@^20
```
Expected: `node_modules/` и `package-lock.json` созданы, без ошибок разрешения зависимостей.

- [ ] **Step 2: Конфигурация сборки**

`frontend/tsconfig.json`:
```json
{
  "extends": "@vue/tsconfig/tsconfig.dom.json",
  "compilerOptions": {
    "noUncheckedIndexedAccess": true,
    "paths": {
      "@/*": ["./src/*"]
    }
  },
  "include": ["env.d.ts", "src/**/*.ts", "src/**/*.vue"]
}
```

`frontend/env.d.ts`:
```ts
/// <reference types="vite/client" />
```

`frontend/vite.config.ts`:
```ts
import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import vuetify from 'vite-plugin-vuetify';
import { defineConfig } from 'vitest/config';

export default defineConfig({
  plugins: [vue(), vuetify({ autoImport: true })],
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  server: {
    port: 5173,
    strictPort: true,
    // Тот же origin для страницы и API: cookie сессии работает без CORS.
    proxy: { '/api': 'http://localhost:8080' },
  },
  test: {
    environment: 'happy-dom',
    // Vuetify поставляет компоненты вместе с .css — без inline Node не сможет их импортировать.
    server: { deps: { inline: ['vuetify'] } },
  },
});
```

`frontend/index.html`:
```html
<!doctype html>
<html lang="ru">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>LDAP Admin</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.ts"></script>
  </body>
</html>
```

- [ ] **Step 3: Модели и DTO**

`frontend/src/common/services/api/dtos.ts`:
```ts
// Формы ответов бэкенда как есть: имена полей совпадают с атрибутами LDAP.

export interface CurrentUserDto {
  uid: string;
  cn: string;
}

export interface UserDto {
  uid: string;
  cn: string;
  sn: string;
  mail: string[];
}

export interface UserDetailsDto extends UserDto {
  groups: string[];
}

export interface GroupSummaryDto {
  cn: string;
  description: string;
  memberCount: number;
}

export interface GroupMemberDto {
  dn: string;
  uid: string;
}

export interface GroupDto {
  cn: string;
  description: string;
  members: GroupMemberDto[];
}

export interface DirectoryNodeDto {
  dn: string;
  rdn: string;
  objectClass: string[];
  hasChildren: boolean;
}

export interface DirectoryEntryDto {
  dn: string;
  attributes: Record<string, string[]>;
  operationalAttributes: Record<string, string[]>;
}
```

`frontend/src/common/services/api/models.ts`:
```ts
export interface CurrentUser {
  uid: string;
  commonName: string;
}

export interface User {
  uid: string;
  commonName: string;
  surname: string;
  emails: string[];
}

export interface UserDetails extends User {
  groups: string[];
}

export interface UserChanges {
  commonName: string;
  surname: string;
  emails: string[];
}

export interface UserDraft extends UserChanges {
  uid: string;
  password: string;
}

export interface GroupSummary {
  name: string;
  description: string;
  memberCount: number;
}

export interface GroupMember {
  dn: string;
  /** null — участник вне ou=people, у него нет uid. */
  uid: string | null;
}

export interface Group {
  name: string;
  description: string;
  members: GroupMember[];
}

export interface GroupDraft {
  name: string;
  description: string;
  memberUids: string[];
}

export interface DirectoryNode {
  dn: string;
  rdn: string;
  objectClasses: string[];
  hasChildren: boolean;
}

export interface DirectoryEntry {
  dn: string;
  attributes: Record<string, string[]>;
  operationalAttributes: Record<string, string[]>;
}
```

- [ ] **Step 4: Написать падающие тесты ошибок API и репозитория входа**

`frontend/src/common/services/api/__tests__/api-error.spec.ts`:
```ts
import { AxiosError, AxiosHeaders } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { ApiError, errorMessage, toApiError } from '../api-error';
import { rejectWithApiError, setUnauthorizedHandler } from '../http-client';

function responseError(status: number, data: unknown): AxiosError {
  const config = { headers: new AxiosHeaders() };
  return new AxiosError('Request failed', 'ERR_BAD_REQUEST', config, null, {
    data, status, statusText: '', headers: {}, config,
  });
}

describe('toApiError', () => {
  it('reads code, message and groups from the backend body', () => {
    const error = toApiError(responseError(409, {
      error: 'sole_member', message: 'user is the only member of groups: solo', groups: ['solo'],
    }));

    expect(error).toBeInstanceOf(ApiError);
    expect(error.status).toBe(409);
    expect(error.code).toBe('sole_member');
    expect(error.groups).toEqual(['solo']);
    expect(errorMessage(error)).toBe('user is the only member of groups: solo');
  });

  it('marks a missing response as a network error', () => {
    const error = toApiError(new AxiosError('Network Error', 'ERR_NETWORK'));

    expect(error.code).toBe('network');
    expect(error.status).toBe(0);
  });

  it('survives a body that is not JSON', () => {
    const error = toApiError(responseError(502, '<html>Bad Gateway</html>'));

    expect(error.code).toBe('unknown');
    expect(error.status).toBe(502);
  });
});

describe('rejectWithApiError', () => {
  it('calls the unauthorized handler only for an expired session', async () => {
    const handler = vi.fn();
    setUnauthorizedHandler(handler);

    await expect(rejectWithApiError(responseError(401, { error: 'invalid_credentials', message: 'x' }))).rejects.toBeInstanceOf(ApiError);
    expect(handler).not.toHaveBeenCalled();

    await expect(rejectWithApiError(responseError(401, { error: 'unauthenticated', message: 'x' }))).rejects.toBeInstanceOf(ApiError);
    expect(handler).toHaveBeenCalledOnce();
  });
});
```

`frontend/src/common/services/api/__tests__/auth.repository.spec.ts`:
```ts
import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { AuthRepository } from '../repositories/auth.repository';

describe('AuthRepository', () => {
  it('maps the backend user to the model', async () => {
    const post = vi.fn().mockResolvedValue({ data: { uid: 'alice', cn: 'Alice Admin' } });
    const repository = new AuthRepository({ post } as unknown as AxiosInstance);

    const user = await repository.login('alice', 'alice-secret');

    expect(post).toHaveBeenCalledWith('/auth/login', { uid: 'alice', password: 'alice-secret' });
    expect(user).toEqual({ uid: 'alice', commonName: 'Alice Admin' });
  });
});
```

- [ ] **Step 5: Запустить и убедиться, что падают**

Run: `cd frontend && npm run test:run`
Expected: FAIL — модулей `api-error`, `http-client`, `auth.repository` нет.

- [ ] **Step 6: Реализовать ошибки, HTTP-клиент и репозиторий входа**

`frontend/src/common/services/api/api-error.ts`:
```ts
import { isAxiosError } from 'axios';

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly groups: string[];

  constructor(status: number, code: string, message: string, groups: string[] = []) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.groups = groups;
  }
}

interface ErrorBody {
  error?: unknown;
  message?: unknown;
  groups?: unknown;
}

export function isApiError(value: unknown): value is ApiError {
  return value instanceof ApiError;
}

export function toApiError(error: unknown): ApiError {
  if (isApiError(error)) {
    return error;
  }
  if (!isAxiosError(error)) {
    return new ApiError(0, 'unknown', error instanceof Error ? error.message : String(error));
  }
  if (!error.response) {
    return new ApiError(0, 'network', 'Бэкенд недоступен');
  }
  const body: ErrorBody = isRecord(error.response.data) ? error.response.data : {};
  return new ApiError(
    error.response.status,
    typeof body.error === 'string' ? body.error : 'unknown',
    typeof body.message === 'string' ? body.message : error.message,
    Array.isArray(body.groups) ? body.groups.filter((name): name is string => typeof name === 'string') : [],
  );
}

export function errorMessage(error: unknown): string {
  return toApiError(error).message;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null;
}
```

`frontend/src/common/services/api/http-client.ts`:
```ts
import axios, { type AxiosInstance } from 'axios';
import { toApiError } from './api-error';

let unauthorizedHandler: () => void = () => {};

export function setUnauthorizedHandler(handler: () => void): void {
  unauthorizedHandler = handler;
}

// invalid_credentials — ответ на неверный пароль при входе, а не истёкшая сессия,
// поэтому на страницу входа отправляет только unauthenticated.
export function rejectWithApiError(error: unknown): Promise<never> {
  const apiError = toApiError(error);
  if (apiError.code === 'unauthenticated') {
    unauthorizedHandler();
  }
  return Promise.reject(apiError);
}

export function createHttpClient(): AxiosInstance {
  const client = axios.create({ baseURL: '/api', withCredentials: true });
  client.interceptors.response.use((response) => response, rejectWithApiError);
  return client;
}
```

`frontend/src/common/services/api/repositories/auth.repository.ts`:
```ts
import type { AxiosInstance } from 'axios';
import type { CurrentUserDto } from '../dtos';
import type { CurrentUser } from '../models';

export class AuthRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  async login(uid: string, password: string): Promise<CurrentUser> {
    const { data } = await this.http.post<CurrentUserDto>('/auth/login', { uid, password });
    return toCurrentUser(data);
  }

  async logout(): Promise<void> {
    await this.http.post('/auth/logout');
  }

  async me(): Promise<CurrentUser> {
    const { data } = await this.http.get<CurrentUserDto>('/auth/me');
    return toCurrentUser(data);
  }
}

function toCurrentUser(dto: CurrentUserDto): CurrentUser {
  return { uid: dto.uid, commonName: dto.cn };
}
```

`frontend/src/common/services/api/api.service.ts`:
```ts
import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
};
```

- [ ] **Step 7: Запустить тесты**

Run: `cd frontend && npm run test:run`
Expected: PASS.

- [ ] **Step 8: Написать падающие тесты входа и guard'а**

`frontend/src/features/auth/__tests__/helpers.spec.ts`:
```ts
import { describe, expect, it } from 'vitest';
import { ApiError } from '@/common/services/api/api-error';
import { loginErrorMessage } from '../helpers/login-error';
import { safeRedirectTarget } from '../helpers/redirect-target';

describe('loginErrorMessage', () => {
  it('distinguishes a wrong password from a missing admin membership', () => {
    expect(loginErrorMessage(new ApiError(401, 'invalid_credentials', 'x'))).toBe('Неверный uid или пароль');
    expect(loginErrorMessage(new ApiError(403, 'not_admin', 'x'))).toBe('Вход разрешён только участникам группы admins');
  });

  it('falls back to the backend message', () => {
    expect(loginErrorMessage(new ApiError(503, 'directory_unavailable', 'directory unavailable'))).toBe('directory unavailable');
  });
});

describe('safeRedirectTarget', () => {
  it('keeps internal paths only', () => {
    expect(safeRedirectTarget('/users/alice')).toBe('/users/alice');
    expect(safeRedirectTarget('//evil.example')).toBe('/');
    expect(safeRedirectTarget('https://evil.example')).toBe('/');
    expect(safeRedirectTarget(undefined)).toBe('/');
    expect(safeRedirectTarget(['/users'])).toBe('/');
  });
});
```

`frontend/src/features/auth/__tests__/auth.store.spec.ts`:
```ts
import { createPinia, setActivePinia } from 'pinia';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiService } from '@/common/services/api/api.service';
import { ApiError } from '@/common/services/api/api-error';
import { useAuthStore } from '../stores/auth.store';

vi.mock('@/common/services/api/api.service', () => ({
  apiService: { auth: { login: vi.fn(), logout: vi.fn(), me: vi.fn() } },
}));

describe('useAuthStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.mocked(apiService.auth.me).mockReset();
    vi.mocked(apiService.auth.logout).mockReset();
  });

  it('treats an expired session as signed out', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'));
    const store = useAuthStore();

    await store.fetchCurrentUser();

    expect(store.isSignedIn).toBe(false);
    expect(store.checked).toBe(true);
  });

  it('rethrows other failures', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(0, 'network', 'down'));
    const store = useAuthStore();

    await expect(store.fetchCurrentUser()).rejects.toBeInstanceOf(ApiError);
    expect(store.checked).toBe(true);
  });

  it('forgets the user on sign out even if the request fails', async () => {
    vi.mocked(apiService.auth.me).mockResolvedValue({ uid: 'alice', commonName: 'Alice Admin' });
    vi.mocked(apiService.auth.logout).mockRejectedValue(new ApiError(0, 'network', 'down'));
    const store = useAuthStore();
    await store.fetchCurrentUser();

    await expect(store.signOut()).rejects.toBeInstanceOf(ApiError);

    expect(store.isSignedIn).toBe(false);
  });
});
```

`frontend/src/router/__tests__/guards.spec.ts`:
```ts
import { createPinia, setActivePinia } from 'pinia';
import type { RouteLocationNormalized } from 'vue-router';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { apiService } from '@/common/services/api/api.service';
import { ApiError } from '@/common/services/api/api-error';
import { authGuard } from '../guards';

vi.mock('@/common/services/api/api.service', () => ({
  apiService: { auth: { login: vi.fn(), logout: vi.fn(), me: vi.fn() } },
}));

function routeTo(fullPath: string, publicAccess = false): RouteLocationNormalized {
  return { fullPath, meta: { publicAccess } } as unknown as RouteLocationNormalized;
}

describe('authGuard', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.mocked(apiService.auth.me).mockReset();
  });

  it('lets public routes through without asking the backend', async () => {
    expect(await authGuard(routeTo('/login', true))).toBe(true);
    expect(apiService.auth.me).not.toHaveBeenCalled();
  });

  it('sends a signed-out user to login and remembers the target', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(401, 'unauthenticated', 'x'));

    expect(await authGuard(routeTo('/users'))).toEqual({ name: 'login', query: { redirect: '/users' } });
  });

  it('treats an unreachable backend as signed out', async () => {
    vi.mocked(apiService.auth.me).mockRejectedValue(new ApiError(0, 'network', 'down'));

    expect(await authGuard(routeTo('/users'))).toEqual({ name: 'login', query: { redirect: '/users' } });
  });

  it('asks the backend only once per page load', async () => {
    vi.mocked(apiService.auth.me).mockResolvedValue({ uid: 'alice', commonName: 'Alice Admin' });

    expect(await authGuard(routeTo('/'))).toBe(true);
    expect(await authGuard(routeTo('/users'))).toBe(true);
    expect(apiService.auth.me).toHaveBeenCalledOnce();
  });
});
```

- [ ] **Step 9: Запустить и убедиться, что падают**

Run: `cd frontend && npm run test:run`
Expected: FAIL — модулей `auth.store`, `login-error`, `redirect-target`, `guards` нет.

- [ ] **Step 10: Реализовать стор входа и помощники**

`frontend/src/features/auth/stores/auth.store.ts`:
```ts
import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { isApiError } from '@/common/services/api/api-error';
import type { CurrentUser } from '@/common/services/api/models';

export const useAuthStore = defineStore('auth', () => {
  const user = ref<CurrentUser | null>(null);
  // checked — сессию уже спрашивали у бэкенда; без этого guard ходил бы в /auth/me на каждый переход.
  const checked = ref(false);
  const isSignedIn = computed(() => user.value !== null);

  async function fetchCurrentUser(): Promise<void> {
    try {
      user.value = await apiService.auth.me();
    } catch (caught) {
      user.value = null;
      if (!isApiError(caught) || caught.code !== 'unauthenticated') {
        throw caught;
      }
    } finally {
      checked.value = true;
    }
  }

  async function signIn(uid: string, password: string): Promise<void> {
    user.value = await apiService.auth.login(uid, password);
    checked.value = true;
  }

  async function signOut(): Promise<void> {
    try {
      await apiService.auth.logout();
    } finally {
      reset();
    }
  }

  function reset(): void {
    user.value = null;
  }

  return { user, checked, isSignedIn, fetchCurrentUser, signIn, signOut, reset };
});
```

`frontend/src/features/auth/helpers/login-error.ts`:
```ts
import { toApiError } from '@/common/services/api/api-error';

const LOGIN_MESSAGES: Record<string, string> = {
  invalid_credentials: 'Неверный uid или пароль',
  not_admin: 'Вход разрешён только участникам группы admins',
};

export function loginErrorMessage(error: unknown): string {
  const apiError = toApiError(error);
  return LOGIN_MESSAGES[apiError.code] ?? apiError.message;
}
```

`frontend/src/features/auth/helpers/redirect-target.ts`:
```ts
// Только пути внутри приложения: «//host» браузер понял бы как адрес другого сайта.
export function safeRedirectTarget(value: unknown): string {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//')) {
    return '/';
  }
  return value;
}
```

- [ ] **Step 11: Реализовать страницу входа и публичный API модуля**

`frontend/src/features/auth/views/LoginView.vue`:
```vue
<script setup lang="ts">
import { ref } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { loginErrorMessage } from '../helpers/login-error';
import { safeRedirectTarget } from '../helpers/redirect-target';
import { useAuthStore } from '../stores/auth.store';

const auth = useAuthStore();
const route = useRoute();
const router = useRouter();

const uid = ref('');
const password = ref('');
const errorText = ref('');
const submitting = ref(false);

async function submit(): Promise<void> {
  errorText.value = '';
  submitting.value = true;
  try {
    await auth.signIn(uid.value.trim(), password.value);
    await router.replace(safeRedirectTarget(route.query.redirect));
  } catch (caught) {
    errorText.value = loginErrorMessage(caught);
  } finally {
    submitting.value = false;
  }
}
</script>

<template>
  <v-main>
    <v-container class="fill-height d-flex justify-center">
      <v-card width="420" title="LDAP Admin" subtitle="Вход для участников группы admins">
        <v-card-text>
          <v-form @submit.prevent="submit">
            <v-text-field v-model="uid" label="uid" autocomplete="username" autofocus />
            <v-text-field v-model="password" label="Пароль" type="password" autocomplete="current-password" />
            <v-alert v-if="errorText" type="error" variant="tonal" class="mb-4">{{ errorText }}</v-alert>
            <v-btn type="submit" color="primary" block :loading="submitting">Войти</v-btn>
          </v-form>
        </v-card-text>
      </v-card>
    </v-container>
  </v-main>
</template>
```

`frontend/src/features/auth/index.ts`:
```ts
export { useAuthStore } from './stores/auth.store';
export { default as LoginView } from './views/LoginView.vue';
```

- [ ] **Step 12: Реализовать роутер, guard и общие компоненты**

`frontend/src/router/types.ts`:
```ts
import 'vue-router';

declare module 'vue-router' {
  interface RouteMeta {
    publicAccess?: boolean;
  }
}

export {};
```

`frontend/src/router/guards.ts`:
```ts
import type { RouteLocationNormalized, RouteLocationRaw } from 'vue-router';
import { useAuthStore } from '@/features/auth';

export async function authGuard(to: RouteLocationNormalized): Promise<true | RouteLocationRaw> {
  if (to.meta.publicAccess) {
    return true;
  }
  const auth = useAuthStore();
  if (!auth.checked) {
    try {
      await auth.fetchCurrentUser();
    } catch {
      auth.reset();
    }
  }
  if (auth.isSignedIn) {
    return true;
  }
  return { name: 'login', query: { redirect: to.fullPath } };
}
```

`frontend/src/router/index.ts`:
```ts
import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router';
import { LoginView } from '@/features/auth';
import DefaultLayout from '@/layouts/DefaultLayout.vue';
import HomeView from '@/views/HomeView.vue';
import { authGuard } from './guards';
import './types';

const routes: RouteRecordRaw[] = [
  { path: '/login', name: 'login', component: LoginView, meta: { publicAccess: true } },
  {
    path: '/',
    component: DefaultLayout,
    children: [
      { path: '', name: 'home', component: HomeView },
    ],
  },
];

export const router = createRouter({ history: createWebHistory(), routes });

router.beforeEach(authGuard);
```

`frontend/src/stores/notifier.store.ts`:
```ts
import { defineStore } from 'pinia';
import { ref } from 'vue';
import { errorMessage } from '@/common/services/api/api-error';

export type NoticeKind = 'success' | 'error';

export const useNotifierStore = defineStore('notifier', () => {
  const visible = ref(false);
  const text = ref('');
  const kind = ref<NoticeKind>('success');

  function show(message: string, noticeKind: NoticeKind): void {
    text.value = message;
    kind.value = noticeKind;
    visible.value = true;
  }

  function success(message: string): void {
    show(message, 'success');
  }

  function error(caught: unknown): void {
    show(errorMessage(caught), 'error');
  }

  return { visible, text, kind, success, error };
});
```

`frontend/src/components/AppNotifier.vue`:
```vue
<script setup lang="ts">
import { storeToRefs } from 'pinia';
import { useNotifierStore } from '@/stores/notifier.store';

const { visible, text, kind } = storeToRefs(useNotifierStore());
</script>

<template>
  <v-snackbar v-model="visible" :color="kind" :timeout="6000" location="top">{{ text }}</v-snackbar>
</template>
```

`frontend/src/components/ConfirmDialog.vue`:
```vue
<script setup lang="ts">
defineProps<{ title: string; text: string; confirmLabel?: string; loading?: boolean }>();
const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ confirm: [] }>();
</script>

<template>
  <v-dialog v-model="open" max-width="480">
    <v-card :title="title" :text="text">
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">Отмена</v-btn>
        <v-btn color="error" :loading="loading" @click="emit('confirm')">{{ confirmLabel ?? 'Удалить' }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
```

`frontend/src/layouts/DefaultLayout.vue`:
```vue
<script setup lang="ts">
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/features/auth';
import { useNotifierStore } from '@/stores/notifier.store';

interface NavigationItem {
  title: string;
  icon: string;
  routeName: string;
}

const NAVIGATION: NavigationItem[] = [
  { title: 'Главная', icon: 'mdi-home', routeName: 'home' },
];

const auth = useAuthStore();
const notifier = useNotifierStore();
const router = useRouter();

async function signOut(): Promise<void> {
  try {
    await auth.signOut();
  } catch (caught) {
    notifier.error(caught);
  }
  await router.push({ name: 'login' });
}
</script>

<template>
  <v-app-bar density="compact" color="primary">
    <v-app-bar-title>LDAP Admin</v-app-bar-title>
    <v-btn
      v-for="item in NAVIGATION"
      :key="item.routeName"
      :to="{ name: item.routeName }"
      :prepend-icon="item.icon"
      variant="text"
    >
      {{ item.title }}
    </v-btn>
    <v-spacer />
    <span class="mr-2">{{ auth.user?.commonName }}</span>
    <v-btn icon="mdi-logout" title="Выйти" @click="signOut" />
  </v-app-bar>
  <v-main>
    <router-view />
  </v-main>
</template>
```

`frontend/src/views/HomeView.vue`:
```vue
<template>
  <v-container>
    <v-card title="Учебная админ-панель OpenLDAP">
      <v-card-text>
        <p>Разделы в меню сверху:</p>
        <ul class="ml-6 mt-2">
          <li>«Дерево» — каталог как есть: записи, их objectClass и атрибуты, включая служебные.</li>
          <li>«Пользователи» — записи inetOrgPerson в ou=people.</li>
          <li>«Группы» — записи groupOfUniqueNames в ou=groups.</li>
        </ul>
      </v-card-text>
    </v-card>
  </v-container>
</template>
```

`frontend/src/libs/vuetify.ts`:
```ts
import '@mdi/font/css/materialdesignicons.css';
import 'vuetify/styles';
import { createVuetify } from 'vuetify';
import { ru } from 'vuetify/locale';

export const vuetify = createVuetify({
  locale: { locale: 'ru', messages: { ru } },
  theme: { defaultTheme: 'light' },
});
```

`frontend/src/App.vue`:
```vue
<script setup lang="ts">
import AppNotifier from '@/components/AppNotifier.vue';
</script>

<template>
  <v-app>
    <router-view />
    <AppNotifier />
  </v-app>
</template>
```

`frontend/src/main.ts`:
```ts
import { createPinia } from 'pinia';
import { createApp } from 'vue';
import App from './App.vue';
import { setUnauthorizedHandler } from '@/common/services/api/http-client';
import { useAuthStore } from '@/features/auth';
import { vuetify } from './libs/vuetify';
import { router } from './router';

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(vuetify);

// Сессия истекла посреди работы — забываем пользователя и отправляем на вход.
setUnauthorizedHandler(() => {
  useAuthStore().reset();
  const current = router.currentRoute.value;
  if (!current.meta.publicAccess) {
    void router.push({ name: 'login', query: { redirect: current.fullPath } });
  }
});

app.mount('#app');
```

- [ ] **Step 13: Проверить тесты, типы и сборку**

Run: `cd frontend && npm run test:run && npm run typecheck && npm run build`
Expected: тесты PASS, `vue-tsc` без ошибок, `vite build` создаёт `dist/`.

- [ ] **Step 14: Проверить вход вживую**

Run в двух терминалах: `cd backend && make run` и `cd frontend && npm run dev`.
Открыть `http://localhost:5173/`:
- без сессии открывается страница входа;
- `alice` / `wrong-pass` → «Неверный uid или пароль»;
- `alice` / `alice-secret` → главная, в шапке «Alice Admin»;
- обновить страницу → сессия сохранилась;
- «Выйти» → страница входа.

- [ ] **Step 15: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/tsconfig.json frontend/env.d.ts \
  frontend/vite.config.ts frontend/index.html frontend/src
git commit -m "Add frontend skeleton with API layer and login

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 16: Фронтенд — дерево каталога

**Files:**
- Create: `frontend/src/common/services/api/repositories/directory.repository.ts`
- Modify: `frontend/src/common/services/api/api.service.ts`
- Create: `frontend/src/features/directory/index.ts`
- Create: `frontend/src/features/directory/composables/use-directory-tree.ts`
- Create: `frontend/src/features/directory/components/AttributeTable.vue`
- Create: `frontend/src/features/directory/components/EntryDetails.vue`
- Create: `frontend/src/features/directory/views/DirectoryView.vue`
- Modify: `frontend/src/router/index.ts`, `frontend/src/layouts/DefaultLayout.vue`
- Test: `frontend/src/common/services/api/__tests__/directory.repository.spec.ts`
- Test: `frontend/src/features/directory/__tests__/use-directory-tree.spec.ts`

**Interfaces:**
- Consumes: `DirectoryNodeDto`, `DirectoryEntryDto`, `DirectoryNode`, `DirectoryEntry` (задача 15), `useNotifierStore`, `apiService`.
- Produces:
  - `DirectoryRepository` с `children(dn: string | null): Promise<DirectoryNode[]>` и `entry(dn: string | null): Promise<DirectoryEntry>`; `apiService.directory`
  - `useDirectoryTree(repository?)` → `{ items, selected, loadRoot, loadChildren, select }`; `toTreeItem(node)`; тип `TreeItem`
  - маршрут `directory` (`/directory`) и пункт меню «Дерево»

- [ ] **Step 1: Написать падающие тесты**

`frontend/src/common/services/api/__tests__/directory.repository.spec.ts`:
```ts
import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { DirectoryRepository } from '../repositories/directory.repository';

describe('DirectoryRepository', () => {
  it('omits dn for the root and maps nodes', async () => {
    const get = vi.fn().mockResolvedValue({
      data: [{ dn: 'ou=people,dc=example,dc=com', rdn: 'ou=people', objectClass: ['organizationalUnit'], hasChildren: true }],
    });
    const repository = new DirectoryRepository({ get } as unknown as AxiosInstance);

    const nodes = await repository.children(null);

    expect(get).toHaveBeenCalledWith('/directory/children', { params: {} });
    expect(nodes).toEqual([
      { dn: 'ou=people,dc=example,dc=com', rdn: 'ou=people', objectClasses: ['organizationalUnit'], hasChildren: true },
    ]);
  });

  it('passes dn as a query parameter', async () => {
    const get = vi.fn().mockResolvedValue({ data: { dn: 'x', attributes: {}, operationalAttributes: {} } });
    const repository = new DirectoryRepository({ get } as unknown as AxiosInstance);

    await repository.entry('uid=alice,ou=people,dc=example,dc=com');

    expect(get).toHaveBeenCalledWith('/directory/entry', { params: { dn: 'uid=alice,ou=people,dc=example,dc=com' } });
  });
});
```

`frontend/src/features/directory/__tests__/use-directory-tree.spec.ts`:
```ts
import { describe, expect, it, vi } from 'vitest';
import type { DirectoryEntry, DirectoryNode } from '@/common/services/api/models';
import { toTreeItem, useDirectoryTree } from '../composables/use-directory-tree';

const people: DirectoryNode = { dn: 'ou=people,dc=example,dc=com', rdn: 'ou=people', objectClasses: [], hasChildren: true };
const alice: DirectoryNode = { dn: 'uid=alice,ou=people,dc=example,dc=com', rdn: 'uid=alice', objectClasses: [], hasChildren: false };
const rootEntry: DirectoryEntry = { dn: 'dc=example,dc=com', attributes: {}, operationalAttributes: {} };

function fakeRepository() {
  return {
    children: vi.fn().mockResolvedValue([people]),
    entry: vi.fn().mockResolvedValue(rootEntry),
  };
}

describe('toTreeItem', () => {
  it('marks loadable nodes with an empty children array and leaves leaves without it', () => {
    expect(toTreeItem(people).children).toEqual([]);
    expect('children' in toTreeItem(alice)).toBe(false);
  });
});

describe('useDirectoryTree', () => {
  it('starts from the base entry and loads children on demand', async () => {
    const repository = fakeRepository();
    const tree = useDirectoryTree(repository);

    await tree.loadRoot();
    const [root] = tree.items.value;
    expect(root?.id).toBe('dc=example,dc=com');

    if (!root) throw new Error('root item is missing');
    await tree.loadChildren(root);

    expect(repository.children).toHaveBeenCalledWith('dc=example,dc=com');
    expect(root.children?.map((child) => child.id)).toEqual(['ou=people,dc=example,dc=com']);
  });

  it('selects an entry by dn', async () => {
    const repository = fakeRepository();
    const tree = useDirectoryTree(repository);

    await tree.select('dc=example,dc=com');

    expect(repository.entry).toHaveBeenCalledWith('dc=example,dc=com');
    expect(tree.selected.value).toEqual(rootEntry);
  });
});
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd frontend && npm run test:run`
Expected: FAIL — модулей `directory.repository` и `use-directory-tree` нет.

- [ ] **Step 3: Реализовать репозиторий и подключить его**

`frontend/src/common/services/api/repositories/directory.repository.ts`:
```ts
import type { AxiosInstance } from 'axios';
import type { DirectoryEntryDto, DirectoryNodeDto } from '../dtos';
import type { DirectoryEntry, DirectoryNode } from '../models';

export class DirectoryRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  /** dn = null — потомки корня каталога (base DN). */
  async children(dn: string | null): Promise<DirectoryNode[]> {
    const { data } = await this.http.get<DirectoryNodeDto[]>('/directory/children', { params: dnParams(dn) });
    return data.map(toDirectoryNode);
  }

  async entry(dn: string | null): Promise<DirectoryEntry> {
    const { data } = await this.http.get<DirectoryEntryDto>('/directory/entry', { params: dnParams(dn) });
    return { dn: data.dn, attributes: data.attributes, operationalAttributes: data.operationalAttributes };
  }
}

function dnParams(dn: string | null): Record<string, string> {
  return dn === null ? {} : { dn };
}

function toDirectoryNode(dto: DirectoryNodeDto): DirectoryNode {
  return { dn: dto.dn, rdn: dto.rdn, objectClasses: dto.objectClass, hasChildren: dto.hasChildren };
}
```

`frontend/src/common/services/api/api.service.ts` (целиком):
```ts
import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';
import { DirectoryRepository } from './repositories/directory.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
  directory: new DirectoryRepository(http),
};
```

- [ ] **Step 4: Реализовать composable дерева**

`frontend/src/features/directory/composables/use-directory-tree.ts`:
```ts
import { ref } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import type { DirectoryEntry, DirectoryNode } from '@/common/services/api/models';
import type { DirectoryRepository } from '@/common/services/api/repositories/directory.repository';

export interface TreeItem {
  id: string;
  title: string;
  children?: TreeItem[];
}

type TreeSource = Pick<DirectoryRepository, 'children' | 'entry'>;

// Пустой массив children — сигнал VTreeview: у узла есть потомки, загрузить их при раскрытии.
// Без поля children узел считается листом и стрелки раскрытия у него нет.
export function toTreeItem(node: DirectoryNode): TreeItem {
  const item: TreeItem = { id: node.dn, title: node.rdn };
  if (node.hasChildren) {
    item.children = [];
  }
  return item;
}

export function useDirectoryTree(repository: TreeSource = apiService.directory) {
  const items = ref<TreeItem[]>([]);
  const selected = ref<DirectoryEntry | null>(null);

  async function loadRoot(): Promise<void> {
    const root = await repository.entry(null);
    items.value = [{ id: root.dn, title: root.dn, children: [] }];
  }

  async function loadChildren(item: TreeItem): Promise<void> {
    const nodes = await repository.children(item.id);
    item.children?.push(...nodes.map(toTreeItem));
  }

  async function select(dn: string): Promise<void> {
    selected.value = await repository.entry(dn);
  }

  return { items, selected, loadRoot, loadChildren, select };
}
```

- [ ] **Step 5: Запустить тесты**

Run: `cd frontend && npm run test:run`
Expected: PASS.

- [ ] **Step 6: Реализовать компоненты и страницу**

`frontend/src/features/directory/components/AttributeTable.vue`:
```vue
<script setup lang="ts">
import { computed } from 'vue';

const props = defineProps<{ title: string; attributes: Record<string, string[]> }>();

const rows = computed(() =>
  Object.entries(props.attributes)
    .map(([name, values]) => ({ name, values }))
    .sort((left, right) => left.name.localeCompare(right.name)),
);
</script>

<template>
  <v-card :title="title" variant="outlined" class="mb-4">
    <v-table density="compact">
      <tbody>
        <tr v-for="row in rows" :key="row.name">
          <td class="font-weight-medium">{{ row.name }}</td>
          <td>
            <div v-for="(value, index) in row.values" :key="index">{{ value }}</div>
          </td>
        </tr>
      </tbody>
    </v-table>
  </v-card>
</template>
```

`frontend/src/features/directory/components/EntryDetails.vue`:
```vue
<script setup lang="ts">
import type { DirectoryEntry } from '@/common/services/api/models';
import AttributeTable from './AttributeTable.vue';

defineProps<{ entry: DirectoryEntry }>();
</script>

<template>
  <div>
    <v-alert type="info" variant="tonal" class="mb-4">
      <div class="text-caption">DN</div>
      <code>{{ entry.dn }}</code>
    </v-alert>
    <AttributeTable title="Атрибуты записи" :attributes="entry.attributes" />
    <AttributeTable title="Служебные атрибуты (ведёт сервер)" :attributes="entry.operationalAttributes" />
  </div>
</template>
```

`frontend/src/features/directory/views/DirectoryView.vue`:
```vue
<script setup lang="ts">
import { onMounted } from 'vue';
import { useNotifierStore } from '@/stores/notifier.store';
import EntryDetails from '../components/EntryDetails.vue';
import { type TreeItem, useDirectoryTree } from '../composables/use-directory-tree';

const notifier = useNotifierStore();
const tree = useDirectoryTree();
const { items, selected } = tree;

async function guarded(action: () => Promise<void>): Promise<void> {
  try {
    await action();
  } catch (caught) {
    notifier.error(caught);
  }
}

// VTreeview передаёт исходный объект элемента, то есть наш TreeItem.
function loadChildren(item: unknown): Promise<void> {
  return guarded(() => tree.loadChildren(item as TreeItem));
}

function onActivated(activated: unknown): void {
  const [dn] = Array.isArray(activated) ? activated : [];
  if (typeof dn === 'string') {
    void guarded(() => tree.select(dn));
  }
}

onMounted(() => guarded(tree.loadRoot));
</script>

<template>
  <v-container fluid>
    <v-row>
      <v-col cols="12" md="5">
        <v-card title="Дерево каталога">
          <v-treeview
            :items="items"
            item-value="id"
            item-title="title"
            :load-children="loadChildren"
            activatable
            density="compact"
            @update:activated="onActivated"
          />
        </v-card>
      </v-col>
      <v-col cols="12" md="7">
        <EntryDetails v-if="selected" :entry="selected" />
        <v-alert v-else type="info" variant="tonal">Выберите запись в дереве, чтобы увидеть её атрибуты.</v-alert>
      </v-col>
    </v-row>
  </v-container>
</template>
```

`frontend/src/features/directory/index.ts`:
```ts
export { default as DirectoryView } from './views/DirectoryView.vue';
```

- [ ] **Step 7: Подключить маршрут и пункт меню**

В `frontend/src/router/index.ts` добавить импорт и дочерний маршрут:
```ts
import { DirectoryView } from '@/features/directory';
```
```ts
    children: [
      { path: '', name: 'home', component: HomeView },
      { path: 'directory', name: 'directory', component: DirectoryView },
    ],
```

В `frontend/src/layouts/DefaultLayout.vue` расширить `NAVIGATION`:
```ts
const NAVIGATION: NavigationItem[] = [
  { title: 'Главная', icon: 'mdi-home', routeName: 'home' },
  { title: 'Дерево', icon: 'mdi-file-tree', routeName: 'directory' },
];
```

- [ ] **Step 8: Проверить тесты, типы и страницу вживую**

Run: `cd frontend && npm run test:run && npm run typecheck`
Expected: PASS, без ошибок типов.

Вживую (бэкенд и `npm run dev` запущены, вход как alice): «Дерево» → корень `dc=example,dc=com` → раскрыть → `ou=groups`, `ou=people` → раскрыть `ou=people` → `uid=alice`. Клик по `uid=alice` справа показывает обычные атрибуты (`cn`, `mail`, `objectClass`, …; без `userPassword`) и служебные (`memberOf`, `entryUUID`, `structuralObjectClass`, …).

- [ ] **Step 9: Commit**

```bash
git add frontend/src
git commit -m "Add directory tree browser

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 17: Фронтенд — пользователи

**Files:**
- Create: `frontend/src/common/services/api/repositories/users.repository.ts`
- Modify: `frontend/src/common/services/api/api.service.ts`
- Create: `frontend/src/features/users/index.ts`
- Create: `frontend/src/features/users/helpers/emails.ts`
- Create: `frontend/src/features/users/components/UserFormDialog.vue`
- Create: `frontend/src/features/users/components/PasswordDialog.vue`
- Create: `frontend/src/features/users/views/UsersView.vue`
- Create: `frontend/src/features/users/views/UserView.vue`
- Modify: `frontend/src/router/index.ts`, `frontend/src/layouts/DefaultLayout.vue`
- Test: `frontend/src/common/services/api/__tests__/users.repository.spec.ts`
- Test: `frontend/src/features/users/__tests__/emails.spec.ts`

**Interfaces:**
- Consumes: `UserDto`, `UserDetailsDto`, `User`, `UserDetails`, `UserDraft`, `UserChanges` (задача 15), `ConfirmDialog`, `useNotifierStore`, `errorMessage`.
- Produces:
  - `UsersRepository`: `list(): Promise<User[]>`, `get(uid): Promise<UserDetails>`, `create(draft: UserDraft): Promise<User>`, `update(uid, changes: UserChanges): Promise<User>`, `setPassword(uid, password): Promise<void>`, `remove(uid): Promise<void>`; `apiService.users`
  - `parseEmails(text: string): string[]`, `formatEmails(emails: string[]): string`
  - маршруты `users` (`/users`) и `user` (`/users/:uid`, prop `uid`); пункт меню «Пользователи»
  - `UserView` показывает группы пользователя чипами — задача 18 превратит их в ссылки

- [ ] **Step 1: Написать падающие тесты**

`frontend/src/common/services/api/__tests__/users.repository.spec.ts`:
```ts
import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { UsersRepository } from '../repositories/users.repository';

const johnDto = { uid: 'jdoe', cn: 'Doe, John', sn: 'Doe', mail: ['jdoe@example.com'] };
const john = { uid: 'jdoe', commonName: 'Doe, John', surname: 'Doe', emails: ['jdoe@example.com'] };

describe('UsersRepository', () => {
  it('maps users and details', async () => {
    const get = vi.fn()
      .mockResolvedValueOnce({ data: [johnDto] })
      .mockResolvedValueOnce({ data: { ...johnDto, groups: ['team'] } });
    const repository = new UsersRepository({ get } as unknown as AxiosInstance);

    expect(await repository.list()).toEqual([john]);
    expect(await repository.get('jdoe')).toEqual({ ...john, groups: ['team'] });
    expect(get).toHaveBeenLastCalledWith('/users/jdoe');
  });

  it('sends LDAP attribute names to the backend', async () => {
    const post = vi.fn().mockResolvedValue({ data: johnDto });
    const put = vi.fn().mockResolvedValue({ data: johnDto });
    const repository = new UsersRepository({ post, put } as unknown as AxiosInstance);

    await repository.create({ uid: 'jdoe', commonName: 'Doe, John', surname: 'Doe', emails: [], password: 'correct-horse' });
    await repository.update('jdoe', { commonName: 'John', surname: 'Doe', emails: ['j@example.com'] });

    expect(post).toHaveBeenCalledWith('/users', { uid: 'jdoe', cn: 'Doe, John', sn: 'Doe', mail: [], password: 'correct-horse' });
    expect(put).toHaveBeenCalledWith('/users/jdoe', { cn: 'John', sn: 'Doe', mail: ['j@example.com'] });
  });
});
```

`frontend/src/features/users/__tests__/emails.spec.ts`:
```ts
import { describe, expect, it } from 'vitest';
import { formatEmails, parseEmails } from '../helpers/emails';

describe('emails', () => {
  it('splits by commas, semicolons and spaces and drops empties', () => {
    expect(parseEmails(' a@example.com, b@example.com;c@example.com  ')).toEqual(['a@example.com', 'b@example.com', 'c@example.com']);
    expect(parseEmails('   ')).toEqual([]);
  });

  it('formats as a comma-separated list', () => {
    expect(formatEmails(['a@example.com', 'b@example.com'])).toBe('a@example.com, b@example.com');
  });
});
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd frontend && npm run test:run`
Expected: FAIL — модулей `users.repository` и `emails` нет.

- [ ] **Step 3: Реализовать репозиторий и помощники**

`frontend/src/common/services/api/repositories/users.repository.ts`:
```ts
import type { AxiosInstance } from 'axios';
import type { UserDetailsDto, UserDto } from '../dtos';
import type { User, UserChanges, UserDetails, UserDraft } from '../models';

export class UsersRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  async list(): Promise<User[]> {
    const { data } = await this.http.get<UserDto[]>('/users');
    return data.map(toUser);
  }

  async get(uid: string): Promise<UserDetails> {
    const { data } = await this.http.get<UserDetailsDto>(userPath(uid));
    return { ...toUser(data), groups: data.groups };
  }

  async create(draft: UserDraft): Promise<User> {
    const { data } = await this.http.post<UserDto>('/users', {
      uid: draft.uid, ...toChangesDto(draft), password: draft.password,
    });
    return toUser(data);
  }

  async update(uid: string, changes: UserChanges): Promise<User> {
    const { data } = await this.http.put<UserDto>(userPath(uid), toChangesDto(changes));
    return toUser(data);
  }

  async setPassword(uid: string, password: string): Promise<void> {
    await this.http.put(`${userPath(uid)}/password`, { password });
  }

  async remove(uid: string): Promise<void> {
    await this.http.delete(userPath(uid));
  }
}

function userPath(uid: string): string {
  return `/users/${encodeURIComponent(uid)}`;
}

function toUser(dto: UserDto): User {
  return { uid: dto.uid, commonName: dto.cn, surname: dto.sn, emails: dto.mail };
}

function toChangesDto(changes: UserChanges): Omit<UserDto, 'uid'> {
  return { cn: changes.commonName, sn: changes.surname, mail: changes.emails };
}
```

`frontend/src/features/users/helpers/emails.ts`:
```ts
export function parseEmails(text: string): string[] {
  return text.split(/[\s,;]+/).filter((part) => part !== '');
}

export function formatEmails(emails: string[]): string {
  return emails.join(', ');
}
```

`frontend/src/common/services/api/api.service.ts` (целиком):
```ts
import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';
import { DirectoryRepository } from './repositories/directory.repository';
import { UsersRepository } from './repositories/users.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
  directory: new DirectoryRepository(http),
  users: new UsersRepository(http),
};
```

- [ ] **Step 4: Запустить тесты**

Run: `cd frontend && npm run test:run`
Expected: PASS.

- [ ] **Step 5: Реализовать диалоги**

`frontend/src/features/users/components/UserFormDialog.vue`:
```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { errorMessage } from '@/common/services/api/api-error';
import type { User } from '@/common/services/api/models';
import { formatEmails, parseEmails } from '../helpers/emails';

const props = defineProps<{ user: User | null }>();
const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [user: User] }>();

const uid = ref('');
const commonName = ref('');
const surname = ref('');
const emails = ref('');
const password = ref('');
const errorText = ref('');
const saving = ref(false);
const isEditing = computed(() => props.user !== null);

watch(open, (isOpen) => {
  if (isOpen) {
    reset();
  }
});

function reset(): void {
  uid.value = props.user?.uid ?? '';
  commonName.value = props.user?.commonName ?? '';
  surname.value = props.user?.surname ?? '';
  emails.value = formatEmails(props.user?.emails ?? []);
  password.value = '';
  errorText.value = '';
}

function persist(): Promise<User> {
  const changes = { commonName: commonName.value, surname: surname.value, emails: parseEmails(emails.value) };
  if (props.user) {
    return apiService.users.update(props.user.uid, changes);
  }
  return apiService.users.create({ uid: uid.value.trim(), password: password.value, ...changes });
}

async function save(): Promise<void> {
  saving.value = true;
  errorText.value = '';
  try {
    emit('saved', await persist());
    open.value = false;
  } catch (caught) {
    errorText.value = errorMessage(caught);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <v-dialog v-model="open" max-width="520">
    <v-card :title="isEditing ? 'Редактирование пользователя' : 'Новый пользователь'">
      <v-card-text>
        <v-text-field v-model="uid" label="uid" :disabled="isEditing" hint="Строчные латинские буквы, цифры, «.», «_», «-»" />
        <v-text-field v-model="commonName" label="cn — полное имя" />
        <v-text-field v-model="surname" label="sn — фамилия" />
        <v-text-field v-model="emails" label="mail — адреса через запятую" />
        <v-text-field v-if="!isEditing" v-model="password" label="Пароль" type="password" hint="Не короче 8 символов" />
        <v-alert v-if="errorText" type="error" variant="tonal">{{ errorText }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">Отмена</v-btn>
        <v-btn color="primary" :loading="saving" @click="save">Сохранить</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
```

`frontend/src/features/users/components/PasswordDialog.vue`:
```vue
<script setup lang="ts">
import { ref, watch } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { errorMessage } from '@/common/services/api/api-error';

const props = defineProps<{ uid: string }>();
const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [] }>();

const password = ref('');
const errorText = ref('');
const saving = ref(false);

watch(open, (isOpen) => {
  if (isOpen) {
    password.value = '';
    errorText.value = '';
  }
});

async function save(): Promise<void> {
  saving.value = true;
  errorText.value = '';
  try {
    await apiService.users.setPassword(props.uid, password.value);
    emit('saved');
    open.value = false;
  } catch (caught) {
    errorText.value = errorMessage(caught);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <v-dialog v-model="open" max-width="420">
    <v-card title="Новый пароль" subtitle="Сервер сохранит его хэшем через операцию Password Modify">
      <v-card-text>
        <v-text-field v-model="password" label="Пароль" type="password" hint="Не короче 8 символов" autofocus />
        <v-alert v-if="errorText" type="error" variant="tonal">{{ errorText }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">Отмена</v-btn>
        <v-btn color="primary" :loading="saving" @click="save">Сохранить</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
```

- [ ] **Step 6: Реализовать страницы**

`frontend/src/features/users/views/UsersView.vue`:
```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { apiService } from '@/common/services/api/api.service';
import type { User } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import UserFormDialog from '../components/UserFormDialog.vue';

const HEADERS = [
  { title: 'uid', key: 'uid' },
  { title: 'cn', key: 'commonName' },
  { title: 'sn', key: 'surname' },
  { title: 'mail', key: 'emails' },
];

const notifier = useNotifierStore();
const router = useRouter();
const users = ref<User[]>([]);
const loading = ref(false);
const creating = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    users.value = await apiService.users.list();
  } catch (caught) {
    notifier.error(caught);
  } finally {
    loading.value = false;
  }
}

function onRowClick(_event: Event, row: { item: User }): void {
  void router.push({ name: 'user', params: { uid: row.item.uid } });
}

function onCreated(created: User): void {
  notifier.success(`Пользователь ${created.uid} создан`);
  void load();
}

onMounted(load);
</script>

<template>
  <v-container>
    <v-card title="Пользователи" subtitle="Записи inetOrgPerson в ou=people">
      <template #append>
        <v-btn color="primary" prepend-icon="mdi-account-plus" @click="creating = true">Создать</v-btn>
      </template>
      <v-data-table :headers="HEADERS" :items="users" :loading="loading" item-value="uid" hover @click:row="onRowClick">
        <template #[`item.emails`]="{ value }">{{ value.join(', ') }}</template>
      </v-data-table>
    </v-card>
    <UserFormDialog v-model="creating" :user="null" @saved="onCreated" />
  </v-container>
</template>
```

`frontend/src/features/users/views/UserView.vue`:
```vue
<script setup lang="ts">
import { ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { apiService } from '@/common/services/api/api.service';
import type { UserDetails } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import PasswordDialog from '../components/PasswordDialog.vue';
import UserFormDialog from '../components/UserFormDialog.vue';

const props = defineProps<{ uid: string }>();

const notifier = useNotifierStore();
const router = useRouter();
const details = ref<UserDetails | null>(null);
const editing = ref(false);
const changingPassword = ref(false);
const confirmingDelete = ref(false);
const deleting = ref(false);

async function load(): Promise<void> {
  try {
    details.value = await apiService.users.get(props.uid);
  } catch (caught) {
    notifier.error(caught);
  }
}

function onSaved(): void {
  notifier.success('Изменения сохранены');
  void load();
}

async function remove(): Promise<void> {
  deleting.value = true;
  try {
    await apiService.users.remove(props.uid);
    notifier.success(`Пользователь ${props.uid} удалён`);
    await router.push({ name: 'users' });
  } catch (caught) {
    notifier.error(caught);
  } finally {
    deleting.value = false;
    confirmingDelete.value = false;
  }
}

watch(() => props.uid, load, { immediate: true });
</script>

<template>
  <v-container>
    <v-card v-if="details" :title="details.commonName" :subtitle="`uid=${details.uid},ou=people`">
      <v-card-text>
        <v-list density="compact">
          <v-list-item title="sn" :subtitle="details.surname" />
          <v-list-item title="mail" :subtitle="details.emails.join(', ') || '—'" />
        </v-list>
        <div class="mt-4">Группы — из служебного атрибута memberOf, его ведёт сервер:</div>
        <v-chip v-for="name in details.groups" :key="name" class="mr-2 mt-2">{{ name }}</v-chip>
        <span v-if="details.groups.length === 0">нет</span>
      </v-card-text>
      <v-card-actions>
        <v-btn prepend-icon="mdi-pencil" @click="editing = true">Редактировать</v-btn>
        <v-btn prepend-icon="mdi-key" @click="changingPassword = true">Сменить пароль</v-btn>
        <v-spacer />
        <v-btn color="error" prepend-icon="mdi-delete" @click="confirmingDelete = true">Удалить</v-btn>
      </v-card-actions>
    </v-card>
    <UserFormDialog v-model="editing" :user="details" @saved="onSaved" />
    <PasswordDialog v-model="changingPassword" :uid="uid" @saved="notifier.success('Пароль изменён')" />
    <ConfirmDialog
      v-model="confirmingDelete"
      title="Удалить пользователя?"
      :text="`Запись uid=${uid} будет удалена. Из групп её уберёт сам сервер (оверлей refint).`"
      :loading="deleting"
      @confirm="remove"
    />
  </v-container>
</template>
```

`frontend/src/features/users/index.ts`:
```ts
export { default as UsersView } from './views/UsersView.vue';
export { default as UserView } from './views/UserView.vue';
```

- [ ] **Step 7: Подключить маршруты и пункт меню**

В `frontend/src/router/index.ts`:
```ts
import { UsersView, UserView } from '@/features/users';
```
```ts
    children: [
      { path: '', name: 'home', component: HomeView },
      { path: 'directory', name: 'directory', component: DirectoryView },
      { path: 'users', name: 'users', component: UsersView },
      { path: 'users/:uid', name: 'user', component: UserView, props: true },
    ],
```

В `frontend/src/layouts/DefaultLayout.vue`:
```ts
const NAVIGATION: NavigationItem[] = [
  { title: 'Главная', icon: 'mdi-home', routeName: 'home' },
  { title: 'Дерево', icon: 'mdi-file-tree', routeName: 'directory' },
  { title: 'Пользователи', icon: 'mdi-account-multiple', routeName: 'users' },
];
```

- [ ] **Step 8: Проверить тесты, типы и сценарии вживую**

Run: `cd frontend && npm run test:run && npm run typecheck`
Expected: PASS, без ошибок типов.

Вживую (вход как alice):
- создать `jdoe`, cn `Doe, John`, sn `Doe`, mail `jdoe@example.com`, пароль `correct-horse` → уведомление «создан», строка в таблице;
- создать с uid `John` → в диалоге текст ошибки про формат uid;
- создать с паролем `short` → ошибка про длину пароля, пользователь не появился;
- открыть `jdoe` → «Редактировать», убрать mail → mail «—»;
- «Сменить пароль» → `another-horse` → «Пароль изменён»;
- открыть `alice` → «Удалить» → уведомление «you cannot delete yourself».

- [ ] **Step 9: Commit**

```bash
git add frontend/src
git commit -m "Add user management pages

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 18: Фронтенд — группы

**Files:**
- Create: `frontend/src/common/services/api/repositories/groups.repository.ts`
- Modify: `frontend/src/common/services/api/api.service.ts`
- Create: `frontend/src/features/groups/index.ts`
- Create: `frontend/src/features/groups/helpers/members.ts`
- Create: `frontend/src/features/groups/components/GroupCreateDialog.vue`
- Create: `frontend/src/features/groups/views/GroupsView.vue`
- Create: `frontend/src/features/groups/views/GroupView.vue`
- Modify: `frontend/src/router/index.ts`, `frontend/src/layouts/DefaultLayout.vue`, `frontend/src/features/users/views/UserView.vue`
- Test: `frontend/src/common/services/api/__tests__/groups.repository.spec.ts`
- Test: `frontend/src/features/groups/__tests__/members.spec.ts`

**Interfaces:**
- Consumes: `GroupSummaryDto`, `GroupDto`, `GroupSummary`, `Group`, `GroupMember`, `GroupDraft`, `User` (задача 15), `apiService.users.list` (задача 17), `ConfirmDialog`, `useNotifierStore`.
- Produces:
  - `GroupsRepository`: `list(): Promise<GroupSummary[]>`, `get(name): Promise<Group>`, `create(draft: GroupDraft): Promise<Group>`, `remove(name): Promise<void>`, `addMember(name, uid): Promise<void>`, `removeMember(name, uid): Promise<void>`; `apiService.groups`
  - `candidateMembers(users: User[], group: Group): User[]`
  - маршруты `groups` (`/groups`) и `group` (`/groups/:name`, prop `name`); пункт меню «Группы»

- [ ] **Step 1: Написать падающие тесты**

`frontend/src/common/services/api/__tests__/groups.repository.spec.ts`:
```ts
import type { AxiosInstance } from 'axios';
import { describe, expect, it, vi } from 'vitest';
import { GroupsRepository } from '../repositories/groups.repository';

describe('GroupsRepository', () => {
  it('maps a member without uid to null', async () => {
    const get = vi.fn().mockResolvedValue({
      data: {
        cn: 'service',
        description: '',
        members: [
          { dn: 'uid=alice,ou=people,dc=example,dc=com', uid: 'alice' },
          { dn: 'ou=robots,dc=example,dc=com', uid: '' },
        ],
      },
    });
    const repository = new GroupsRepository({ get } as unknown as AxiosInstance);

    const group = await repository.get('service');

    expect(group.members).toEqual([
      { dn: 'uid=alice,ou=people,dc=example,dc=com', uid: 'alice' },
      { dn: 'ou=robots,dc=example,dc=com', uid: null },
    ]);
  });

  it('creates a group with member uids and encodes path parts', async () => {
    const post = vi.fn().mockResolvedValue({ data: { cn: 'team', description: 'Team', members: [] } });
    const del = vi.fn().mockResolvedValue({ data: null });
    const repository = new GroupsRepository({ post, delete: del } as unknown as AxiosInstance);

    await repository.create({ name: 'team', description: 'Team', memberUids: ['alice'] });
    await repository.removeMember('team', 'a.b');

    expect(post).toHaveBeenCalledWith('/groups', { cn: 'team', description: 'Team', members: ['alice'] });
    expect(del).toHaveBeenCalledWith('/groups/team/members/a.b');
  });
});
```

`frontend/src/features/groups/__tests__/members.spec.ts`:
```ts
import { describe, expect, it } from 'vitest';
import type { Group, User } from '@/common/services/api/models';
import { candidateMembers } from '../helpers/members';

function person(uid: string): User {
  return { uid, commonName: uid, surname: uid, emails: [] };
}

describe('candidateMembers', () => {
  it('offers only users who are not in the group yet', () => {
    const group: Group = {
      name: 'team',
      description: '',
      members: [
        { dn: 'uid=alice,ou=people,dc=example,dc=com', uid: 'alice' },
        { dn: 'ou=robots,dc=example,dc=com', uid: null },
      ],
    };

    expect(candidateMembers([person('alice'), person('bob')], group).map((user) => user.uid)).toEqual(['bob']);
  });
});
```

- [ ] **Step 2: Запустить и убедиться, что падают**

Run: `cd frontend && npm run test:run`
Expected: FAIL — модулей `groups.repository` и `members` нет.

- [ ] **Step 3: Реализовать репозиторий и помощник**

`frontend/src/common/services/api/repositories/groups.repository.ts`:
```ts
import type { AxiosInstance } from 'axios';
import type { GroupDto, GroupSummaryDto } from '../dtos';
import type { Group, GroupDraft, GroupSummary } from '../models';

export class GroupsRepository {
  private readonly http: AxiosInstance;

  constructor(http: AxiosInstance) {
    this.http = http;
  }

  async list(): Promise<GroupSummary[]> {
    const { data } = await this.http.get<GroupSummaryDto[]>('/groups');
    return data.map((dto) => ({ name: dto.cn, description: dto.description, memberCount: dto.memberCount }));
  }

  async get(name: string): Promise<Group> {
    const { data } = await this.http.get<GroupDto>(groupPath(name));
    return toGroup(data);
  }

  async create(draft: GroupDraft): Promise<Group> {
    const { data } = await this.http.post<GroupDto>('/groups', {
      cn: draft.name, description: draft.description, members: draft.memberUids,
    });
    return toGroup(data);
  }

  async remove(name: string): Promise<void> {
    await this.http.delete(groupPath(name));
  }

  async addMember(name: string, uid: string): Promise<void> {
    await this.http.post(`${groupPath(name)}/members`, { uid });
  }

  async removeMember(name: string, uid: string): Promise<void> {
    await this.http.delete(`${groupPath(name)}/members/${encodeURIComponent(uid)}`);
  }
}

function groupPath(name: string): string {
  return `/groups/${encodeURIComponent(name)}`;
}

function toGroup(dto: GroupDto): Group {
  return {
    name: dto.cn,
    description: dto.description,
    members: dto.members.map((member) => ({ dn: member.dn, uid: member.uid === '' ? null : member.uid })),
  };
}
```

`frontend/src/features/groups/helpers/members.ts`:
```ts
import type { Group, User } from '@/common/services/api/models';

export function candidateMembers(users: User[], group: Group): User[] {
  const memberUids = new Set(group.members.map((member) => member.uid));
  return users.filter((user) => !memberUids.has(user.uid));
}
```

`frontend/src/common/services/api/api.service.ts` (целиком):
```ts
import { createHttpClient } from './http-client';
import { AuthRepository } from './repositories/auth.repository';
import { DirectoryRepository } from './repositories/directory.repository';
import { GroupsRepository } from './repositories/groups.repository';
import { UsersRepository } from './repositories/users.repository';

const http = createHttpClient();

export const apiService = {
  auth: new AuthRepository(http),
  directory: new DirectoryRepository(http),
  groups: new GroupsRepository(http),
  users: new UsersRepository(http),
};
```

- [ ] **Step 4: Запустить тесты**

Run: `cd frontend && npm run test:run`
Expected: PASS.

- [ ] **Step 5: Реализовать диалог создания и страницы**

`frontend/src/features/groups/components/GroupCreateDialog.vue`:
```vue
<script setup lang="ts">
import { ref, watch } from 'vue';
import { apiService } from '@/common/services/api/api.service';
import { errorMessage } from '@/common/services/api/api-error';
import type { Group, User } from '@/common/services/api/models';

const open = defineModel<boolean>({ required: true });
const emit = defineEmits<{ saved: [group: Group] }>();

const name = ref('');
const description = ref('');
const memberUids = ref<string[]>([]);
const users = ref<User[]>([]);
const errorText = ref('');
const saving = ref(false);

watch(open, async (isOpen) => {
  if (!isOpen) {
    return;
  }
  name.value = '';
  description.value = '';
  memberUids.value = [];
  errorText.value = '';
  try {
    users.value = await apiService.users.list();
  } catch (caught) {
    errorText.value = errorMessage(caught);
  }
});

async function save(): Promise<void> {
  saving.value = true;
  errorText.value = '';
  try {
    const created = await apiService.groups.create({
      name: name.value.trim(), description: description.value, memberUids: memberUids.value,
    });
    emit('saved', created);
    open.value = false;
  } catch (caught) {
    errorText.value = errorMessage(caught);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <v-dialog v-model="open" max-width="520">
    <v-card title="Новая группа" subtitle="groupOfUniqueNames требует хотя бы одного участника">
      <v-card-text>
        <v-text-field v-model="name" label="cn — имя группы" hint="Строчные латинские буквы, цифры, «.», «_», «-»" />
        <v-text-field v-model="description" label="description" />
        <v-autocomplete
          v-model="memberUids"
          :items="users"
          item-title="uid"
          item-value="uid"
          label="Участники (uniqueMember)"
          multiple
          chips
        />
        <v-alert v-if="errorText" type="error" variant="tonal">{{ errorText }}</v-alert>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn @click="open = false">Отмена</v-btn>
        <v-btn color="primary" :loading="saving" @click="save">Создать</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>
```

`frontend/src/features/groups/views/GroupsView.vue`:
```vue
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { apiService } from '@/common/services/api/api.service';
import type { Group, GroupSummary } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import GroupCreateDialog from '../components/GroupCreateDialog.vue';

const HEADERS = [
  { title: 'cn', key: 'name' },
  { title: 'description', key: 'description' },
  { title: 'Участников', key: 'memberCount' },
];

const notifier = useNotifierStore();
const router = useRouter();
const groups = ref<GroupSummary[]>([]);
const loading = ref(false);
const creating = ref(false);

async function load(): Promise<void> {
  loading.value = true;
  try {
    groups.value = await apiService.groups.list();
  } catch (caught) {
    notifier.error(caught);
  } finally {
    loading.value = false;
  }
}

function onRowClick(_event: Event, row: { item: GroupSummary }): void {
  void router.push({ name: 'group', params: { name: row.item.name } });
}

function onCreated(created: Group): void {
  notifier.success(`Группа ${created.name} создана`);
  void load();
}

onMounted(load);
</script>

<template>
  <v-container>
    <v-card title="Группы" subtitle="Записи groupOfUniqueNames в ou=groups">
      <template #append>
        <v-btn color="primary" prepend-icon="mdi-account-group" @click="creating = true">Создать</v-btn>
      </template>
      <v-data-table :headers="HEADERS" :items="groups" :loading="loading" item-value="name" hover @click:row="onRowClick" />
    </v-card>
    <GroupCreateDialog v-model="creating" @saved="onCreated" />
  </v-container>
</template>
```

`frontend/src/features/groups/views/GroupView.vue`:
```vue
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import { apiService } from '@/common/services/api/api.service';
import type { Group, User } from '@/common/services/api/models';
import { useNotifierStore } from '@/stores/notifier.store';
import { candidateMembers } from '../helpers/members';

const props = defineProps<{ name: string }>();

const notifier = useNotifierStore();
const router = useRouter();
const group = ref<Group | null>(null);
const users = ref<User[]>([]);
const newMember = ref<string | null>(null);
const confirmingDelete = ref(false);
const deleting = ref(false);

const candidates = computed(() => (group.value ? candidateMembers(users.value, group.value) : []));

async function load(): Promise<void> {
  try {
    [group.value, users.value] = await Promise.all([apiService.groups.get(props.name), apiService.users.list()]);
  } catch (caught) {
    notifier.error(caught);
  }
}

async function runAndReload(action: () => Promise<void>, successMessage: string): Promise<void> {
  try {
    await action();
    notifier.success(successMessage);
    await load();
  } catch (caught) {
    notifier.error(caught);
  }
}

function addMember(): void {
  const uid = newMember.value;
  if (!uid) {
    return;
  }
  newMember.value = null;
  void runAndReload(() => apiService.groups.addMember(props.name, uid), `${uid} добавлен в группу`);
}

function removeMember(uid: string): void {
  void runAndReload(() => apiService.groups.removeMember(props.name, uid), `${uid} убран из группы`);
}

async function remove(): Promise<void> {
  deleting.value = true;
  try {
    await apiService.groups.remove(props.name);
    notifier.success(`Группа ${props.name} удалена`);
    await router.push({ name: 'groups' });
  } catch (caught) {
    notifier.error(caught);
  } finally {
    deleting.value = false;
    confirmingDelete.value = false;
  }
}

watch(() => props.name, load, { immediate: true });
</script>

<template>
  <v-container>
    <v-card v-if="group" :title="group.name" :subtitle="group.description || 'без описания'">
      <v-card-text>
        <v-table density="compact">
          <thead>
            <tr>
              <th>uniqueMember (DN)</th>
              <th>uid</th>
              <th />
            </tr>
          </thead>
          <tbody>
            <tr v-for="member in group.members" :key="member.dn">
              <td><code>{{ member.dn }}</code></td>
              <td>{{ member.uid ?? '— вне ou=people' }}</td>
              <td class="text-right">
                <v-btn
                  v-if="member.uid"
                  icon="mdi-account-remove"
                  size="small"
                  variant="text"
                  title="Убрать из группы"
                  @click="removeMember(member.uid)"
                />
              </td>
            </tr>
          </tbody>
        </v-table>
        <div class="d-flex ga-2 mt-4">
          <v-autocomplete
            v-model="newMember"
            :items="candidates"
            item-title="uid"
            item-value="uid"
            label="Добавить участника"
            density="compact"
            hide-details
          />
          <v-btn color="primary" :disabled="!newMember" @click="addMember">Добавить</v-btn>
        </div>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn color="error" prepend-icon="mdi-delete" @click="confirmingDelete = true">Удалить группу</v-btn>
      </v-card-actions>
    </v-card>
    <ConfirmDialog
      v-model="confirmingDelete"
      title="Удалить группу?"
      :text="`Запись cn=${name} будет удалена. У участников исчезнет memberOf этой группы.`"
      :loading="deleting"
      @confirm="remove"
    />
  </v-container>
</template>
```

`frontend/src/features/groups/index.ts`:
```ts
export { default as GroupsView } from './views/GroupsView.vue';
export { default as GroupView } from './views/GroupView.vue';
```

- [ ] **Step 6: Подключить маршруты, меню и ссылки из карточки пользователя**

В `frontend/src/router/index.ts`:
```ts
import { GroupsView, GroupView } from '@/features/groups';
```
```ts
      { path: 'groups', name: 'groups', component: GroupsView },
      { path: 'groups/:name', name: 'group', component: GroupView, props: true },
```

В `frontend/src/layouts/DefaultLayout.vue`:
```ts
const NAVIGATION: NavigationItem[] = [
  { title: 'Главная', icon: 'mdi-home', routeName: 'home' },
  { title: 'Дерево', icon: 'mdi-file-tree', routeName: 'directory' },
  { title: 'Пользователи', icon: 'mdi-account-multiple', routeName: 'users' },
  { title: 'Группы', icon: 'mdi-account-group', routeName: 'groups' },
];
```

В `frontend/src/features/users/views/UserView.vue` заменить строку с `v-chip`:
```vue
        <v-chip v-for="name in details.groups" :key="name" :to="{ name: 'group', params: { name } }" class="mr-2 mt-2">{{ name }}</v-chip>
```

- [ ] **Step 7: Проверить тесты, типы, сборку и сценарии вживую**

Run: `cd frontend && npm run test:run && npm run typecheck && npm run build`
Expected: PASS, без ошибок типов, сборка успешна.

Вживую (вход как alice; пользователь `jdoe` из задачи 17 существует):
- создать группу `team` без участников → ошибка «a group needs at least one member»;
- создать `team` с участником `jdoe` → группа в списке, 1 участник;
- открыть `team` → добавить `alice` → 2 участника; карточка `alice` показывает `admins` и `team`;
- убрать `alice`, затем попытаться убрать `jdoe` → «cannot remove the last member of a group»;
- открыть `jdoe` → «Удалить» → ошибка «user is the only member of groups: team»;
- удалить группу `admins` → «this group is protected».

- [ ] **Step 8: Commit**

```bash
git add frontend/src
git commit -m "Add group management pages

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---
### Task 19: CLAUDE.md, README и итоговая проверка

**Files:**
- Create: `CLAUDE.md`
- Create: `README.md`

**Interfaces:**
- Consumes: всё из задач 1–18.
- Produces: документация проекта; подтверждение критерия успеха из спецификации.

- [ ] **Step 1: Создать `CLAUDE.md`**

`CLAUDE.md`:
````markdown
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
````

- [ ] **Step 2: Создать `README.md`**

`README.md`:
````markdown
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
````

- [ ] **Step 3: Прогнать все проверки**

Run:
```bash
cd backend && make build && make vet && make test && make test-integration
cd ../frontend && npm run test:run && npm run typecheck && npm run build
```
Expected: всё зелёное.

- [ ] **Step 4: Проверить критерий успеха из спецификации**

Запустить бэкенд и фронтенд. В браузере, войдя как `alice`:
1. Открыть «Дерево» — видно `ou=people` и `ou=groups`.
2. В «Пользователях» создать `ivan` (cn `Ivan Petrov`, sn `Petrov`, пароль `ivan-secret`).
3. В «Группах» открыть `admins` и добавить `ivan`.

Затем в терминале:
```bash
docker exec ldap-openldap-1 ldapsearch -x -H ldap://localhost -LLL \
  -D cn=admin,dc=example,dc=com -w admin \
  -b uid=ivan,ou=people,dc=example,dc=com -s base cn memberOf
docker exec ldap-openldap-1 ldapwhoami -x -H ldap://localhost \
  -D uid=ivan,ou=people,dc=example,dc=com -w ivan-secret
```
Expected: `cn: Ivan Petrov`, `memberOf: cn=admins,ou=groups,dc=example,dc=com`, и `ldapwhoami` печатает DN `ivan`.

Выйти и войти как `ivan` / `ivan-secret` — вход проходит (он теперь в `admins`).

Прибрать за собой: выйти, войти снова как `alice` (себя удалить нельзя), в «Группах» → `admins` убрать `ivan`, затем удалить `ivan` в «Пользователях».

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md README.md
git commit -m "Add project documentation

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```
