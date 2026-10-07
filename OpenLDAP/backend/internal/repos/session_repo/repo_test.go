package session_repo_test

import (
	"errors"
	"testing"

	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/repos/session_repo"
)

func TestSaveFindDelete(t *testing.T) {
	repo := session_repo.New()
	stored := session.Session{ID: "abc", UID: "alice"}

	if err := repo.Save(t.Context(), stored); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	found, err := repo.Find(t.Context(), "abc")
	if err != nil || found.UID != "alice" {
		t.Fatalf("Find() = %+v, %v", found, err)
	}
	if err := repo.Delete(t.Context(), "abc"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Find(t.Context(), "abc"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Find() after delete error = %v, want ErrNotFound", err)
	}
}
