package auth_service

import (
	"fmt"
	"samba-admin/internal/domain/session"
)

func (svc *Service) Current(id string) (session.Session, error) {
	const op string = "auth_service.Current"

	found, err := svc.sessionStore.Find(id)
	if err != nil {
		return session.Session{}, fmt.Errorf("%s: %w", op, err)
	}

	if found.IsExpired(svc.clock.Now()) {
		_ = svc.sessionStore.Delete(found.ID)
		return session.Session{}, fmt.Errorf("%s: %w", op, session.ErrExpired)
	}

	return found, nil
}
