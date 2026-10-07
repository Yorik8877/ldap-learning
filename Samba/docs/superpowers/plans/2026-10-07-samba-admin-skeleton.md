# samba-admin: инфраструктура и каркас — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Поднять Samba AD DC в Docker с начальными данными и сделать каркас Go-бэкенда (сервер, маршруты, DTO, ручки-заглушки), в который пользователь сам допишет домен, сервисы, репозитории и слой LDAP.

**Architecture:** `Samba/` — отдельный проект рядом с `OpenLDAP/`. Контейнер собирается из своего Dockerfile на Debian, домен создаётся при первом старте, `seed.sh` заводит данные через `samba-tool`. Бэкенд — луковица как в `OpenLDAP/backend`, но заполнены только внешние слои: `config`, `app`, `server`, `api/*`; каталоги внутренних слоёв пусты.

**Tech Stack:** Debian trixie, Samba 4.22 (`samba-ad-dc`), OpenSSL, Docker Compose; Go (`go 1.25.0` в `go.mod`), stdlib `net/http`, `log/slog`.

**Spec:** `Samba/docs/superpowers/specs/2026-10-07-samba-admin-design.md`

## Global Constraints

- Модуль Go — `samba-admin`, в `go.mod` — `go 1.25.0`. Сторонних зависимостей в каркасе нет.
- Каталоги `internal/{domain,services,repos,db}` содержат только `.gitkeep`: их код пишет пользователь.
- Домен: realm `CORP.EXAMPLE.COM`, NetBIOS `CORP`, корень `DC=corp,DC=example,DC=com`, хост контейнера `dc1`.
- Наружу публикуется только порт `636` (LDAPS). Compose-проект называется `samba`.
- Ошибки API — `{code, message}`; коды — из таблицы спецификации (`invalid_request`, `unauthorized`, `forbidden`, `not_found`, `already_exists`, `password_policy`, `not_implemented`, `unavailable`, `internal`).
- Переменные окружения: `HTTP_ADDR`, `LDAP_URL`, `LDAP_CA_FILE`, `LDAP_BIND_DN`, `LDAP_BIND_PASSWORD`, `LDAP_BASE_DN`, `ADMIN_GROUP`, `SESSION_TTL`.
- Порт бэкенда по умолчанию — `:8081` (спецификация порт не фиксирует; 8080 занят бэкендом OpenLDAP-проекта).
- Стиль кода: без однобуквенных имён и сокращений, guard clauses, комментарии только «почему».

## Review Focus

- Повторный старт контейнера на существующем томе не должен заново создавать домен — `entrypoint.sh` проверяет `sam.ldb`; Task 1 проверяет `down` + `up`.
- `seed.sh`, запущенный сразу после `up`, пока домен ещё создаётся (~1 мин), должен дождаться LDAPS, а не упасть на полпути.
- Занятый на хосте порт 636 ломает `docker compose up` ошибкой привязки — это видно сразу, обходного кода нет.
- `LDAP_CA_FILE` по умолчанию относительный (`certs/ca.pem`) и считается от каталога запуска; `make run` запускает из `backend/`.
- Пустое, битое или слишком большое JSON-тело должно давать 400 `invalid_request`, а не 500 — покрыто тестами `httpjson`.

Все файлы ниже проверены до написания плана: контейнер поднят, `seed.sh` прогнан дважды, LDAPS проверен из Go и `openssl`, Go-код прошёл `go vet` и `go test`.

---

### Task 1: Контейнер Samba AD DC

**Files:**
- Create: `Samba/samba/Dockerfile`
- Create: `Samba/samba/entrypoint.sh` (исполняемый)
- Create: `Samba/docker-compose.yml`

**Interfaces:**
- Produces: сервис compose `samba` (контейнер `samba-samba-1`), LDAPS на `localhost:636`; CA сертификата в контейнере — `/var/lib/samba/private/panel-tls/ca.pem`; пароль администратора домена — переменная `SAMBA_ADMIN_PASSWORD` внутри контейнера.

- [ ] **Step 1: Написать Dockerfile**

```dockerfile
FROM debian:trixie

RUN apt-get update \
 && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
      samba-ad-dc samba-ad-provision ldap-utils openssl \
 && rm -rf /var/lib/apt/lists/* \
 && rm -f /etc/samba/smb.conf

COPY entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
```

- [ ] **Step 2: Написать entrypoint и сделать его исполняемым**

