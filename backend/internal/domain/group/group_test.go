package group_test

import (
	"errors"
	"strings"
	"testing"

	"ldap-admin/internal/domain/group"
)

func TestParseName(t *testing.T) {
	if _, err := group.ParseName("dev-team"); err != nil {
		t.Fatalf("ParseName(dev-team) error = %v", err)
	}
	for _, raw := range []string{"", "Admins", "a,b", "a b"} {
		if _, err := group.ParseName(raw); !errors.Is(err, group.ErrInvalid) {
			t.Errorf("ParseName(%q) error = %v, want ErrInvalid", raw, err)
		}
	}
}

func TestNewRequiresAtLeastOneMember(t *testing.T) {
	_, err := group.New("team", "", nil)
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewRejectsDuplicateMember(t *testing.T) {
	_, err := group.New("team", "", []group.Member{{UID: "alice"}, {UID: "alice"}})
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewRejectsMemberWithoutIdentity(t *testing.T) {
	_, err := group.New("team", "", []group.Member{{}})
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewRejectsTooLongDescription(t *testing.T) {
	_, err := group.New("team", strings.Repeat("d", 1025), []group.Member{{UID: "alice"}})
	if !errors.Is(err, group.ErrInvalid) {
		t.Fatalf("New() error = %v, want ErrInvalid", err)
	}
}

func TestNewAcceptsMemberOutsidePeople(t *testing.T) {
	created, err := group.New("team", "  Team  ", []group.Member{{DN: "cn=admin,dc=example,dc=com"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if created.Description != "Team" {
		t.Fatalf("Description = %q, want trimmed", created.Description)
	}
}

func TestCheckRemoval(t *testing.T) {
	pair, _ := group.New("team", "", []group.Member{{UID: "alice"}, {UID: "bob"}})
	solo, _ := group.New("solo", "", []group.Member{{UID: "alice"}})

	if err := pair.CheckRemoval("bob"); err != nil {
		t.Fatalf("CheckRemoval(bob) error = %v", err)
	}
	if err := pair.CheckRemoval("carol"); !errors.Is(err, group.ErrNotMember) {
		t.Fatalf("CheckRemoval(carol) error = %v, want ErrNotMember", err)
	}
	if err := solo.CheckRemoval("alice"); !errors.Is(err, group.ErrLastMember) {
		t.Fatalf("CheckRemoval(last) error = %v, want ErrLastMember", err)
	}
}

func TestIsSoleMember(t *testing.T) {
	pair, _ := group.New("team", "", []group.Member{{UID: "alice"}, {UID: "bob"}})
	solo, _ := group.New("solo", "", []group.Member{{UID: "alice"}})

	if pair.IsSoleMember("alice") || !solo.IsSoleMember("alice") || solo.IsSoleMember("bob") {
		t.Fatalf("IsSoleMember gives wrong answers")
	}
}

func TestSoleMemberErrorNamesGroups(t *testing.T) {
	err := &group.SoleMemberError{Groups: []group.Name{"solo", "duo"}}
	if !strings.Contains(err.Error(), "solo, duo") {
		t.Fatalf("Error() = %q, want group names", err.Error())
	}
}
