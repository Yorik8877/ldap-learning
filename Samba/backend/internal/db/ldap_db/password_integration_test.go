//go:build integration

package ldap_db

import (
	"errors"
	"testing"
)

// Учётная запись из Samba/seed.sh.
const (
	aliceDN       = "CN=Alice Admin,OU=Staff,DC=corp,DC=example,DC=com"
	alicePassword = "Alice-Secret1"
)

func TestVerifyPassword(t *testing.T) {
	client := newStandClient(t)

	testCases := []struct {
		caseName  string
		bindDN    string
		password  string
		wantError error
	}{
		{"correct password", aliceDN, alicePassword, nil},
		{"wrong password", aliceDN, "wrong-password", ErrInvalidCredentials},
		{"empty password", aliceDN, "", ErrInvalidCredentials},
		{"empty DN", "", alicePassword, ErrInvalidCredentials},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			err := client.VerifyPassword(testCase.bindDN, testCase.password)
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("VerifyPassword(%q) error = %v, want %v", testCase.bindDN, err, testCase.wantError)
			}
		})
	}
}

// Bind меняет пользователя соединения, поэтому проверка пароля идёт на отдельном соединении.
// Если бы она шла на общем, дальше все запросы выполнялись бы от имени alice.
func TestVerifyPasswordKeepsSharedConnectionIdentity(t *testing.T) {
	client := newStandClient(t)

	if err := client.VerifyPassword(aliceDN, alicePassword); err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}

	shared, err := client.connection()
	if err != nil {
		t.Fatalf("connection() error = %v", err)
	}
	if identity := authorizationID(t, shared); identity != `u:CORP\svc-panel` {
		t.Fatalf("shared connection works as %q, want u:CORP\\svc-panel", identity)
	}
}