```sh
#!/bin/sh
# Первый старт: выпускает сертификат LDAPS и создаёт домен. Следующие старты: только запуск samba.
set -eu

tls_dir=/var/lib/samba/private/panel-tls

# Сертификат, который Samba выпускает сама, содержит имя хоста только в CN, без subjectAltName.
# Go (crypto/x509) смотрит только на subjectAltName, поэтому выпускаем свой — с localhost.
issue_certificates() {
  mkdir -p "$tls_dir"
  cd "$tls_dir"
  openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -subj "/CN=samba-admin dev CA" -keyout ca.key -out ca.pem 2>/dev/null
  openssl req -newkey rsa:2048 -nodes -subj "/CN=localhost" \
    -keyout key.pem -out server.csr 2>/dev/null
  printf 'subjectAltName=DNS:localhost,IP:127.0.0.1,DNS:%s\n' "$(hostname)" > san.ext
  openssl x509 -req -in server.csr -CA ca.pem -CAkey ca.key -CAcreateserial \
    -days 3650 -extfile san.ext -out cert.pem 2>/dev/null
  chmod 600 key.pem ca.key
  rm -f server.csr san.ext
}

# xattr_tdb: без --privileged контейнер не может писать ACL в расширенные атрибуты файлов,
# поэтому Samba хранит их в своей базе.
provision_domain() {
  samba-tool domain provision \
    --realm="$SAMBA_REALM" --domain="$SAMBA_DOMAIN" \
    --server-role=dc --dns-backend=SAMBA_INTERNAL \
    --adminpass="$SAMBA_ADMIN_PASSWORD" \
    --option="vfs objects = dfs_samba4 acl_xattr xattr_tdb" \
    --option="tls keyfile = $tls_dir/key.pem" \
    --option="tls certfile = $tls_dir/cert.pem" \
    --option="tls cafile = $tls_dir/ca.pem"
}

if [ ! -f /var/lib/samba/private/sam.ldb ]; then
  issue_certificates
  provision_domain
fi
exec samba --foreground --no-process-group
```

Run: `chmod +x Samba/samba/entrypoint.sh`

- [ ] **Step 3: Написать docker-compose.yml**

```yaml
name: samba

services:
  samba:
    build: ./samba
    # Имя хоста зашивается в домен при создании; другое имя после пересоздания контейнера сломает DC.
    hostname: dc1
    environment:
      SAMBA_REALM: CORP.EXAMPLE.COM
      SAMBA_DOMAIN: CORP
      SAMBA_ADMIN_PASSWORD: Admin-Passw0rd
    ports:
      - "636:636"
    volumes:
      - samba-data:/var/lib/samba
      - samba-config:/etc/samba

volumes:
  samba-data:
  samba-config:
```

- [ ] **Step 4: Поднять контейнер и дождаться LDAPS**

Run (из `Samba/`):
```bash
docker compose up -d --build
for attempt in $(seq 1 60); do
  docker compose exec -T samba sh -c 'LDAPTLS_CACERT=/var/lib/samba/private/panel-tls/ca.pem ldapwhoami -x -H ldaps://localhost -D Administrator@corp.example.com -w "$SAMBA_ADMIN_PASSWORD"' 2>/dev/null && break
  sleep 2
done
```
Expected: последней строкой `u:CORP\Administrator`.

- [ ] **Step 5: Проверить, что bind без шифрования запрещён**

Run: `docker compose exec -T samba ldapwhoami -x -H ldap://localhost -D Administrator@corp.example.com -w Admin-Passw0rd`
Expected: `ldap_bind: Strong(er) authentication required (8)`.

- [ ] **Step 6: Проверить сертификат с хоста**

Run: `echo | openssl s_client -connect localhost:636 2>/dev/null | openssl x509 -noout -subject -ext subjectAltName`
Expected: `subject=CN = localhost` и `DNS:localhost, IP Address:127.0.0.1, DNS:dc1`.

- [ ] **Step 7: Проверить, что пересоздание контейнера не создаёт домен заново**

Run:
```bash
docker compose exec -T samba samba-tool ou add OU=ProbeRestart
docker compose down && docker compose up -d
sleep 20
docker compose exec -T samba samba-tool ou list
docker compose exec -T samba samba-tool ou delete OU=ProbeRestart
docker compose logs samba | grep -c 'Applied Domain Update'
```
Expected: в списке OU есть `OU=ProbeRestart`; последняя команда печатает `0` (после пересоздания provision не запускался).

- [ ] **Step 8: Commit**

```bash
git add Samba/samba Samba/docker-compose.yml
git commit -m "Add Samba AD DC container for samba-admin"
```

### Task 2: Начальные данные

**Files:**
- Create: `Samba/seed.sh` (исполняемый)
- Modify: `.gitignore` (корень репозитория) — добавить строку `Samba/backend/certs/`

**Interfaces:**
- Consumes: сервис `samba` из Task 1.
- Produces: `OU=Staff`, `OU=Groups`; `CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com` / `Svc-Panel-Passw0rd` (в `Domain Admins`); группа `CN=PanelAdmins,OU=Groups,...`; пользователь `CN=Alice Admin,OU=Staff,...`, `sAMAccountName: alice`, пароль `Alice-Secret1`, участник `PanelAdmins`; файл `Samba/backend/certs/ca.pem`.

