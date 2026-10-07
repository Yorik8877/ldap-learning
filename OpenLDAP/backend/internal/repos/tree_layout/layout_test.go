package tree_layout_test

import (
	"testing"

	"ldap-admin/internal/repos/tree_layout"
)

func TestLayoutBuildsDNs(t *testing.T) {
	layout := tree_layout.New("dc=example,dc=com")

	if got := layout.PeopleDN(); got != "ou=people,dc=example,dc=com" {
		t.Errorf("PeopleDN() = %q", got)
	}
	if got := layout.GroupsDN(); got != "ou=groups,dc=example,dc=com" {
		t.Errorf("GroupsDN() = %q", got)
	}
	if got := layout.UserDN("alice"); got != "uid=alice,ou=people,dc=example,dc=com" {
		t.Errorf("UserDN() = %q", got)
	}
	if got := layout.GroupDN("admins"); got != "cn=admins,ou=groups,dc=example,dc=com" {
		t.Errorf("GroupDN() = %q", got)
	}
}

func TestUIDOf(t *testing.T) {
	layout := tree_layout.New("dc=example,dc=com")

	if uid, found := layout.UIDOf("UID=alice,OU=People,dc=example,dc=com"); !found || uid != "alice" {
		t.Errorf("UIDOf(person) = (%q, %v), want (alice, true)", uid, found)
	}
	if _, found := layout.UIDOf("cn=admin,dc=example,dc=com"); found {
		t.Errorf("UIDOf(cn=admin) found a uid")
	}
	if _, found := layout.UIDOf("uid=Bob,ou=people,dc=example,dc=com"); found {
		t.Errorf("UIDOf(uid=Bob) accepted a uid that breaks domain rules")
	}
}
