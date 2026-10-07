package ldap_db_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/db/ldap_db"
)

func TestUnreachableServerIsUnavailable(t *testing.T) {
	client := ldap_db.New(ldap_db.Config{URL: "ldap://127.0.0.1:1", BindDN: "cn=admin", BindPassword: "secret"})

	_, err := client.Search(t.Context(), ldap_db.SearchRequest{BaseDN: "dc=example,dc=com", Scope: ldap_db.ScopeBase})

	if !errors.Is(err, ldap_db.ErrUnavailable) {
		t.Fatalf("Search() error = %v, want ErrUnavailable", err)
	}
}

func TestVerifyPasswordRejectsEmptyPasswordWithoutNetwork(t *testing.T) {
	client := ldap_db.New(ldap_db.Config{URL: "ldap://127.0.0.1:1"})

	err := client.VerifyPassword(t.Context(), "uid=alice,ou=people,dc=example,dc=com", "")

	if !errors.Is(err, ldap_db.ErrInvalidCredentials) {
		t.Fatalf("VerifyPassword() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestEntryValuesIgnoresAttributeNameCase(t *testing.T) {
	entry := ldap_db.Entry{Attributes: map[string][]string{"memberOf": {"cn=admins"}}}

	if got := entry.First("memberof"); got != "cn=admins" {
		t.Fatalf("First() = %q, want %q", got, "cn=admins")
	}
	if got := entry.Values("missing"); got != nil {
		t.Fatalf("Values() = %v, want nil", got)
	}
}