- [ ] **Step 1: Написать seed.sh**

```sh
#!/bin/sh
# Начальные данные домена. Повторный запуск безопасен: существующие объекты пропускаются.
set -eu
cd "$(dirname "$0")"

samba_tool() {
  docker compose exec -T samba samba-tool "$@"
}

wait_until_ready() {
  attempt=0
  until docker compose exec -T samba sh -c \
    'LDAPTLS_CACERT=/var/lib/samba/private/panel-tls/ca.pem ldapwhoami -x -H ldaps://localhost -D "Administrator@corp.example.com" -w "$SAMBA_ADMIN_PASSWORD"' \
    >/dev/null 2>&1; do
    attempt=$((attempt + 1))
    if [ "$attempt" -ge 60 ]; then
      echo "samba is not answering on LDAPS" >&2
      exit 1
    fi
    sleep 2
  done
}

ensure_ou() {
  if samba_tool ou list | grep -qx "$1"; then
    echo "exists: $1"
    return
  fi
  samba_tool ou add "$1"
}

ensure_user() {
  login=$1
  shift
  if samba_tool user show "$login" >/dev/null 2>&1; then
    echo "exists: $login"
    return
  fi
  samba_tool user add "$login" "$@"
  samba_tool user setexpiry "$login" --noexpiry
}

ensure_group() {
  if samba_tool group show "$1" >/dev/null 2>&1; then
    echo "exists: $1"
    return
  fi
  samba_tool group add "$1" --groupou=OU=Groups --description="$2"
}

ensure_member() {
  if samba_tool group listmembers "$1" | grep -qx "$2"; then
    echo "exists: $2 in $1"
    return
  fi
  samba_tool group addmembers "$1" "$2"
}

copy_ca_certificate() {
  mkdir -p backend/certs
  docker compose cp samba:/var/lib/samba/private/panel-tls/ca.pem backend/certs/ca.pem
}

wait_until_ready
ensure_ou OU=Staff
ensure_ou OU=Groups
ensure_user svc-panel 'Svc-Panel-Passw0rd' --description="samba-admin service account"
ensure_member "Domain Admins" svc-panel
ensure_group PanelAdmins "samba-admin administrators"
ensure_user alice 'Alice-Secret1' --userou=OU=Staff --given-name=Alice --surname=Admin \
  --mail-address=alice@corp.example.com
ensure_member PanelAdmins alice
copy_ca_certificate
```

Run: `chmod +x Samba/seed.sh`

- [ ] **Step 2: Исключить сертификат из git**

Добавить в конец корневого `.gitignore` строку:
```
Samba/backend/certs/
```

- [ ] **Step 3: Прогнать seed дважды**

Run: `Samba/seed.sh && Samba/seed.sh`
Expected: первый прогон печатает `Added ou ...`, `User 'svc-panel' added successfully`, `Added group PanelAdmins`, `User 'alice' added successfully`, `Added members to group PanelAdmins`; второй — только строки `exists: ...`. Оба копируют `ca.pem`.

- [ ] **Step 4: Проверить CA и учётки**

Run (из `Samba/`):
```bash
echo | openssl s_client -connect localhost:636 -CAfile backend/certs/ca.pem -verify_hostname localhost 2>/dev/null | grep 'Verify return code'
docker compose exec -T samba sh -c 'export LDAPTLS_CACERT=/var/lib/samba/private/panel-tls/ca.pem
  ldapwhoami -x -H ldaps://localhost -D "CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com" -w Svc-Panel-Passw0rd
  ldapwhoami -x -H ldaps://localhost -D alice@corp.example.com -w Alice-Secret1'
git check-ignore -v backend/certs/ca.pem
```
Expected: `Verify return code: 0 (ok)`, `u:CORP\svc-panel`, `u:CORP\alice`, строка `.gitignore` с `Samba/backend/certs/`.

- [ ] **Step 5: Commit**

```bash
git add Samba/seed.sh .gitignore
git commit -m "Add Samba seed script with service account and PanelAdmins"
```

### Task 3: Модуль бэкенда и конфигурация

**Files:**
- Create: `Samba/backend/go.mod`
- Create: `Samba/backend/internal/config/config_test.go`
- Create: `Samba/backend/internal/config/config.go`
- Create: `Samba/backend/Makefile`
- Create: `Samba/backend/.env.example`
- Create: `Samba/backend/internal/{domain,services,repos,db}/.gitkeep`

**Interfaces:**
- Produces: `config.Load(lookup func(string) (string, bool)) (config.Config, error)`; поля `Config`: `HTTPAddress`, `LDAPURL`, `LDAPCAFile`, `BindDN`, `BindPassword`, `BaseDN`, `AdminGroup string`, `SessionTTL time.Duration`.

