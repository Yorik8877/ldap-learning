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
