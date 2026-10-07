package auth_service_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/services/auth_service"
)

const sessionTTL = 8 * time.Hour

var startTime = time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)

type fakePasswords struct {
	err   error
	calls int
}

func (f *fakePasswords) Verify(_ context.Context, _ user.UID, _ string) error {
	f.calls++
	return f.err
}

type fakeAdmins struct {
	members map[user.UID]bool
	err     error
}

func (f *fakeAdmins) IsMember(_ context.Context, _ group.Name, uid user.UID) (bool, error) {
	return f.members[uid], f.err
}

type fakeUsers map[user.UID]user.User

func (f fakeUsers) Get(_ context.Context, uid user.UID) (user.User, error) {
	found, exists := f[uid]
	if !exists {
		return user.User{}, user.ErrNotFound
	}
	return found, nil
}

type fakeSessions map[string]session.Session

func (f fakeSessions) Save(_ context.Context, stored session.Session) error {
	f[stored.ID] = stored
	return nil
}

func (f fakeSessions) Find(_ context.Context, id string) (session.Session, error) {
	found, exists := f[id]
	if !exists {
		return session.Session{}, session.ErrNotFound
	}
	return found, nil
}

func (f fakeSessions) Delete(_ context.Context, id string) error {
	delete(f, id)
	return nil
}

type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

type sequenceIDs struct{ issued int }

func (g *sequenceIDs) NewID() (string, error) {
	g.issued++
	return fmt.Sprintf("session-%d", g.issued), nil
}

type fixture struct {
	service   *auth_service.Service
	passwords *fakePasswords
	admins    *fakeAdmins
	sessions  fakeSessions
	clock     *fakeClock
}

func newFixture() fixture {
	passwords := &fakePasswords{}
	admins := &fakeAdmins{members: map[user.UID]bool{"alice": true}}
	users := fakeUsers{
		"alice": {UID: "alice", CommonName: "Alice Admin"},
		"bob":   {UID: "bob", CommonName: "Bob"},
	}
	sessions := fakeSessions{}
	clock := &fakeClock{now: startTime}
	service := auth_service.New(passwords, admins, users, sessions, clock, &sequenceIDs{}, "admins", sessionTTL)
	return fixture{service: service, passwords: passwords, admins: admins, sessions: sessions, clock: clock}
}

func TestLoginStartsSessionForAdmin(t *testing.T) {
	f := newFixture()

	started, err := f.service.Login(t.Context(), "alice", "alice-secret")

	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	want := session.Session{ID: "session-1", UID: "alice", CommonName: "Alice Admin", ExpiresAt: startTime.Add(sessionTTL)}
	if started != want {
		t.Fatalf("Login() = %+v, want %+v", started, want)
	}
	if _, saved := f.sessions["session-1"]; !saved {
		t.Fatalf("session was not saved")
	}
}

func TestLoginRejectsMalformedInputWithoutDirectory(t *testing.T) {
	cases := []struct {
		name     string
		uid      string
		password string
	}{
		{name: "uid in another case", uid: "Alice", password: "alice-secret"},
		{name: "filter injection", uid: "a*)(uid=*", password: "alice-secret"},
		{name: "empty password", uid: "alice", password: ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			f := newFixture()

			_, err := f.service.Login(t.Context(), testCase.uid, testCase.password)

			if !errors.Is(err, session.ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
			}
			if f.passwords.calls != 0 {
				t.Fatalf("directory was called %d times", f.passwords.calls)
			}
		})
	}
}

func TestLoginWithWrongPassword(t *testing.T) {
	f := newFixture()
	f.passwords.err = session.ErrInvalidCredentials

	_, err := f.service.Login(t.Context(), "alice", "wrong-password")

	if !errors.Is(err, session.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginOfNonAdmin(t *testing.T) {
	f := newFixture()

	_, err := f.service.Login(t.Context(), "bob", "bob-secret")

	if !errors.Is(err, session.ErrNotAdmin) {
		t.Fatalf("Login() error = %v, want ErrNotAdmin", err)
	}
	if len(f.sessions) != 0 {
		t.Fatalf("session saved for a non-admin")
	}
}

func TestLoginPropagatesDirectoryFailure(t *testing.T) {
	f := newFixture()
	f.admins.err = directory.ErrUnavailable

	_, err := f.service.Login(t.Context(), "alice", "alice-secret")

	if !errors.Is(err, directory.ErrUnavailable) {
		t.Fatalf("Login() error = %v, want ErrUnavailable", err)
	}
}

func TestAuthenticate(t *testing.T) {
	f := newFixture()
	started, _ := f.service.Login(t.Context(), "alice", "alice-secret")

	found, err := f.service.Authenticate(t.Context(), started.ID)
	if err != nil || found.UID != "alice" {
		t.Fatalf("Authenticate() = %+v, %v", found, err)
	}

	f.clock.now = started.ExpiresAt
	if _, err := f.service.Authenticate(t.Context(), started.ID); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Authenticate(expired) error = %v, want ErrNotFound", err)
	}
	if _, kept := f.sessions[started.ID]; kept {
		t.Fatalf("expired session was not deleted")
	}
	if _, err := f.service.Authenticate(t.Context(), "missing"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Authenticate(missing) error = %v, want ErrNotFound", err)
	}
}

func TestLogoutDeletesSession(t *testing.T) {
	f := newFixture()
	started, _ := f.service.Login(t.Context(), "alice", "alice-secret")

	if err := f.service.Logout(t.Context(), started.ID); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if len(f.sessions) != 0 {
		t.Fatalf("session is still stored")
	}
}