- [ ] **Step 1: Создать go.mod**

```
module samba-admin

go 1.25.0
```

- [ ] **Step 2: Написать падающий тест конфигурации**

```go
package config_test

import (
	"testing"
	"time"

	"samba-admin/internal/config"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	loaded, err := config.Load(lookupFrom(map[string]string{"LDAP_BIND_PASSWORD": "secret"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := config.Config{
		HTTPAddress:  ":8081",
		LDAPURL:      "ldaps://localhost:636",
		LDAPCAFile:   "certs/ca.pem",
		BindDN:       "CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com",
		BindPassword: "secret",
		BaseDN:       "DC=corp,DC=example,DC=com",
		AdminGroup:   "PanelAdmins",
		SessionTTL:   8 * time.Hour,
	}
	if loaded != want {
		t.Fatalf("Load() = %+v, want %+v", loaded, want)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	loaded, err := config.Load(lookupFrom(map[string]string{
		"LDAP_BIND_PASSWORD": "secret",
		"HTTP_ADDR":          ":9090",
		"ADMIN_GROUP":        "Operators",
		"SESSION_TTL":        "30m",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.HTTPAddress != ":9090" || loaded.AdminGroup != "Operators" || loaded.SessionTTL != 30*time.Minute {
		t.Fatalf("Load() = %+v, overrides not applied", loaded)
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
			"LDAP_BIND_PASSWORD": "secret",
			"SESSION_TTL":        ttl,
		}))
		if err == nil {
			t.Errorf("Load() accepted session TTL %q", ttl)
		}
	}
}
```

- [ ] **Step 3: Убедиться, что тест падает**

Run (из `Samba/backend`): `go test ./internal/config/`
Expected: FAIL — `no non-test Go files` / `undefined: config.Load`.

- [ ] **Step 4: Написать config.go**

```go
// Package config читает настройки из переменных окружения.
// Конфиг грузится один раз в composition root и дальше передаётся параметрами.
package config

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	HTTPAddress  string
	LDAPURL      string
	LDAPCAFile   string
	BindDN       string
	BindPassword string
	BaseDN       string
	AdminGroup   string
	SessionTTL   time.Duration
}

func Load(lookup func(string) (string, bool)) (Config, error) {
	bindPassword, _ := lookup("LDAP_BIND_PASSWORD")
	if bindPassword == "" {
		return Config{}, errors.New("LDAP_BIND_PASSWORD is required")
	}
	rawTTL := valueOr(lookup, "SESSION_TTL", "8h")
	sessionTTL, err := time.ParseDuration(rawTTL)
	if err != nil || sessionTTL <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL must be a positive duration, got %q", rawTTL)
	}
	return Config{
		HTTPAddress:  valueOr(lookup, "HTTP_ADDR", ":8081"),
		LDAPURL:      valueOr(lookup, "LDAP_URL", "ldaps://localhost:636"),
		LDAPCAFile:   valueOr(lookup, "LDAP_CA_FILE", "certs/ca.pem"),
		BindDN:       valueOr(lookup, "LDAP_BIND_DN", "CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com"),
		BindPassword: bindPassword,
		BaseDN:       valueOr(lookup, "LDAP_BASE_DN", "DC=corp,DC=example,DC=com"),
		AdminGroup:   valueOr(lookup, "ADMIN_GROUP", "PanelAdmins"),
		SessionTTL:   sessionTTL,
	}, nil
}

func valueOr(lookup func(string) (string, bool), name, fallback string) string {
	if value, present := lookup(name); present && value != "" {
		return value
	}
	return fallback
}
```

- [ ] **Step 5: Убедиться, что тест проходит**

Run: `go test ./internal/config/`
Expected: `ok  	samba-admin/internal/config`.

- [ ] **Step 6: Makefile, .env.example и пустые каталоги слоёв**

`Samba/backend/Makefile` (отступы — табы):

```makefile
.PHONY: build vet test run

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/samba-admin
```

`Samba/backend/.env.example`:

```
HTTP_ADDR=:8081
LDAP_URL=ldaps://localhost:636
LDAP_CA_FILE=certs/ca.pem
LDAP_BIND_DN=CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com
LDAP_BIND_PASSWORD=Svc-Panel-Passw0rd
LDAP_BASE_DN=DC=corp,DC=example,DC=com
ADMIN_GROUP=PanelAdmins
SESSION_TTL=8h
```

Run (из `Samba/backend`): `touch internal/domain/.gitkeep internal/services/.gitkeep internal/repos/.gitkeep internal/db/.gitkeep` (каталоги создать заранее: `mkdir -p internal/{domain,services,repos,db}`).

- [ ] **Step 7: Commit**

