package session_repo_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"samba-admin/internal/domain/session"
	"samba-admin/internal/repos/session_repo"
)

const concurrentCallers = 50

var expiresAt = time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)

func aliceSession(id string) session.Session {
	return session.Session{ID: id, Login: "alice", DisplayName: "Alice Admin", ExpiresAt: expiresAt}
}

func TestFindReturnsSavedSession(t *testing.T) {
	repo := session_repo.New()
	saved := aliceSession("first")
	if err := repo.Save(saved); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	found, err := repo.Find("first")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if found != saved {
		t.Fatalf("Find() = %+v, want %+v", found, saved)
	}
}

func TestFindUnknownID(t *testing.T) {
	repo := session_repo.New()
	if _, err := repo.Find("unknown"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Find(unknown) error = %v, want session.ErrNotFound", err)
	}
}

func TestDeleteRemovesSession(t *testing.T) {
	repo := session_repo.New()
	if err := repo.Save(aliceSession("first")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	if err := repo.Delete("first"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := repo.Find("first"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Find() after Delete() error = %v, want session.ErrNotFound", err)
	}
}

// Выход должен срабатывать и повторно, и с уже удалённой сессией.
func TestDeleteUnknownIDIsNotError(t *testing.T) {
	repo := session_repo.New()
	if err := repo.Delete("unknown"); err != nil {
		t.Fatalf("Delete(unknown) error = %v, want nil", err)
	}
}

func TestSaveOverwritesSameID(t *testing.T) {
	repo := session_repo.New()
	if err := repo.Save(aliceSession("first")); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	updated := aliceSession("first")
	updated.ExpiresAt = expiresAt.Add(time.Hour)
	if err := repo.Save(updated); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	found, err := repo.Find("first")
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if found != updated {
		t.Fatalf("Find() = %+v, want updated %+v", found, updated)
	}
}

// HTTP-сервер обрабатывает запросы параллельно. Без мьютекса одновременная запись и чтение map
// роняют процесс (fatal error: concurrent map read and map write); -race ловит гонку и раньше.
func TestConcurrentAccess(t *testing.T) {
	repo := session_repo.New()

	var wait sync.WaitGroup
	for callerIndex := range concurrentCallers {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id := fmt.Sprintf("session-%d", callerIndex)
			if err := repo.Save(aliceSession(id)); err != nil {
				t.Errorf("Save(%s) error = %v", id, err)
			}
			if _, err := repo.Find(id); err != nil {
				t.Errorf("Find(%s) error = %v", id, err)
			}
			if err := repo.Delete(id); err != nil {
				t.Errorf("Delete(%s) error = %v", id, err)
			}
		}()
	}
	wait.Wait()
}
