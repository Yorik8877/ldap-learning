package session

import (
	"errors"
	"time"

	"ldap-admin/internal/domain/user"
)

var (
	ErrInvalidCredentials = errors.New("invalid uid or password")
	ErrNotAdmin           = errors.New("user is not a member of the admins group")
	ErrNotFound           = errors.New("session not found or expired")
)

type Session struct {
	ID         string
	UID        user.UID
	CommonName string
	ExpiresAt  time.Time
}

func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() (string, error)
}