```bash
git add Samba/backend
git commit -m "Add samba-admin backend module and config"
```

### Task 4: JSON-помощники и перевод ошибок

**Files:**
- Create: `Samba/backend/internal/api/httpjson/httpjson_test.go`
- Create: `Samba/backend/internal/api/httpjson/httpjson.go`

**Interfaces:**
- Produces: `httpjson.Write(w, status int, body any)`; `httpjson.Decode(w, r, target any) error` (ошибка оборачивает `httpjson.ErrMalformedBody`); `httpjson.WriteError(w, logger *slog.Logger, err error)`; `httpjson.NotImplemented(w)`; тип `httpjson.ErrorBody{Code, Message string}`.

- [ ] **Step 1: Написать падающий тест**

```go
package httpjson_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"samba-admin/internal/api/httpjson"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

type loginPayload struct {
	Login string `json:"login"`
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) httpjson.ErrorBody {
	t.Helper()
	var body httpjson.ErrorBody
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestDecodeReadsValidBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"login":"alice"}`))
	var target loginPayload

	if err := httpjson.Decode(httptest.NewRecorder(), request, &target); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if target.Login != "alice" {
		t.Fatalf("Decode() login = %q, want alice", target.Login)
	}
}

func TestDecodeRejectsBadBodies(t *testing.T) {
	oversized := `{"login":"` + strings.Repeat("a", 2<<20) + `"}`
	cases := map[string]string{
		"unknown field": `{"login":"alice","extra":1}`,
		"broken json":   `{"login":`,
		"too large":     oversized,
		"empty body":    ``,
	}
	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
			var target loginPayload
			err := httpjson.Decode(httptest.NewRecorder(), request, &target)
			if !errors.Is(err, httpjson.ErrMalformedBody) {
				t.Fatalf("Decode() error = %v, want ErrMalformedBody", err)
			}
		})
	}
}

func TestWriteErrorMapsMalformedBodyTo400(t *testing.T) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"login":`))
	decodeErr := httpjson.Decode(recorder, request, &loginPayload{})

	httpjson.WriteError(recorder, silentLogger, decodeErr)

	body := decodeBody(t, recorder)
	if recorder.Code != http.StatusBadRequest || body.Code != "invalid_request" {
		t.Fatalf("WriteError() = %d %+v, want 400 invalid_request", recorder.Code, body)
	}
}

func TestWriteErrorHidesUnknownErrorText(t *testing.T) {
	recorder := httptest.NewRecorder()

	httpjson.WriteError(recorder, silentLogger, errors.New("ldap: dial tcp 127.0.0.1:636: secret details"))

	body := decodeBody(t, recorder)
	if recorder.Code != http.StatusInternalServerError || body.Code != "internal" {
		t.Fatalf("WriteError() = %d %+v, want 500 internal", recorder.Code, body)
	}
	if strings.Contains(body.Message, "secret details") {
		t.Fatalf("WriteError() leaked error text: %q", body.Message)
	}
}

func TestNotImplementedWrites501(t *testing.T) {
	recorder := httptest.NewRecorder()

	httpjson.NotImplemented(recorder)

	body := decodeBody(t, recorder)
	if recorder.Code != http.StatusNotImplemented || body.Code != "not_implemented" {
		t.Fatalf("NotImplemented() = %d %+v, want 501 not_implemented", recorder.Code, body)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/api/httpjson/`
Expected: FAIL — `undefined: httpjson.Decode` и т. п.

- [ ] **Step 3: Написать httpjson.go**

```go
// Package httpjson читает и пишет JSON и переводит доменные ошибки в HTTP-ответы.
package httpjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

const maxBodyBytes = 1 << 20

var ErrMalformedBody = errors.New("malformed request body")

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorRule struct {
	target error
	status int
	code   string
	// detailed — в ответ уходит полный текст ошибки, а не только текст sentinel.
	// Только для ошибок ввода: в их тексте нет деталей драйвера LDAP.
	detailed bool
}

// errorRules — таблица «доменная ошибка → ответ». Новая доменная ошибка — новая строка.
// Ошибка не из таблицы становится 500 internal: её текст пишется в лог, клиенту не уходит.
var errorRules = []errorRule{
	{target: ErrMalformedBody, status: http.StatusBadRequest, code: "invalid_request", detailed: true},
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
	for _, rule := range errorRules {
		if !errors.Is(err, rule.target) {
			continue
		}
		if rule.status >= http.StatusInternalServerError {
			logger.Error("request failed", "error", err)
		}
		Write(w, rule.status, ErrorBody{Code: rule.code, Message: messageFor(rule, err)})
		return
	}
	logger.Error("unexpected error", "error", err)
	Write(w, http.StatusInternalServerError, ErrorBody{Code: "internal", Message: "internal server error"})
}

func NotImplemented(w http.ResponseWriter) {
	Write(w, http.StatusNotImplemented, ErrorBody{Code: "not_implemented", Message: "not implemented yet"})
}

func messageFor(rule errorRule, err error) string {
	if rule.detailed {
		return err.Error()
	}
	return rule.target.Error()
}
```

