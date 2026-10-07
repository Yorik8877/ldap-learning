package user_service_test

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/services/user_service"
)

var errDirectoryHiccup = errors.New("directory hiccup")

type fakeStore struct {
	users          map[user.UID]user.User
	passwords      map[user.UID]string
	setPasswordErr error
	deleted        []user.UID
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: map[user.UID]user.User{}, passwords: map[user.UID]string{}}
}

func (f *fakeStore) List(_ context.Context) ([]user.User, error) {
	return slices.Collect(maps.Values(f.users)), nil
}

func (f *fakeStore) Get(_ context.Context, uid user.UID) (user.User, error) {
	found, exists := f.users[uid]
	if !exists {
		return user.User{}, user.ErrNotFound
	}
	return found, nil
}

func (f *fakeStore) Create(_ context.Context, created user.User) error {
	if _, exists := f.users[created.UID]; exists {
		return user.ErrAlreadyExists
	}
	f.users[created.UID] = created
	return nil
}

func (f *fakeStore) Update(_ context.Context, updated user.User) error {
	f.users[updated.UID] = updated
	return nil
}

func (f *fakeStore) SetPassword(_ context.Context, uid user.UID, password string) error {
	if f.setPasswordErr != nil {
		return f.setPasswordErr
	}
	f.passwords[uid] = password
	return nil
}

func (f *fakeStore) Delete(_ context.Context, uid user.UID) error {
	f.deleted = append(f.deleted, uid)
	delete(f.users, uid)
	return nil
}

type fakeGroups map[user.UID][]group.Group

func (f fakeGroups) GroupsOf(_ context.Context, uid user.UID) ([]group.Group, error) {
	return f[uid], nil
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

var john = user.User{UID: "jdoe", CommonName: "John Doe", Surname: "Doe", Emails: []string{}}

func TestCreateSetsPassword(t *testing.T) {
	store := newFakeStore()
	service := user_service.New(store, fakeGroups{})

	if err := service.Create(t.Context(), john, "correct-horse"); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if store.passwords["jdoe"] != "correct-horse" {
		t.Fatalf("password was not set")
	}
}

func TestCreateRejectsShortPasswordBeforeWriting(t *testing.T) {
	store := newFakeStore()
	service := user_service.New(store, fakeGroups{})

	err := service.Create(t.Context(), john, "short")

	if !errors.Is(err, user.ErrInvalid) {
		t.Fatalf("Create() error = %v, want ErrInvalid", err)
	}
	if len(store.users) != 0 {
		t.Fatalf("user was written despite invalid password")
	}
}

func TestCreateRollsBackWhenPasswordFails(t *testing.T) {
	store := newFakeStore()
	store.setPasswordErr = errDirectoryHiccup
	service := user_service.New(store, fakeGroups{})

	err := service.Create(t.Context(), john, "correct-horse")

	if !errors.Is(err, errDirectoryHiccup) {
		t.Fatalf("Create() error = %v, want the password error", err)
	}
	if !slices.Equal(store.deleted, []user.UID{"jdoe"}) || len(store.users) != 0 {
		t.Fatalf("user without password was not rolled back: deleted=%v users=%v", store.deleted, store.users)
	}
}

func TestSetPasswordValidatesLength(t *testing.T) {
	service := user_service.New(newFakeStore(), fakeGroups{})

	if err := service.SetPassword(t.Context(), "jdoe", "short"); !errors.Is(err, user.ErrInvalid) {
		t.Fatalf("SetPassword() error = %v, want ErrInvalid", err)
	}
}

func TestGetReturnsGroupNames(t *testing.T) {
	store := newFakeStore()
	store.users["jdoe"] = john
	groups := fakeGroups{"jdoe": {mustGroup(t, "team", "jdoe", "alice")}}
	service := user_service.New(store, groups)

	found, names, err := service.Get(t.Context(), "jdoe")

	if err != nil || found.UID != "jdoe" || !slices.Equal(names, []group.Name{"team"}) {
		t.Fatalf("Get() = %+v, %v, %v", found, names, err)
	}
}

func TestDeleteSelfIsRejected(t *testing.T) {
	store := newFakeStore()
	store.users["alice"] = user.User{UID: "alice"}
	service := user_service.New(store, fakeGroups{})

	err := service.Delete(t.Context(), "alice", "alice")

	if !errors.Is(err, user.ErrSelfDelete) || len(store.deleted) != 0 {
		t.Fatalf("Delete(self) error = %v, deleted = %v", err, store.deleted)
	}
}

func TestDeleteSoleMemberIsRejectedWithGroupNames(t *testing.T) {
	store := newFakeStore()
	store.users["jdoe"] = john
	groups := fakeGroups{"jdoe": {mustGroup(t, "solo", "jdoe"), mustGroup(t, "team", "jdoe", "alice")}}
	service := user_service.New(store, groups)

	err := service.Delete(t.Context(), "alice", "jdoe")

	var soleMember *group.SoleMemberError
	if !errors.As(err, &soleMember) || !slices.Equal(soleMember.Groups, []group.Name{"solo"}) {
		t.Fatalf("Delete() error = %v, want SoleMemberError{solo}", err)
	}
	if len(store.deleted) != 0 {
		t.Fatalf("user was deleted despite sole membership")
	}
}

func TestDeleteRemovesUserSharingGroups(t *testing.T) {
	store := newFakeStore()
	store.users["jdoe"] = john
	groups := fakeGroups{"jdoe": {mustGroup(t, "team", "jdoe", "alice")}}
	service := user_service.New(store, groups)

	if err := service.Delete(t.Context(), "alice", "jdoe"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !slices.Equal(store.deleted, []user.UID{"jdoe"}) {
		t.Fatalf("deleted = %v, want [jdoe]", store.deleted)
	}
}
