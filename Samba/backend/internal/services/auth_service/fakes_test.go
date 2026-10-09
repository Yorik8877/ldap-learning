package auth_service_test

import (
	"testing"
	"time"

	"samba-admin/internal/domain/user"
	"samba-admin/internal/repos/session_repo"
	"samba-admin/internal/services/auth_service"
)

const (
	adminGroup = "PanelAdmins"
	sessionTTL = 8 * time.Hour
	sessionID  = "session-1"
)

var startTime = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

var alice = user.User{Login: "alice", DisplayName: "Alice Admin", Groups: []string{"PanelAdmins"}}

// fakeAuthenticator подменяет репозиторий пользователей: отдаёт заранее заданного пользователя или ошибку.
type fakeAuthenticator struct {
	found user.User
	err   error
}

func (fake fakeAuthenticator) Authenticate(_, _ string) (user.User, error) {
	return fake.found, fake.err
}

type fakeIDGenerator struct {
	id  string
	err error
}

func (fake fakeIDGenerator) NewID() (string, error) {
	return fake.id, fake.err
}

// fakeClock — часы, которые тест двигает руками.
type fakeClock struct {
	now time.Time
}

func (clock *fakeClock) Now() time.Time {
	return clock.now
}

type testEnvironment struct {
	service *auth_service.Service
	store   *session_repo.Repo
	clock   *fakeClock
}

func newTestEnvironment(t *testing.T, authenticator fakeAuthenticator, ids fakeIDGenerator) testEnvironment {
	t.Helper()
	store := session_repo.New()
	clock := &fakeClock{now: startTime}
	return testEnvironment{
		service: auth_service.New(authenticator, store, ids, clock, adminGroup, sessionTTL),
		store:   store,
		clock:   clock,
	}
}

// newLoggedInEnvironment — окружение, в котором alice уже вошла и получила сессию sessionID.
func newLoggedInEnvironment(t *testing.T) testEnvironment {
	t.Helper()
	environment := newTestEnvironment(t, fakeAuthenticator{found: alice}, fakeIDGenerator{id: sessionID})
	if _, err := environment.service.Login("alice", "password"); err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	return environment
}