- [ ] **Step 4: Убедиться, что тест проходит**

Run: `go test ./internal/api/httpjson/`
Expected: `ok  	samba-admin/internal/api/httpjson`.

- [ ] **Step 5: Commit**

```bash
git add Samba/backend/internal/api/httpjson
git commit -m "Add JSON helpers and error mapping for samba-admin"
```

### Task 5: Ручки-заглушки, маршруты и запуск

**Files:**
- Create: `Samba/backend/internal/server/routes_test.go`
- Create: `Samba/backend/internal/api/auth/{dto.go,handler.go}`
- Create: `Samba/backend/internal/api/users/{dto.go,handler.go}`
- Create: `Samba/backend/internal/api/groups/{dto.go,handler.go}`
- Create: `Samba/backend/internal/server/{routes.go,server.go}`
- Create: `Samba/backend/internal/app/app.go`
- Create: `Samba/backend/cmd/samba-admin/main.go`

**Interfaces:**
- Consumes: `httpjson.NotImplemented` (Task 4), `config.Load` (Task 3).
- Produces: `auth.New(*slog.Logger) *auth.Handler` с методами `Login`, `Logout`, `Me`, `RequireSession(http.Handler) http.Handler`; `users.New` с `List`, `Create`, `Get`, `Update`, `SetPassword`, `SetEnabled`, `Delete`; `groups.New` с `List`, `Create`, `Get`, `Delete`, `AddMember`, `RemoveMember`; `server.NewHandler(server.Handlers, *slog.Logger) http.Handler`; `server.Run(ctx, address, handler, logger) error`; `app.Main()`. DTO — типы в `dto.go` каждого пакета.

- [ ] **Step 1: Написать падающий тест маршрутов**

```go
package server_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"samba-admin/internal/api/auth"
	"samba-admin/internal/api/groups"
	"samba-admin/internal/api/users"
	"samba-admin/internal/server"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

func newTestHandler() http.Handler {
	return server.NewHandler(server.Handlers{
		Auth:   auth.New(silentLogger),
		Users:  users.New(silentLogger),
		Groups: groups.New(silentLogger),
	}, silentLogger)
}

func serve(handler http.Handler, route string) int {
	method, path, _ := strings.Cut(route, " ")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder.Code
}

// Проверяется только, что маршрут зарегистрирован: тест остаётся верным,
// когда заглушки заменяются реализацией.
func TestEveryContractRouteIsRegistered(t *testing.T) {
	handler := newTestHandler()
	contract := []string{
		"POST /api/auth/login", "POST /api/auth/logout", "GET /api/auth/me",
		"GET /api/users", "POST /api/users", "GET /api/users/alice", "PUT /api/users/alice",
		"PUT /api/users/alice/password", "PUT /api/users/alice/enabled", "DELETE /api/users/alice",
		"GET /api/groups", "POST /api/groups", "GET /api/groups/PanelAdmins", "DELETE /api/groups/PanelAdmins",
		"PUT /api/groups/PanelAdmins/members/alice", "DELETE /api/groups/PanelAdmins/members/alice",
	}
	for _, route := range contract {
		status := serve(handler, route)
		if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
			t.Errorf("%s: status = %d, route is not registered", route, status)
		}
	}
}

func TestUnknownRoutesAreRejected(t *testing.T) {
	handler := newTestHandler()
	cases := map[string]int{
		"GET /api/unknown":        http.StatusNotFound,
		"PATCH /api/users/alice":  http.StatusMethodNotAllowed,
		"GET /api/groups/a/b/c/d": http.StatusNotFound,
	}
	for route, want := range cases {
		if status := serve(handler, route); status != want {
			t.Errorf("%s: status = %d, want %d", route, status, want)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/server/`
Expected: FAIL — пакеты `samba-admin/internal/api/auth` и др. не найдены.

- [ ] **Step 3: Пакет auth**

`internal/api/auth/dto.go`:

```go
package auth

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type CurrentUserResponse struct {
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
}
```

`internal/api/auth/handler.go`:

```go
// Package auth — вход, выход и проверка сессии.
package auth

import (
	"log/slog"
	"net/http"

	"samba-admin/internal/api/httpjson"
)

type Handler struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

// Login: POST /api/auth/login, тело LoginRequest → 200 CurrentUserResponse + cookie сессии.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Logout: POST /api/auth/logout → 204.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Me: GET /api/auth/me → 200 CurrentUserResponse.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// RequireSession пропускает запрос дальше только с действующей сессией, иначе 401 unauthorized.
// TODO: сейчас пропускает всех.
func (h *Handler) RequireSession(next http.Handler) http.Handler {
	return next
}
```

