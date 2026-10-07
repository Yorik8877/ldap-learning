package directory_service_test

import (
	"context"
	"testing"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/services/directory_service"
)

type recordingBrowser struct {
	requested []*directory.DN
}

func (b *recordingBrowser) Children(_ context.Context, dn *directory.DN) ([]directory.Node, error) {
	b.requested = append(b.requested, dn)
	return []directory.Node{{DN: "ou=people,dc=example,dc=com", RDN: "ou=people"}}, nil
}

func (b *recordingBrowser) Entry(_ context.Context, dn *directory.DN) (directory.Entry, error) {
	b.requested = append(b.requested, dn)
	return directory.Entry{DN: "dc=example,dc=com"}, nil
}

func TestServicePassesRequestedDNThrough(t *testing.T) {
	browser := &recordingBrowser{}
	service := directory_service.New(browser)
	people := directory.DN("ou=people,dc=example,dc=com")

	nodes, err := service.Children(t.Context(), nil)
	if err != nil || len(nodes) != 1 {
		t.Fatalf("Children() = %v, %v", nodes, err)
	}
	if _, err := service.Entry(t.Context(), &people); err != nil {
		t.Fatalf("Entry() error = %v", err)
	}
	if browser.requested[0] != nil || *browser.requested[1] != people {
		t.Fatalf("requested = %v, want [nil, people]", browser.requested)
	}
}
