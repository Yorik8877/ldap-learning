//go:build integration

package directory_repo_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/repos/directory_repo"
	"ldap-admin/internal/testsupport/ldapstand"
)

func dnPointer(value string) *directory.DN {
	dn := directory.DN(value)
	return &dn
}

func TestChildrenOfBase(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	stand.CreatePerson(t, people, "alice")
	stand.CreateOU(t, "groups")
	repo := directory_repo.New(stand.Client, stand.Base)

	nodes, err := repo.Children(t.Context(), nil)
	if err != nil {
		t.Fatalf("Children() error = %v", err)
	}
	if len(nodes) != 2 || nodes[0].RDN != "ou=groups" || nodes[1].RDN != "ou=people" {
		t.Fatalf("Children() = %+v, want groups then people", nodes)
	}
	if nodes[0].HasChildren || !nodes[1].HasChildren {
		t.Fatalf("HasChildren flags = %v/%v, want false/true", nodes[0].HasChildren, nodes[1].HasChildren)
	}
	if len(nodes[1].ObjectClasses) == 0 {
		t.Fatalf("ObjectClasses are empty")
	}
}

func TestEntrySplitsAttributesAndHidesPassword(t *testing.T) {
	stand := ldapstand.Connect(t)
	people := stand.CreateOU(t, "people")
	aliceDN := stand.CreatePerson(t, people, "alice")
	if err := stand.Client.SetPassword(t.Context(), aliceDN, "correct-horse"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}
	repo := directory_repo.New(stand.Client, stand.Base)

	entry, err := repo.Entry(t.Context(), dnPointer(aliceDN))
	if err != nil {
		t.Fatalf("Entry() error = %v", err)
	}
	if len(entry.Attributes["cn"]) == 0 {
		t.Errorf("Attributes have no cn: %v", entry.Attributes)
	}
	for name := range entry.Attributes {
		if name == "userPassword" || name == "userpassword" {
			t.Errorf("userPassword leaked into Attributes")
		}
	}
	if len(entry.OperationalAttributes["entryUUID"]) == 0 || len(entry.OperationalAttributes["structuralObjectClass"]) == 0 {
		t.Errorf("OperationalAttributes = %v, want entryUUID and structuralObjectClass", entry.OperationalAttributes)
	}
}

func TestEntryWithoutDNIsBase(t *testing.T) {
	stand := ldapstand.Connect(t)
	repo := directory_repo.New(stand.Client, stand.Base)

	entry, err := repo.Entry(t.Context(), nil)
	if err != nil || string(entry.DN) != stand.Base {
		t.Fatalf("Entry(nil) = %v, %v; want base %s", entry.DN, err, stand.Base)
	}
}

func TestRejectedDNs(t *testing.T) {
	stand := ldapstand.Connect(t)
	repo := directory_repo.New(stand.Client, stand.Base)

	if _, err := repo.Entry(t.Context(), dnPointer("cn=config")); !errors.Is(err, directory.ErrOutsideBase) {
		t.Errorf("Entry(cn=config) error = %v, want ErrOutsideBase", err)
	}
	if _, err := repo.Children(t.Context(), dnPointer("not a dn")); !errors.Is(err, directory.ErrInvalidDN) {
		t.Errorf("Children(not a dn) error = %v, want ErrInvalidDN", err)
	}
	if _, err := repo.Entry(t.Context(), dnPointer("ou=missing,"+stand.Base)); !errors.Is(err, directory.ErrNotFound) {
		t.Errorf("Entry(missing) error = %v, want ErrNotFound", err)
	}
	// go-ldap разбирает такой DN, а сервер отвечает 34 invalidDNSyntax: атрибута foo нет в схеме.
	if _, err := repo.Entry(t.Context(), dnPointer("foo=bar,"+stand.Base)); !errors.Is(err, directory.ErrInvalidDN) {
		t.Errorf("Entry(foo=bar) error = %v, want ErrInvalidDN", err)
	}
}