- [ ] **Step 4: Пакет users**

`internal/api/users/dto.go`:

```go
package users

type CreateRequest struct {
	Login       string `json:"login"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type UpdateRequest struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type PasswordRequest struct {
	Password string `json:"password"`
}

// EnabledRequest.Enabled — указатель, чтобы отличить «поле не передано» от false.
type EnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

type SummaryResponse struct {
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Enabled     bool   `json:"enabled"`
}

type DetailsResponse struct {
	Login       string   `json:"login"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Enabled     bool     `json:"enabled"`
	Groups      []string `json:"groups"`
}
```

`internal/api/users/handler.go`:

```go
// Package users — управление пользователями домена. {login} в пути — sAMAccountName.
package users

import (
	"log/slog"
	"net/http"

	"samba-admin/internal/api/httpjson"
)

type Handler struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

// List: GET /api/users → 200 []SummaryResponse.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Create: POST /api/users, тело CreateRequest → 201 DetailsResponse.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Get: GET /api/users/{login} → 200 DetailsResponse.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Update: PUT /api/users/{login}, тело UpdateRequest → 200 DetailsResponse.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// SetPassword: PUT /api/users/{login}/password, тело PasswordRequest → 204.
func (h *Handler) SetPassword(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// SetEnabled: PUT /api/users/{login}/enabled, тело EnabledRequest → 204.
func (h *Handler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Delete: DELETE /api/users/{login} → 204.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}
```

- [ ] **Step 5: Пакет groups**

`internal/api/groups/dto.go`:

```go
package groups

type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SummaryResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}

type MemberResponse struct {
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
}

type DetailsResponse struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Members     []MemberResponse `json:"members"`
}
```

`internal/api/groups/handler.go`:

```go
// Package groups — управление группами безопасности. {name} в пути — cn группы, {login} — sAMAccountName.
package groups

import (
	"log/slog"
	"net/http"

	"samba-admin/internal/api/httpjson"
)

type Handler struct {
	logger *slog.Logger
}

func New(logger *slog.Logger) *Handler {
	return &Handler{logger: logger}
}

// List: GET /api/groups → 200 []SummaryResponse.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Create: POST /api/groups, тело CreateRequest → 201 DetailsResponse.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Get: GET /api/groups/{name} → 200 DetailsResponse.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// Delete: DELETE /api/groups/{name} → 204.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// AddMember: PUT /api/groups/{name}/members/{login} → 204.
func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}

// RemoveMember: DELETE /api/groups/{name}/members/{login} → 204.
func (h *Handler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	httpjson.NotImplemented(w)
}
```

- [ ] **Step 6: Сервер и маршруты**

`internal/server/server.go`:

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

`internal/server/routes.go`:

```go
package server

import (
	"log/slog"
	"net/http"
	"time"

	"samba-admin/internal/api/auth"
	"samba-admin/internal/api/groups"
	"samba-admin/internal/api/users"
)

type Handlers struct {
	Auth   *auth.Handler
	Users  *users.Handler
	Groups *groups.Handler
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
		{"GET /api/users/{login}", handlers.Users.Get},
		{"PUT /api/users/{login}", handlers.Users.Update},
		{"PUT /api/users/{login}/password", handlers.Users.SetPassword},
		{"PUT /api/users/{login}/enabled", handlers.Users.SetEnabled},
		{"DELETE /api/users/{login}", handlers.Users.Delete},
		{"GET /api/groups", handlers.Groups.List},
		{"POST /api/groups", handlers.Groups.Create},
		{"GET /api/groups/{name}", handlers.Groups.Get},
		{"DELETE /api/groups/{name}", handlers.Groups.Delete},
		{"PUT /api/groups/{name}/members/{login}", handlers.Groups.AddMember},
		{"DELETE /api/groups/{name}/members/{login}", handlers.Groups.RemoveMember},
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

- [ ] **Step 7: Убедиться, что тест маршрутов проходит**

Run: `go test ./internal/server/`
Expected: `ok  	samba-admin/internal/server`.

- [ ] **Step 8: Composition root и main**

`internal/app/app.go`:

```go
// Package app — composition root: читает конфиг и собирает слои.
// Здесь же подключаются транспорт LDAP → репозитории → сервисы → хендлеры.
package app

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"samba-admin/internal/api/auth"
	"samba-admin/internal/api/groups"
	"samba-admin/internal/api/users"
	"samba-admin/internal/config"
	"samba-admin/internal/server"
)

