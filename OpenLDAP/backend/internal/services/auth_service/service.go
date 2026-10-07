// Package auth_service — вход в админку: пароль проверяется bind'ом в LDAP, доступ —
// членством в группе админов, дальше работает сессия.
package auth_service

import (
	"context"
	"fmt"
	"time"

	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
)

type PasswordVerifier interface {
	Verify(ctx context.Context, uid user.UID, password string) error
}

type AdminChecker interface {
	IsMember(ctx context.Context, name group.Name, uid user.UID) (bool, error)
}

type UserReader interface {
	Get(ctx context.Context, uid user.UID) (user.User, error)
}

type SessionStore interface {
	Save(ctx context.Context, stored session.Session) error
	Find(ctx context.Context, id string) (session.Session, error)
	Delete(ctx context.Context, id string) error
}

type Service struct {
	passwords   PasswordVerifier
	admins      AdminChecker
	users       UserReader
	sessions    SessionStore
	clock       session.Clock
	ids         session.IDGenerator
	adminsGroup group.Name
	sessionTTL  time.Duration
}

func New(
	passwords PasswordVerifier,
	admins AdminChecker,
	users UserReader,
	sessions SessionStore,
	clock session.Clock,
	ids session.IDGenerator,
	adminsGroup group.Name,
	sessionTTL time.Duration,
) *Service {
	return &Service{
		passwords:   passwords,
		admins:      admins,
		users:       users,
		sessions:    sessions,
		clock:       clock,
		ids:         ids,
		adminsGroup: adminsGroup,
		sessionTTL:  sessionTTL,
	}
}

func (s *Service) Login(ctx context.Context, rawUID, password string) (session.Session, error) {
	uid, err := user.ParseUID(rawUID)
	// Ответ одинаков для любой причины отказа: он не должен подсказывать, что именно не так.
	if err != nil || password == "" {
		return session.Session{}, session.ErrInvalidCredentials
	}
	if err := s.passwords.Verify(ctx, uid, password); err != nil {
		return session.Session{}, err
	}
	isAdmin, err := s.admins.IsMember(ctx, s.adminsGroup, uid)
	if err != nil {
		return session.Session{}, err
	}
	if !isAdmin {
		return session.Session{}, session.ErrNotAdmin
	}
	account, err := s.users.Get(ctx, uid)
	if err != nil {
		return session.Session{}, err
	}
	return s.start(ctx, account)
}

func (s *Service) Authenticate(ctx context.Context, id string) (session.Session, error) {
	found, err := s.sessions.Find(ctx, id)
	if err != nil {
		return session.Session{}, err
	}
	if !found.IsExpired(s.clock.Now()) {
		return found, nil
	}
	if err := s.sessions.Delete(ctx, id); err != nil {
		return session.Session{}, err
	}
	return session.Session{}, session.ErrNotFound
}

func (s *Service) Logout(ctx context.Context, id string) error {
	return s.sessions.Delete(ctx, id)
}

func (s *Service) start(ctx context.Context, account user.User) (session.Session, error) {
	id, err := s.ids.NewID()
	if err != nil {
		return session.Session{}, fmt.Errorf("generate session id: %w", err)
	}
	started := session.Session{
		ID:         id,
		UID:        account.UID,
		CommonName: account.CommonName,
		ExpiresAt:  s.clock.Now().Add(s.sessionTTL),
	}
	if err := s.sessions.Save(ctx, started); err != nil {
		return session.Session{}, err
	}
	return started, nil
}
