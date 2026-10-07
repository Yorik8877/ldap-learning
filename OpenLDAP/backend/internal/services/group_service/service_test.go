package group_service_test

import (
	"context"
	"errors"
	"testing"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/services/group_service"
)

type fakeStore struct {
	groups  map[group.Name]group.Group
	removed []user.UID
	added   []user.UID
	deleted []group.Name
}

func (f *fakeStore) List(_ context.Context) ([]group.Group, error) { return nil, nil }

func (f *fakeStore) Get(_ context.Context, name group.Name) (group.Group, error) {
	found, exists := f.groups[name]
	if !exists {
		return group.Group{}, group.ErrNotFound
	}
	return found, nil
}

func (f *fakeStore) Create(_ context.Context, created group.Group) error {
	f.groups[created.Name] = created
	return nil
}

func (f *fakeStore) Delete(_ context.Context, name group.Name) error {
	f.deleted = append(f.deleted, name)
	return nil
}

func (f *fakeStore) AddMember(_ context.Context, _ group.Name, uid user.UID) error {
	f.added = append(f.added, uid)
	return nil
}

func (f *fakeStore) RemoveMember(_ context.Context, _ group.Name, uid user.UID) error {
	f.removed = append(f.removed, uid)
	return nil
}

type fakeUsers map[user.UID]bool

func (f fakeUsers) Get(_ context.Context, uid user.UID) (user.User, error) {
	if !f[uid] {
		return user.User{}, user.ErrNotFound
	}
	return user.User{UID: uid}, nil
}

func newService(t *testing.T, groups ...group.Group) (*group_service.Service, *fakeStore) {
	t.Helper()
	store := &fakeStore{groups: map[group.Name]group.Group{}}
	for _, existing := range groups {
		store.groups[existing.Name] = existing
	}
	users := fakeUsers{"alice": true, "bob": true}
	return group_service.New(store, users, "admins"), store
}

func mustGroup(t *testing.T, name group.Name, uids ...user.UID) group.Group {
	t.Helper()
	members := make([]group.Member, 0, len(uids))
	for _, uid := range uids {
		members = append(members, group.Member{UID: uid})
	}
	created, err := group.New(name, "", members)
	if err != nil {
		t.Fatalf("group.New() error = %v", err)
	}
	return created
}

func TestCreateChecksMembersExist(t *testing.T) {
	service, store := newService(t)

	_, err := service.Create(t.Context(), mustGroup(t, "team", "alice", "ghost"))

	if !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("Create() error = %v, want user.ErrNotFound", err)
	}
	if len(store.groups) != 0 {
		t.Fatalf("group was written despite missing member")
	}
}

func TestCreateReturnsStoredGroup(t *testing.T) {
	service, _ := newService(t)

	created, err := service.Create(t.Context(), mustGroup(t, "team", "alice"))

	if err != nil || created.Name != "team" {
		t.Fatalf("Create() = %+v, %v", created, err)
	}
}

func TestDeleteAdminsGroupIsProtected(t *testing.T) {
	service, store := newService(t, mustGroup(t, "admins", "alice"))

	if err := service.Delete(t.Context(), "admins"); !errors.Is(err, group.ErrProtected) {
		t.Fatalf("Delete(admins) error = %v, want ErrProtected", err)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("protected group was deleted")
	}
}

func TestAddMemberRequiresExistingUser(t *testing.T) {
	service, store := newService(t, mustGroup(t, "team", "alice"))

	if err := service.AddMember(t.Context(), "team", "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("AddMember(ghost) error = %v, want user.ErrNotFound", err)
	}
	if err := service.AddMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("AddMember(bob) error = %v", err)
	}
	if len(store.added) != 1 || store.added[0] != "bob" {
		t.Fatalf("added = %v, want [bob]", store.added)
	}
}

func TestRemoveMemberChecksDomainRulesFirst(t *testing.T) {
	service, store := newService(t, mustGroup(t, "solo", "alice"), mustGroup(t, "team", "alice", "bob"))

	if err := service.RemoveMember(t.Context(), "solo", "alice"); !errors.Is(err, group.ErrLastMember) {
		t.Errorf("RemoveMember(last) error = %v, want ErrLastMember", err)
	}
	if err := service.RemoveMember(t.Context(), "team", "carol"); !errors.Is(err, group.ErrNotMember) {
		t.Errorf("RemoveMember(stranger) error = %v, want ErrNotMember", err)
	}
	if err := service.RemoveMember(t.Context(), "ghost", "alice"); !errors.Is(err, group.ErrNotFound) {
		t.Errorf("RemoveMember(missing group) error = %v, want ErrNotFound", err)
	}
	if len(store.removed) != 0 {
		t.Fatalf("store was called for rejected removals: %v", store.removed)
	}
	if err := service.RemoveMember(t.Context(), "team", "bob"); err != nil {
		t.Fatalf("RemoveMember(bob) error = %v", err)
	}
}
