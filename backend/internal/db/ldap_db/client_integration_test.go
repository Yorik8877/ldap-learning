//go:build integration

package ldap_db_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/testsupport/ldapstand"
)

func TestAddSearchAndDelete(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	aliceDN := stand.CreatePerson(t, people, "alice")

	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: people, Scope: ldap_db.ScopeOneLevel,
		Filter: ldap_db.FilterEquals("uid", "alice"), Attributes: []string{"cn"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(entries) != 1 || entries[0].First("cn") != "alice" {
		t.Fatalf("Search() = %+v, want one entry with cn=alice", entries)
	}

	if err := stand.Client.Delete(t.Context(), aliceDN); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = stand.Client.Search(t.Context(), ldap_db.SearchRequest{BaseDN: aliceDN, Scope: ldap_db.ScopeBase})
	if !errors.Is(err, ldap_db.ErrNoSuchObject) {
		t.Fatalf("Search() after delete error = %v, want ErrNoSuchObject", err)
	}
}

func TestAddExistingEntryIsAlreadyExists(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")

	err := stand.Client.Add(t.Context(), people, map[string][]string{"objectClass": {"organizationalUnit"}, "ou": {"people"}})

	if !errors.Is(err, ldap_db.ErrAlreadyExists) {
		t.Fatalf("Add() error = %v, want ErrAlreadyExists", err)
	}
}

func TestMemberOfFollowsUniqueMemberChanges(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	modifyMember(t, stand, teamDN, ldap_db.ChangeAdd, bobDN)
	if !containsFold(memberOf(t, stand, bobDN), teamDN) {
		t.Fatalf("memberOf(bob) after add = %v, want %s", memberOf(t, stand, bobDN), teamDN)
	}

	modifyMember(t, stand, teamDN, ldap_db.ChangeDelete, bobDN)
	if got := memberOf(t, stand, bobDN); len(got) != 0 {
		t.Fatalf("memberOf(bob) after delete = %v, want empty", got)
	}
}

func TestRefintRemovesDeletedUserFromGroups(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN, bobDN)

	if err := stand.Client.Delete(t.Context(), bobDN); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	members := uniqueMembers(t, stand, teamDN)
	if len(members) != 1 || !strings.EqualFold(members[0], aliceDN) {
		t.Fatalf("uniqueMember after delete = %v, want only %s", members, aliceDN)
	}
}

func TestRemovingLastUniqueMemberIsObjectClassViolation(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	err := stand.Client.Modify(t.Context(), teamDN, []ldap_db.Change{
		{Operation: ldap_db.ChangeDelete, Attribute: "uniqueMember", Values: []string{aliceDN}},
	})

	if !errors.Is(err, ldap_db.ErrObjectClassViolation) {
		t.Fatalf("Modify() error = %v, want ErrObjectClassViolation", err)
	}
}

func TestAddingPresentValueAndDeletingAbsentValue(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	addErr := stand.Client.Modify(t.Context(), teamDN, []ldap_db.Change{
		{Operation: ldap_db.ChangeAdd, Attribute: "uniqueMember", Values: []string{aliceDN}},
	})
	deleteErr := stand.Client.Modify(t.Context(), teamDN, []ldap_db.Change{
		{Operation: ldap_db.ChangeDelete, Attribute: "uniqueMember", Values: []string{bobDN}},
	})

	if !errors.Is(addErr, ldap_db.ErrValueExists) {
		t.Fatalf("add present value error = %v, want ErrValueExists", addErr)
	}
	if !errors.Is(deleteErr, ldap_db.ErrNoSuchValue) {
		t.Fatalf("delete absent value error = %v, want ErrNoSuchValue", deleteErr)
	}
}

func TestCompareUniqueMember(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	groups := stand.CreateOU(t, "groups")
	aliceDN := stand.CreatePerson(t, people, "alice")
	bobDN := stand.CreatePerson(t, people, "bob")
	teamDN := stand.CreateGroup(t, groups, "team", aliceDN)

	aliceMatched, aliceErr := stand.Client.Compare(t.Context(), teamDN, "uniqueMember", aliceDN)
	bobMatched, bobErr := stand.Client.Compare(t.Context(), teamDN, "uniqueMember", bobDN)

	if aliceErr != nil || !aliceMatched {
		t.Fatalf("Compare(alice) = (%v, %v), want (true, nil)", aliceMatched, aliceErr)
	}
	if bobErr != nil || bobMatched {
		t.Fatalf("Compare(bob) = (%v, %v), want (false, nil)", bobMatched, bobErr)
	}
}

