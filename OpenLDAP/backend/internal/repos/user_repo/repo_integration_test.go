//go:build integration

package user_repo_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/tree_layout"
	"ldap-admin/internal/repos/user_repo"
	"ldap-admin/internal/testsupport/ldapstand"
)

func newRepo(t *testing.T) (*user_repo.Repo, ldapstand.Stand) {
	t.Helper()
	stand := ldapstand.Connect(t)
	stand.CreateOU(t, "people")
	stand.CreateOU(t, "groups")
	return user_repo.New(stand.Client, tree_layout.New(stand.Base)), stand
}

func mustUser(t *testing.T, uid user.UID, commonName string, emails ...string) user.User {
	t.Helper()
	created, err := user.New(uid, commonName, "Doe", emails)
	if err != nil {
		t.Fatalf("user.New() error = %v", err)
	}
	return created
}

func TestCreateGetUpdateDelete(t *testing.T) {
	repo, _ := newRepo(t)
	john := mustUser(t, "jdoe", "Doe, John", "jdoe@example.com")

	if err := repo.Create(t.Context(), john); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	stored, err := repo.Get(t.Context(), "jdoe")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if stored.CommonName != "Doe, John" || len(stored.Emails) != 1 {
		t.Fatalf("Get() = %+v", stored)
	}

	if err := repo.Update(t.Context(), mustUser(t, "jdoe", "John Doe")); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	updated, _ := repo.Get(t.Context(), "jdoe")
	if updated.CommonName != "John Doe" || len(updated.Emails) != 0 {
		t.Fatalf("Get() after update = %+v, want new cn and no mail", updated)
	}

	if err := repo.Delete(t.Context(), "jdoe"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Get(t.Context(), "jdoe"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("Get() after delete error = %v, want ErrNotFound", err)
	}
}

// Домен уже отсекает такие адреса; тест проверяет страховку на случай, если значение
// дойдёт до сервера в обход домена: атрибут mail имеет синтаксис IA5String (только ASCII).
func TestValueRejectedBySchemaSyntaxIsInvalid(t *testing.T) {
	repo, _ := newRepo(t)
	bypassingDomain := user.User{UID: "jdoe", CommonName: "John", Surname: "Doe", Emails: []string{"иван@example.com"}}

	if err := repo.Create(t.Context(), bypassingDomain); !errors.Is(err, user.ErrInvalid) {
		t.Fatalf("Create() error = %v, want ErrInvalid", err)
	}
}

func TestCreateDuplicateIsAlreadyExists(t *testing.T) {
	repo, _ := newRepo(t)
	john := mustUser(t, "jdoe", "John")
	if err := repo.Create(t.Context(), john); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := repo.Create(t.Context(), john); !errors.Is(err, user.ErrAlreadyExists) {
		t.Fatalf("second Create() error = %v, want ErrAlreadyExists", err)
	}
}

func TestMissingUserIsNotFound(t *testing.T) {
	repo, _ := newRepo(t)

	if _, err := repo.Get(t.Context(), "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
	if err := repo.Update(t.Context(), mustUser(t, "ghost", "Ghost")); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Update() error = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(t.Context(), "ghost"); !errors.Is(err, user.ErrNotFound) {
		t.Errorf("Delete() error = %v, want ErrNotFound", err)
	}
}

func TestSetPasswordAndVerify(t *testing.T) {
	repo, _ := newRepo(t)
	if err := repo.Create(t.Context(), mustUser(t, "jdoe", "John")); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := repo.SetPassword(t.Context(), "jdoe", "correct-horse"); err != nil {
		t.Fatalf("SetPassword() error = %v", err)
	}

	if err := repo.Verify(t.Context(), "jdoe", "correct-horse"); err != nil {
		t.Errorf("Verify(correct) error = %v", err)
	}
	if err := repo.Verify(t.Context(), "jdoe", "wrong-horse"); !errors.Is(err, session.ErrInvalidCredentials) {
		t.Errorf("Verify(wrong) error = %v, want ErrInvalidCredentials", err)
	}
	if err := repo.Verify(t.Context(), "ghost", "whatever-1"); !errors.Is(err, session.ErrInvalidCredentials) {
		t.Errorf("Verify(missing user) error = %v, want ErrInvalidCredentials", err)
	}
}

func TestListIsSortedAndSkipsEntriesBreakingDomainRules(t *testing.T) {
	repo, stand := newRepo(t)
	for _, uid := range []user.UID{"zed", "amy"} {
		if err := repo.Create(t.Context(), mustUser(t, uid, string(uid))); err != nil {
			t.Fatalf("Create(%s) error = %v", uid, err)
		}
	}
	stand.CreatePerson(t, "ou=people,"+stand.Base, "Bob")

	users, err := repo.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 2 || users[0].UID != "amy" || users[1].UID != "zed" {
		t.Fatalf("List() = %+v, want [amy zed]", users)
	}
}

func TestListWithoutPeopleBranchIsNotNotFound(t *testing.T) {
	stand := ldapstand.Connect(t)
	repo := user_repo.New(stand.Client, tree_layout.New(stand.Base))

	_, err := repo.List(t.Context())

	if err == nil || errors.Is(err, user.ErrNotFound) {
		t.Fatalf("List() error = %v, want a configuration error", err)
	}
}
