package ldap_db

import (
	"errors"
	"testing"
)

func TestCommonName(t *testing.T) {
	testCases := []struct {
		caseName  string
		dn        string
		wantName  string
		wantError error
	}{
		{"group in OU", "CN=PanelAdmins,OU=Groups,DC=corp,DC=example,DC=com", "PanelAdmins", nil},
		{"built-in group in CN=Users", "CN=Domain Admins,CN=Users,DC=corp,DC=example,DC=com", "Domain Admins", nil},
		{"escaped comma", `CN=Smith\, John,OU=Groups,DC=corp,DC=example,DC=com`, "Smith, John", nil},
		{"hex-escaped comma", `CN=Smith\2C John,OU=Groups,DC=corp,DC=example,DC=com`, "Smith, John", nil},
		{"escaped backslash", `CN=a\\b,OU=Groups,DC=corp,DC=example,DC=com`, `a\b`, nil},
		{"escaped trailing space", `CN=trailing\ `, "trailing ", nil},
		{"lower-case type", "cn=lower,OU=Groups,DC=corp,DC=example,DC=com", "lower", nil},
		{"multi-valued RDN", "sn=y+cn=x,DC=corp,DC=example,DC=com", "x", nil},
		{"first RDN is not CN", "OU=Groups,DC=corp,DC=example,DC=com", "", ErrNoCommonName},
		{"empty string", "", "", ErrEmptyDNGiven},
		{"space", " ", "", ErrEmptyDNGiven},
		{"no-break space", "\u00a0", "", ErrEmptyDNGiven},
		{"em space", "\u2003", "", ErrEmptyDNGiven},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			name, err := CommonName(testCase.dn)
			if !errors.Is(err, testCase.wantError) {
				t.Fatalf("CommonName(%q) error = %v, want %v", testCase.dn, err, testCase.wantError)
			}
			if name != testCase.wantName {
				t.Fatalf("CommonName(%q) = %q, want %q", testCase.dn, name, testCase.wantName)
			}
		})
	}
}

func TestCommonNameRejectsMalformedDN(t *testing.T) {
	name, err := CommonName("garbage")
	if err == nil {
		t.Fatalf("CommonName(%q) = %q, want parse error", "garbage", name)
	}
}
