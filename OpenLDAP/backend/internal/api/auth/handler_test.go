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
