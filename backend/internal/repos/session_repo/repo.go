// Package session_repo хранит сессии в памяти процесса: перезапуск бэкенда
// разлогинивает всех, для учебного проекта это принято.
package session_repo

import (
	"context"
	"sync"

	"ldap-admin/internal/domain/session"
)

type Repo struct {
	mutex    sync.Mutex
	sessions map[string]session.Session
}

func New() *Repo {
	return &Repo{sessions: make(map[string]session.Session)}
}

func (r *Repo) Save(_ context.Context, stored session.Session) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.sessions[stored.ID] = stored
	return nil
}

func (r *Repo) Find(_ context.Context, id string) (session.Session, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	found, exists := r.sessions[id]
	if !exists {
		return session.Session{}, session.ErrNotFound
	}
	return found, nil
}

func (r *Repo) Delete(_ context.Context, id string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.sessions, id)
	return nil
}
