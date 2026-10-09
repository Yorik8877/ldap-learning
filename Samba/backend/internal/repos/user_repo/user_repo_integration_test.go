//go:build integration

// Интеграционные тесты репозитория против Samba из Samba/docker-compose.yml и учёток из Samba/seed.sh.
// Запуск: make test-integration. Без запущенной Samba или без certs/ca.pem тесты пропускаются.
package user_repo

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"testing"

	"samba-admin/internal/db/ldap_db"
	"samba-admin/internal/domain/user"
)

const alicePassword = "Alice-Secret1"

var aliceGroups = []string{"PanelAdmins"}

func settingOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func newStandRepo(t *testing.T) *Repo {
	t.Helper()
	caFile := settingOr("LDAP_CA_FILE", "../../../certs/ca.pem")
	client, err := ldap_db.NewClient(
		settingOr("LDAP_URL", "ldaps://localhost:636"),
		caFile,
		settingOr("LDAP_BIND_DN", "CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com"),
		settingOr("LDAP_BIND_PASSWORD", "Svc-Panel-Passw0rd"),
	)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("CA file %s not found, run Samba/seed.sh: %v", caFile, err)
	}
	if errors.Is(err, ldap_db.ErrUnavailable) {
		t.Skipf("Samba is not reachable: %v", err)
	}
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return New(settingOr("LDAP_BASE_DN", "DC=corp,DC=example,DC=com"), client)
}

func TestFindByLogin(t *testing.T) {
	repo := newStandRepo(t)

	testCases := []struct {
		caseName   string
		login      string
		wantLogin  string
		wantGroups []string
		wantError  error
	}{
		{"existing user", "alice", "alice", aliceGroups, nil},
		{"login is case-insensitive", "ALICE", "alice", aliceGroups, nil},
		{"unknown login", "nobody", "", nil, user.ErrNotFound},
		{"filter wildcard is escaped", "*", "", nil, user.ErrNotFound},
		{"service account outside OU=Staff", "svc-panel", "", nil, user.ErrNotFound},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			found, err := repo.FindByLogin(testCase.login)
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("FindByLogin(%q) error = %v, want %v", testCase.login, err, testCase.wantError)
			}
			if found.Login != testCase.wantLogin {
				t.Fatalf("FindByLogin(%q).Login = %q, want %q", testCase.login, found.Login, testCase.wantLogin)
			}
			if !slices.Equal(found.Groups, testCase.wantGroups) {
				t.Fatalf("FindByLogin(%q).Groups = %q, want %q", testCase.login, found.Groups, testCase.wantGroups)
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	repo := newStandRepo(t)

	testCases := []struct {
		caseName   string
		login      string
		password   string
		wantLogin  string
		wantGroups []string
		wantError  error
	}{
		{"correct password", "alice", alicePassword, "alice", aliceGroups, nil},
		{"login is case-insensitive", "ALICE", alicePassword, "alice", aliceGroups, nil},
		{"wrong password", "alice", "wrong-password", "", nil, user.ErrWrongLoginOrPassword},
		{"empty password", "alice", "", "", nil, user.ErrWrongLoginOrPassword},
		{"unknown login", "nobody", "any-password", "", nil, user.ErrNotFound},
		{"service account outside OU=Staff", "svc-panel", "Svc-Panel-Passw0rd", "", nil, user.ErrNotFound},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			authenticated, err := repo.Authenticate(testCase.login, testCase.password)
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("Authenticate(%q) error = %v, want %v", testCase.login, err, testCase.wantError)
			}
			if authenticated.Login != testCase.wantLogin {
				t.Fatalf("Authenticate(%q).Login = %q, want %q", testCase.login, authenticated.Login, testCase.wantLogin)
			}
			if !slices.Equal(authenticated.Groups, testCase.wantGroups) {
				t.Fatalf("Authenticate(%q).Groups = %q, want %q", testCase.login, authenticated.Groups, testCase.wantGroups)
			}
		})
	}
}
