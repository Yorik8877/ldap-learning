package httpjson_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"samba-admin/internal/api/httpjson"
	"samba-admin/internal/domain/group"
	"samba-admin/internal/domain/session"
	"samba-admin/internal/domain/user"
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

// Доменные ошибки приходят обёрнутыми в цепочку op'ов. Клиент получает статус и код из контракта,
// а в message — только текст самой доменной ошибки, без имён внутренних функций.
func TestWriteErrorMapsDomainErrors(t *testing.T) {
	testCases := []struct {
		caseName   string
		domainErr  error
		wantStatus int
		wantCode   string
	}{
		{"user not found", user.ErrNotFound, http.StatusNotFound, "not_found"},
		{"wrong login or password", user.ErrWrongLoginOrPassword, http.StatusUnauthorized, "unauthorized"},
		{"not an admin", user.ErrNoAdminPrivilege, http.StatusForbidden, "forbidden"},
		{"session not found", session.ErrNotFound, http.StatusUnauthorized, "unauthorized"},
		{"session expired", session.ErrExpired, http.StatusUnauthorized, "unauthorized"},
		{"group not found", group.ErrNotFound, http.StatusNotFound, "not_found"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			wrapped := fmt.Errorf("auth_service.Login: %w", fmt.Errorf("user_repo.Authenticate: %w", testCase.domainErr))

			httpjson.WriteError(recorder, silentLogger, wrapped)

			body := decodeBody(t, recorder)
			if recorder.Code != testCase.wantStatus || body.Code != testCase.wantCode {
				t.Fatalf("WriteError() = %d %+v, want %d %s", recorder.Code, body, testCase.wantStatus, testCase.wantCode)
			}
			if body.Message != testCase.domainErr.Error() {
				t.Fatalf("WriteError() message = %q, want %q", body.Message, testCase.domainErr.Error())
			}
		})
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