func TestSetPasswordStoresHashAndVerifies(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	aliceDN := stand.CreatePerson(t, people, "alice")

	if err := stand.Client.SetPassword(t.Context(), aliceDN, "correct-horse"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}

	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: aliceDN, Scope: ldap_db.ScopeBase, Attributes: []string{"userPassword"},
	})
	if err != nil || len(entries) != 1 {
		t.Fatalf("read userPassword: entries=%v err=%v", entries, err)
	}
	if !strings.HasPrefix(entries[0].First("userPassword"), "{SSHA}") {
		t.Fatalf("userPassword is not an SSHA hash")
	}
	if err := stand.Client.VerifyPassword(t.Context(), aliceDN, "correct-horse"); err != nil {
		t.Fatalf("VerifyPassword(correct) error = %v", err)
	}
	if err := stand.Client.VerifyPassword(t.Context(), aliceDN, "wrong-horse"); !errors.Is(err, ldap_db.ErrInvalidCredentials) {
		t.Fatalf("VerifyPassword(wrong) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestHasSubordinatesIsReturnedOnlyWhenRequested(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	stand.CreatePerson(t, people, "alice")
	stand.CreateOU(t, "groups")

	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: stand.Base, Scope: ldap_db.ScopeOneLevel, Attributes: []string{"hasSubordinates"},
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	flags := map[string]string{}
	for _, entry := range entries {
		rdn, _ := ldap_db.FirstRDN(entry.DN)
		flags[rdn] = entry.First("hasSubordinates")
	}
	if flags["ou=people"] != "TRUE" || flags["ou=groups"] != "FALSE" {
		t.Fatalf("hasSubordinates = %v, want people TRUE and groups FALSE", flags)
	}
}

func TestWrongServicePasswordIsServiceBindError(t *testing.T) {
	stand := ldapstand.Connect(t)
	config := stand.Config
	config.BindPassword = "definitely-wrong"

	_, err := ldap_db.New(config).Search(t.Context(), ldap_db.SearchRequest{BaseDN: stand.Base, Scope: ldap_db.ScopeBase})

	if !errors.Is(err, ldap_db.ErrServiceBind) {
		t.Fatalf("Search() error = %v, want ErrServiceBind", err)
	}
	if errors.Is(err, ldap_db.ErrInvalidCredentials) {
		t.Fatalf("service bind failure must not look like user invalid credentials")
	}
}

func modifyMember(t *testing.T, stand ldapstand.Stand, groupDN string, operation ldap_db.ChangeOperation, memberDN string) {
	t.Helper()
	err := stand.Client.Modify(t.Context(), groupDN, []ldap_db.Change{
		{Operation: operation, Attribute: "uniqueMember", Values: []string{memberDN}},
	})
	if err != nil {
		t.Fatalf("Modify() error = %v", err)
	}
}

func memberOf(t *testing.T, stand ldapstand.Stand, dn string) []string {
	t.Helper()
	return readAttribute(t, stand, dn, "memberOf")
}

func uniqueMembers(t *testing.T, stand ldapstand.Stand, dn string) []string {
	t.Helper()
	return readAttribute(t, stand, dn, "uniqueMember")
}

func readAttribute(t *testing.T, stand ldapstand.Stand, dn, attribute string) []string {
	t.Helper()
	entries, err := stand.Client.Search(t.Context(), ldap_db.SearchRequest{
		BaseDN: dn, Scope: ldap_db.ScopeBase, Attributes: []string{attribute},
	})
	if err != nil || len(entries) != 1 {
		t.Fatalf("read %s of %s: entries=%v err=%v", attribute, dn, entries, err)
	}
	return entries[0].Values(attribute)
}

func containsFold(values []string, target string) bool {
	return slices.ContainsFunc(values, func(value string) bool { return strings.EqualFold(value, target) })
}
