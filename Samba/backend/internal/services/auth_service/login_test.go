package auth_service_test

import (
	"errors"
	"testing"

	"samba-admin/internal/domain/session"
	"samba-admin/internal/domain/user"
)

func TestLoginOpensSession(t *testing.T) {
	environment := newTestEnvironment(t, fakeAuthenticator{found: alice}, fakeIDGenerator{id: sessionID})

	opened, err := environment.service.Login("ALICE", "password")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	want := session.Session{
		ID:          sessionID,
		Login:       "alice",
		DisplayName: "Alice Admin",
		ExpiresAt:   startTime.Add(sessionTTL),
	}
	if opened != want {
		t.Fatalf("Login() = %+v, want %+v", opened, want)
	}
	stored, err := environment.store.Find(sessionID)
	if err != nil {
		t.Fatalf("session is not saved: Find() error = %v", err)
	}
	if stored != want {
		t.Fatalf("saved session = %+v, want %+v", stored, want)
	}
}

func TestLoginLetsAdminIn(t *testing.T) {
	testCases := []struct {
		caseName string
		groups   []string
	}{
		{"member of admin group", []string{"Developers", "PanelAdmins"}},
		{"group name in other case", []string{"paneladmins"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			authenticator := fakeAuthenticator{found: user.User{Login: "alice", Groups: testCase.groups}}
			environment := newTestEnvironment(t, authenticator, fakeIDGenerator{id: sessionID})

			opened, err := environment.service.Login("alice", "password")
			if err != nil {
				t.Fatalf("Login() error = %v", err)
			}
			if opened.Login != "alice" {
				t.Fatalf("Login().Login = %q, want alice", opened.Login)
			}
		})
	}
}

func TestLoginRejects(t *testing.T) {
	errUnavailable := errors.New("samba is unavailable")
	errNoRandom := errors.New("no random bytes")
	workingIDs := fakeIDGenerator{id: sessionID}

	testCases := []struct {
		caseName      string
		authenticator fakeAuthenticator
		ids           fakeIDGenerator
		wantError     error
	}{
		{
			"user outside admin group",
			fakeAuthenticator{found: user.User{Login: "bob", Groups: []string{"Developers"}}},
			workingIDs,
			user.ErrNoAdminPrivilege,
		},
		{"user without groups", fakeAuthenticator{found: user.User{Login: "bob"}}, workingIDs, user.ErrNoAdminPrivilege},
		{"wrong password", fakeAuthenticator{err: user.ErrWrongLoginOrPassword}, workingIDs, user.ErrWrongLoginOrPassword},
		// Неизвестный логин и неверный пароль должны давать одну ошибку (один 401),
		// иначе по разнице ответов можно перебирать существующие логины.
		{"unknown login", fakeAuthenticator{err: user.ErrNotFound}, workingIDs, user.ErrWrongLoginOrPassword},
		{"directory unavailable", fakeAuthenticator{err: errUnavailable}, workingIDs, errUnavailable},
		{"ID generator fails", fakeAuthenticator{found: alice}, fakeIDGenerator{err: errNoRandom}, errNoRandom},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			environment := newTestEnvironment(t, testCase.authenticator, testCase.ids)

			opened, err := environment.service.Login("bob", "password")
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("Login() error = %v, want %v", err, testCase.wantError)
			}
			if opened != (session.Session{}) {
				t.Fatalf("Login() returned session %+v together with error", opened)
			}
			if _, err := environment.store.Find(sessionID); !errors.Is(err, session.ErrNotFound) {
				t.Fatalf("failed login left a session in the store: Find() error = %v", err)
			}
		})
	}
}

func TestLoginHidesUnknownLogin(t *testing.T) {
	environment := newTestEnvironment(t, fakeAuthenticator{err: user.ErrNotFound}, fakeIDGenerator{id: sessionID})

	_, err := environment.service.Login("nobody", "password")
	if errors.Is(err, user.ErrNotFound) {
		t.Fatalf("Login() error = %v: ErrNotFound must not leak out of login", err)
	}
}
