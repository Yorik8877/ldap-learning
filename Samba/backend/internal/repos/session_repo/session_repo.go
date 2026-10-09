package session_repo

import (
	"samba-admin/internal/domain/session"
	"sync"
)

type Repo struct {
	mu       sync.Mutex
	sessions map[string]session.Session
}

func New() *Repo {
	return &Repo{
		sessions: make(map[string]session.Session),
	}
}

func (r *Repo) Save(stored session.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[stored.ID] = stored

	return nil
}

func (r *Repo) Find(id string) (session.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	found, ok := r.sessions[id]
	if !ok {
		return session.Session{}, session.ErrNotFound
	}

	return found, nil
}

func (r *Repo) Delete(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessions, id)

	return nil
}
