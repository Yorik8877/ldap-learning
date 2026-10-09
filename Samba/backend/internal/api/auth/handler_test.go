package auth_test

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"samba-admin/internal/api/auth"
	"samba-admin/internal/domain/session"
	"samba-admin/internal/domain/user"
)

const sessionID = "session-1"

var (
	silentLogger = slog.New(slog.NewTextHandler(io.Discard, nil))
	aliceSession = session.Session{
		ID:          sessionID,
		Login:       "alice",
		DisplayName: "Alice Admin",
		ExpiresAt:   time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC),
	}
)

// fakeService подменяет auth_service: одна живая сессия sessionID, вход — по заданному результату.
type fakeService struct {
	loginErr       error
	currentErr     error
	loggedOutIDs   []string
	receivedLogin  string
	receivedSecret string
}

func (fake *fakeService) Login(login, password string) (session.Session, error) {
	fake.receivedLogin, fake.receivedSecret = login, password
	if fake.loginErr != nil {
		return session.Session{}, fake.loginErr
	}
	return aliceSession, nil
}

func (fake *fakeService) Current(id string) (session.Session, error) {
	if fake.currentErr != nil {
		return session.Session{}, fake.currentErr
	}
	if id != sessionID {
		return session.Session{}, session.ErrNotFound
	}
	return aliceSession, nil
}

func (fake *fakeService) Logout(id string) error {
	fake.loggedOutIDs = append(fake.loggedOutIDs, id)
	return nil
}

func postLogin(handler *auth.Handler, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	handler.Login(recorder, request)
	return recorder
}

// serveProtected прогоняет запрос через RequireSession, как это делает роутер.
func serveProtected(handler *auth.Handler, next http.HandlerFunc, cookie *http.Cookie) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	handler.RequireSession(next).ServeHTTP(recorder, request)
	return recorder
}

func sessionCookie(recorder *httptest.ResponseRecorder) *http.Cookie {
	for _, cookie := range recorder.Result().Cookies() {
		if cookie.Name == "session" {
			return cookie
		}
	}
	return nil
}

func decodeCurrentUser(t *testing.T, recorder *httptest.ResponseRecorder) auth.CurrentUserResponse {
	t.Helper()
	var body auth.CurrentUserResponse
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}

func TestLoginSetsSessionCookieAndReturnsUser(t *testing.T) {
	service := &fakeService{}
	recorder := postLogin(auth.New(silentLogger, service), `{"login":"ALICE","password":"Alice-Secret1"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Login() status = %d, want 200", recorder.Code)
	}
	if service.receivedLogin != "ALICE" || service.receivedSecret != "Alice-Secret1" {
		t.Fatalf("service got %q/%q, want ALICE/Alice-Secret1", service.receivedLogin, service.receivedSecret)
	}
	if body := decodeCurrentUser(t, recorder); body != (auth.CurrentUserResponse{Login: "alice", DisplayName: "Alice Admin"}) {
		t.Fatalf("Login() body = %+v, want alice / Alice Admin", body)
	}

	cookie := sessionCookie(recorder)
	if cookie == nil {
		t.Fatalf("Login() did not set session cookie")
	}
	if cookie.Value != sessionID || cookie.Path != "/" || !cookie.Expires.Equal(aliceSession.ExpiresAt) {
		t.Fatalf("session cookie = %+v, want value %s, path /, expires %s", cookie, sessionID, aliceSession.ExpiresAt)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("session cookie must be HttpOnly and SameSite=Strict, got %+v", cookie)
	}
}

func TestLoginErrors(t *testing.T) {
	testCases := []struct {
		caseName   string
		body       string
		loginErr   error
		wantStatus int
	}{
		{"wrong password", `{"login":"alice","password":"wrong"}`, user.ErrWrongLoginOrPassword, http.StatusUnauthorized},
		{"not an admin", `{"login":"bob","password":"Bob-Secret1"}`, user.ErrNoAdminPrivilege, http.StatusForbidden},
		{"directory failure", `{"login":"alice","password":"Alice-Secret1"}`, errors.New("samba is unavailable"), http.StatusInternalServerError},
		{"broken json", `{"login":`, nil, http.StatusBadRequest},
		{"unknown field", `{"login":"alice","password":"x","extra":1}`, nil, http.StatusBadRequest},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			recorder := postLogin(auth.New(silentLogger, &fakeService{loginErr: testCase.loginErr}), testCase.body)

			if recorder.Code != testCase.wantStatus {
				t.Fatalf("Login() status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
			if sessionCookie(recorder) != nil {
				t.Fatalf("failed Login() set a session cookie")
			}
		})
	}
}

func TestRequireSessionRejects(t *testing.T) {
	testCases := []struct {
		caseName   string
		cookie     *http.Cookie
		currentErr error
		wantStatus int
	}{
		{"no cookie", nil, nil, http.StatusUnauthorized},
		{"unknown session", &http.Cookie{Name: "session", Value: "forged"}, nil, http.StatusUnauthorized},
		{"expired session", &http.Cookie{Name: "session", Value: sessionID}, session.ErrExpired, http.StatusUnauthorized},
		{"store failure", &http.Cookie{Name: "session", Value: sessionID}, errors.New("store is down"), http.StatusInternalServerError},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			reached := false
			next := func(w http.ResponseWriter, r *http.Request) { reached = true }

			handler := auth.New(silentLogger, &fakeService{currentErr: testCase.currentErr})
			recorder := serveProtected(handler, next, testCase.cookie)

			if reached {
				t.Fatalf("RequireSession() let the request through")
			}
			if recorder.Code != testCase.wantStatus {
				t.Fatalf("RequireSession() status = %d, want %d", recorder.Code, testCase.wantStatus)
			}
		})
	}
}

func TestMeReturnsSessionUser(t *testing.T) {
	handler := auth.New(silentLogger, &fakeService{})
	recorder := serveProtected(handler, handler.Me, &http.Cookie{Name: "session", Value: sessionID})

	if recorder.Code != http.StatusOK {
		t.Fatalf("Me() status = %d, want 200", recorder.Code)
	}
	if body := decodeCurrentUser(t, recorder); body != (auth.CurrentUserResponse{Login: "alice", DisplayName: "Alice Admin"}) {
		t.Fatalf("Me() body = %+v, want alice / Alice Admin", body)
	}
}

func TestLogoutClosesSessionAndClearsCookie(t *testing.T) {
	service := &fakeService{}
	handler := auth.New(silentLogger, service)
	recorder := serveProtected(handler, handler.Logout, &http.Cookie{Name: "session", Value: sessionID})

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("Logout() status = %d, want 204", recorder.Code)
	}
	if len(service.loggedOutIDs) != 1 || service.loggedOutIDs[0] != sessionID {
		t.Fatalf("service.Logout got %v, want [%s]", service.loggedOutIDs, sessionID)
	}
	cookie := sessionCookie(recorder)
	if cookie == nil || cookie.MaxAge >= 0 {
		t.Fatalf("Logout() must expire the session cookie, got %+v", cookie)
	}
}

// Ручки Me и Logout рассчитывают на RequireSession. Если маршрут забыли защитить, они отвечают 401, а не паникуют.
func TestMeWithoutRequireSession(t *testing.T) {
	recorder := httptest.NewRecorder()
	auth.New(silentLogger, &fakeService{}).Me(recorder, httptest.NewRequest(http.MethodGet, "/api/auth/me", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("Me() without session status = %d, want 401", recorder.Code)
	}
}