func Main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("samba-admin stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	loaded, err := config.Load(os.LookupEnv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	handler := server.NewHandler(server.Handlers{
		Auth:   auth.New(logger),
		Users:  users.New(logger),
		Groups: groups.New(logger),
	}, logger)
	return server.Run(ctx, loaded.HTTPAddress, handler, logger)
}
```

`cmd/samba-admin/main.go`:

```go
package main

import "samba-admin/internal/app"

func main() {
	app.Main()
}
```

- [ ] **Step 9: Полная проверка и пробный запуск**

Run (из `Samba/backend`):
```bash
gofmt -l . && make build vet test
cp .env.example .env
make run &
sleep 3
curl -s -i http://localhost:8081/api/users | sed -n '1p;$p'
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8081/api/unknown
kill %1
```
Expected: `gofmt` ничего не печатает; тесты `ok`; `HTTP/1.1 501 Not Implemented` и `{"code":"not_implemented","message":"not implemented yet"}`; `404`. `make run` порождает дочерний процесс `go run` — если `kill %1` его не остановил, найти по порту: `ss -ltnp | grep 8081`.

- [ ] **Step 10: Commit**

```bash
git add Samba/backend
git commit -m "Add samba-admin HTTP skeleton with stub handlers"
```

`backend/.env` в git не попадает — его исключает `.env` в корневом `.gitignore`.

### Task 6: Документация

**Files:**
- Create: `Samba/CLAUDE.md`
- Create: `Samba/README.md`

- [ ] **Step 1: Написать CLAUDE.md**

Факты в разделе «Особенности Samba» получены на прототипе контейнера при подготовке плана; при выполнении — сверить ключевые на поднятом контейнере (Step 3).

````markdown
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
- Наружу опубликован только 636. OpenLDAP-проект занимает 389 — оба работают одновременно.
- Сертификат LDAPS выпускает `samba/entrypoint.sh` (свой CA и SAN `localhost`): сертификат,
  который выпускает сама Samba, без SAN, и Go его не примет.
- Контейнер работает без `--privileged`: ACL файлов Samba хранит в своей базе (`xattr_tdb`).
- `svc-panel` — участник `Domain Admins`. Это упрощение: в реальном домене сервисному аккаунту
  делегируют права только на нужные OU.
- Пароли `svc-panel` и `alice` не истекают (`samba-tool user setexpiry --noexpiry`), иначе политика
  просрочит их через 42 дня.
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
````

- [ ] **Step 2: Написать README.md**

````markdown
# samba-admin

Учебная админ-панель для Samba AD DC: бэкенд на Go, фронтенд на Vue (позже).
Каркас бэкенда готов, ручки отвечают `501`; домен, сервисы, репозитории и работу с LDAP
пишет автор проекта.

## Запуск

```bash
docker compose up -d --build   # Samba AD DC; LDAPS на localhost:636
./seed.sh                      # начальные данные и CA для бэкенда

cd backend
cp .env.example .env
make run                       # http://localhost:8081
```

Первый старт контейнера занимает около минуты: создаётся домен `corp.example.com`.
`seed.sh` дожидается готовности сам, повторный запуск безопасен.

Вход в админку: `alice` / `Alice-Secret1`.

## Проверка

```bash
curl -i http://localhost:8081/api/users      # 501 {"code":"not_implemented",...}
cd backend && make test
```

## Что почитать

- `CLAUDE.md` — команды, правила слоёв, проверенные особенности Samba.
- `docs/superpowers/specs/2026-10-07-samba-admin-design.md` — спецификация и контракт API.
````

- [ ] **Step 3: Сверить ключевые факты на контейнере**

Run (из `Samba/`):
```bash
docker compose exec -T samba sh -c 'export LDAPTLS_CACERT=/var/lib/samba/private/panel-tls/ca.pem
S="-x -H ldaps://localhost -D svc-panel@corp.example.com -w Svc-Panel-Passw0rd"
ldappasswd $S -s New-Passw0rd1 "CN=Alice Admin,OU=Staff,DC=corp,DC=example,DC=com" 2>&1 | tail -1
printf "dn: CN=probe,OU=Staff,DC=corp,DC=example,DC=com\nobjectClass: user\nsAMAccountName: probe\n" | ldapadd $S >/dev/null
ldapsearch $S -LLL -b CN=probe,OU=Staff,DC=corp,DC=example,DC=com -s base userAccountControl primaryGroupID
ldapdelete $S CN=probe,OU=Staff,DC=corp,DC=example,DC=com'
```
Expected: `Additional info: Extended Operation(1.3.6.1.4.1.4203.1.11.1) not supported`; `userAccountControl: 546`, `primaryGroupID: 513`. Пароль alice не меняется.

- [ ] **Step 4: Commit**

```bash
git add Samba/CLAUDE.md Samba/README.md
git commit -m "Add samba-admin documentation with verified Samba behaviours"
```
