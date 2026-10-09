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
	"samba-admin/internal/domain/session"
	"samba-admin/internal/server"
)

var silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// rejectingAuthService — сервис входа, у которого нет ни одной сессии. Тестам маршрутов неважно,
// что ответит обработчик: важно, что отвечает он, а не роутер.
type rejectingAuthService struct{}

func (rejectingAuthService) Login(_, _ string) (session.Session, error) {
	return session.Session{}, session.ErrNotFound
}

func (rejectingAuthService) Current(_ string) (session.Session, error) {
	return session.Session{}, session.ErrNotFound
}

func (rejectingAuthService) Logout(_ string) error {
	return nil
}

func newTestHandler() http.Handler {
	return server.NewHandler(server.Handlers{
		Auth:   auth.New(silentLogger, rejectingAuthService{}),
		Users:  users.New(silentLogger),
		Groups: groups.New(silentLogger),
	}, silentLogger)
}

func serve(handler http.Handler, route string) *httptest.ResponseRecorder {
	method, path, _ := strings.Cut(route, " ")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	return recorder
}

// Мультиплексор отвечает на незарегистрированный маршрут text/plain, а все
// обработчики проекта отвечают JSON, поэтому JSON 404 от обработчика не считается ошибкой.
func isUnregistered(recorder *httptest.ResponseRecorder) bool {
	isNotFoundOrNotAllowed := recorder.Code == http.StatusNotFound || recorder.Code == http.StatusMethodNotAllowed
	isJSON := strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json")
	return isNotFoundOrNotAllowed && !isJSON
}

// Маршрут считается зарегистрированным, пока 404/405 не ответил сам роутер:
// JSON 404 от обработчика (например, пользователь не найден) — законный ответ.
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
		recorder := serve(handler, route)
		if isUnregistered(recorder) {
			t.Errorf("%s: status = %d, route is not registered", route, recorder.Code)
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
		recorder := serve(handler, route)
		if recorder.Code != want {
			t.Errorf("%s: status = %d, want %d", route, recorder.Code, want)
		}
		if !isUnregistered(recorder) {
			t.Errorf("%s: response must come from the router, got Content-Type %q",
				route, recorder.Header().Get("Content-Type"))
		}
	}
}
