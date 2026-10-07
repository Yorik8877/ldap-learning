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
