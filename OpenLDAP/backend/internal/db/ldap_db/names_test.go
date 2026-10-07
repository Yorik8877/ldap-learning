package ldap_db_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/db/ldap_db"
)

func TestFilterEqualsEscapesFilterSyntax(t *testing.T) {
	got := ldap_db.FilterEquals("uid", "a*)(uid=*")
	want := `(uid=a\2a\29\28uid=\2a)`
	if got != want {
		t.Fatalf("FilterEquals() = %q, want %q", got, want)
	}
}

func TestRDNEscapesDNSyntax(t *testing.T) {
	got := ldap_db.RDN("cn", "Doe, John")
	want := `cn=Doe\, John`
	if got != want {
		t.Fatalf("RDN() = %q, want %q", got, want)
	}
}

func TestIsWithin(t *testing.T) {
	const base = "dc=example,dc=com"
	cases := []struct {
		name string
		dn   string
		want bool
	}{
		{name: "base itself in another case", dn: "DC=Example,DC=Com", want: true},
		{name: "direct child", dn: "ou=people,dc=example,dc=com", want: true},
		{name: "deep descendant", dn: "uid=alice,ou=people,dc=example,dc=com", want: true},
		{name: "other tree", dn: "cn=config", want: false},
		{name: "empty dn", dn: "", want: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ldap_db.IsWithin(testCase.dn, base)
			if err != nil {
				t.Fatalf("IsWithin() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("IsWithin(%q) = %v, want %v", testCase.dn, got, testCase.want)
			}
		})
	}
}

func TestIsWithinRejectsMalformedDN(t *testing.T) {
	_, err := ldap_db.IsWithin("not a dn", "dc=example,dc=com")
	if !errors.Is(err, ldap_db.ErrInvalidDN) {
		t.Fatalf("IsWithin() error = %v, want ErrInvalidDN", err)
	}
}

func TestFirstRDN(t *testing.T) {
	got, err := ldap_db.FirstRDN("uid=alice,ou=people,dc=example,dc=com")
	if err != nil {
		t.Fatalf("FirstRDN() error = %v", err)
	}
	if got != "uid=alice" {
		t.Fatalf("FirstRDN() = %q, want %q", got, "uid=alice")
	}
}

func TestChildValue(t *testing.T) {
	const people = "ou=people,dc=example,dc=com"
	cases := []struct {
		name      string
		dn        string
		wantValue string
		wantFound bool
	}{
		{name: "direct child", dn: "uid=alice,ou=people,dc=example,dc=com", wantValue: "alice", wantFound: true},
		{name: "other attribute", dn: "cn=alice,ou=people,dc=example,dc=com", wantFound: false},
		{name: "outside parent", dn: "cn=admin,dc=example,dc=com", wantFound: false},
		{name: "grandchild", dn: "uid=x,ou=nested,ou=people,dc=example,dc=com", wantFound: false},
		{name: "malformed", dn: "garbage", wantFound: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			value, found := ldap_db.ChildValue(testCase.dn, "uid", people)
			if found != testCase.wantFound || value != testCase.wantValue {
				t.Fatalf("ChildValue() = (%q, %v), want (%q, %v)", value, found, testCase.wantValue, testCase.wantFound)
			}
		})
	}
}
