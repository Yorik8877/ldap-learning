package auth_service_test

import (
	"errors"
	"testing"

	"samba-admin/internal/domain/user"
	"samba-admin/internal/services/auth_service"
)

const adminGroup = "PanelAdmins"

// fakeAuthenticator подменяет репозиторий: отдаёт заранее заданного пользователя или ошибку.
type fakeAuthenticator struct {
	found user.User
	err   error
}

func (fake fakeAuthenticator) Authenticate(_, _ string) (user.User, error) {
	return fake.found, fake.err
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

			loggedIn, err := auth_service.New(authenticator, adminGroup).Login("alice", "password")
			if err != nil {
				t.Fatalf("Login() error = %v", err)
			}
			if loggedIn.Login != "alice" {
				t.Fatalf("Login().Login = %q, want alice", loggedIn.Login)
			}
		})
	}
}

func TestLoginRejects(t *testing.T) {
	errUnavailable := errors.New("samba is unavailable")

	testCases := []struct {
		caseName      string
		authenticator fakeAuthenticator
		wantError     error
	}{
		{
			"user outside admin group",
			fakeAuthenticator{found: user.User{Login: "bob", Groups: []string{"Developers"}}},
			user.ErrNoAdminPrivilege,
		},
		{"user without groups", fakeAuthenticator{found: user.User{Login: "bob"}}, user.ErrNoAdminPrivilege},
		{"wrong password", fakeAuthenticator{err: user.ErrWrongLoginOrPassword}, user.ErrWrongLoginOrPassword},
		// Неизвестный логин и неверный пароль должны давать одну ошибку (один 401),
		// иначе по разнице ответов можно перебирать существующие логины.
		{"unknown login", fakeAuthenticator{err: user.ErrNotFound}, user.ErrWrongLoginOrPassword},
		{"directory unavailable", fakeAuthenticator{err: errUnavailable}, errUnavailable},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			loggedIn, err := auth_service.New(testCase.authenticator, adminGroup).Login("bob", "password")
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("Login() error = %v, want %v", err, testCase.wantError)
			}
			if loggedIn.Login != "" {
				t.Fatalf("Login() returned user %q together with error", loggedIn.Login)
			}
		})
	}
}

func TestLoginHidesUnknownLogin(t *testing.T) {
	_, err := auth_service.New(fakeAuthenticator{err: user.ErrNotFound}, adminGroup).Login("nobody", "password")
	if errors.Is(err, user.ErrNotFound) {
		t.Fatalf("Login() error = %v: ErrNotFound must not leak out of login", err)
	}
}
