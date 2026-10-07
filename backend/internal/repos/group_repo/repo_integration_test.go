//go:build integration

package group_repo_test

import (
	"errors"
	"slices"
	"testing"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/group_repo"
	"ldap-admin/internal/repos/tree_layout"
	"ldap-admin/internal/testsupport/ldapstand"
)

type fixture struct {
	repo   *group_repo.Repo
	stand  ldapstand.Stand
	layout tree_layout.Layout
}

func newFixture(t *testing.T, uids ...string) fixture {
	t.Helper()
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	stand.CreateOU(t, "groups")
	for _, uid := range uids {
		stand.CreatePerson(t, people, uid)
	}
	layout := tree_layout.New(stand.Base)
	return fixture{repo: group_repo.New(stand.Client, layout), stand: stand, layout: layout}
}

func (f fixture) createGroup(t *testing.T, name group.Name, uids ...user.UID) {
	t.Helper()
	members := make([]group.Member, 0, len(uids))
	for _, uid := range uids {
		members = append(members, group.Member{UID: uid})
	}
	created, err := group.New(name, "Test group", members)
	if err != nil {
		t.Fatalf("group.New() error = %v", err)
	}
	if err := f.repo.Create(t.Context(), created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestCreateGetListDelete(t *testing.T) {
	f := newFixture(t, "alice")
	f.createGroup(t, "team", "alice")

	stored, err := f.repo.Get(t.Context(), "team")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.Description != "Test group" || len(stored.Members) != 1 || stored.Members[0].UID != "alice" {
		t.Fatalf("Get() = %+v", stored)
	}
	if stored.Members[0].DN == "" {
		t.Fatalf("member DN is empty")
	}

	groups, err := f.repo.List(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("List() = %v, %v", groups, err)
	}

	if err := f.repo.Delete(t.Context(), "team"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := f.repo.Get(t.Context(), "team"); !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

func TestCreateDuplicateIsAlreadyExists(t *testing.T) {
	f := newFixture(t, "alice")
	f.createGroup(t, "team", "alice")

	duplicate, _ := group.New("team", "", []group.Member{{UID: "alice"}})
	if err := f.repo.Create(t.Context(), duplicate); !errors.Is(err, group.ErrAlreadyExists) {
		t.Fatalf("Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestMembershipChangesAreVisibleThroughMemberOf(t *testing.T) {
	f := newFixture(t, "alice", "bob")
	f.createGroup(t, "team", "alice")

	if err := f.repo.AddMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}
	groups, err := f.repo.GroupsOf(t.Context(), "bob")
	if err != nil || len(groups) != 1 || groups[0].Name != "team" {
		t.Fatalf("GroupsOf(bob) = %v, %v; want [team]", groups, err)
	}

	if err := f.repo.RemoveMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("RemoveMember() error = %v", err)
	}
	groups, err = f.repo.GroupsOf(t.Context(), "bob")
	if err != nil || len(groups) != 0 {
		t.Fatalf("GroupsOf(bob) after removal = %v, %v; want none", groups, err)
	}
}

func TestMembershipErrors(t *testing.T) {
	f := newFixture(t, "alice", "bob")
	f.createGroup(t, "team", "alice")

	if err := f.repo.AddMember(t.Context(), "team", "alice"); !errors.Is(err, group.ErrAlreadyMember) {
		t.Errorf("AddMember(existing) error = %v, want ErrAlreadyMember", err)
	}
	if err := f.repo.RemoveMember(t.Context(), "team", "bob"); !errors.Is(err, group.ErrNotMember) {
		t.Errorf("RemoveMember(absent) error = %v, want ErrNotMember", err)
	}
	if err := f.repo.RemoveMember(t.Context(), "team", "alice"); !errors.Is(err, group.ErrLastMember) {
		t.Errorf("RemoveMember(last) error = %v, want ErrLastMember", err)
	}
	if err := f.repo.AddMember(t.Context(), "ghost", "alice"); !errors.Is(err, group.ErrNotFound) {
		t.Errorf("AddMember(missing group) error = %v, want ErrNotFound", err)
	}
}

func TestIsMember(t *testing.T) {
	f := newFixture(t, "alice", "bob")
	f.createGroup(t, "admins", "alice")

	cases := []struct {
		name group.Name
		uid  user.UID
		want bool
	}{
		{name: "admins", uid: "alice", want: true},
		{name: "admins", uid: "bob", want: false},
		{name: "ghost", uid: "alice", want: false},
	}
	for _, testCase := range cases {
		got, err := f.repo.IsMember(t.Context(), testCase.name, testCase.uid)
		if err != nil || got != testCase.want {
			t.Errorf("IsMember(%s, %s) = (%v, %v), want (%v, nil)", testCase.name, testCase.uid, got, err, testCase.want)
		}
	}
}

func TestMemberOutsidePeopleHasNoUID(t *testing.T) {
	f := newFixture(t)
	f.stand.CreateGroup(t, f.layout.GroupsDN(), "service", f.stand.Base)

	stored, err := f.repo.Get(t.Context(), "service")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if len(stored.Members) != 1 || stored.Members[0].UID != "" || stored.Members[0].DN == "" {
		t.Fatalf("Get() members = %+v, want one member with DN only", stored.Members)
	}
}

func TestListSkipsGroupsBreakingDomainRules(t *testing.T) {
	f := newFixture(t, "alice")
	aliceDN := f.layout.UserDN("alice")
	f.stand.CreateGroup(t, f.layout.GroupsDN(), "Mixed Case", aliceDN)
	f.createGroup(t, "team", "alice")

	groups, err := f.repo.List(t.Context())
	if err != nil || len(groups) != 1 || groups[0].Name != "team" {
		t.Fatalf("List() = %v, %v; want only team", groups, err)
	}
}

func TestGroupsOfMissingUserIsUserNotFound(t *testing.T) {
	f := newFixture(t)

	if _, err := f.repo.GroupsOf(t.Context(), "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("GroupsOf() error = %v, want user.ErrNotFound", err)
	}
}

// Группа, заведённая через ldapadd и не проходящая доменные правила, не должна ломать
// карточку и удаление своих участников: они читают группы через GroupsOf.
func TestGroupsOfIncludesGroupsBreakingDomainRules(t *testing.T) {
	f := newFixture(t, "alice", "bob")
	aliceDN := f.layout.UserDN("alice")
	f.stand.CreateGroup(t, f.layout.GroupsDN(), "Developers", aliceDN)
	f.stand.CreateGroup(t, f.layout.GroupsDN(), "Pair", aliceDN, f.layout.UserDN("bob"))

	memberships, err := f.repo.GroupsOf(t.Context(), "alice")

	if err != nil {
		t.Fatalf("GroupsOf() error = %v", err)
	}
	names := make([]string, 0, len(memberships))
	for _, membership := range memberships {
		names = append(names, string(membership.Name))
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Developers", "Pair"}) {
		t.Fatalf("GroupsOf() names = %v, want [Developers Pair]", names)
	}
}
