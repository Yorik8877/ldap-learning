package auth_service

import (
	"samba-admin/internal/domain/session"
	"samba-admin/internal/domain/user"
	"time"
)

type UserAuthenticator interface {
	Authenticate(login, password string) (user.User, error)
}

type SessionStore interface {
	Save(stored session.Session) error
	Find(id string) (session.Session, error)
	Delete(id string) error
}

type IDGenerator interface {
	NewID() (string, error)
}

type Clock interface {
	Now() time.Time
}

type Service struct {
	users        UserAuthenticator
	sessionStore SessionStore
	idGenerator  IDGenerator
	clock        Clock
	adminGroup   string
	sessionTTL   time.Duration
}

func New(
	users UserAuthenticator,
	sessionStore SessionStore,
	idGenerator IDGenerator,
	clock Clock,
	adminGroup string,
	sessionTTL time.Duration,
) *Service {
	return &Service{
		users:        users,
		sessionStore: sessionStore,
		idGenerator:  idGenerator,
		clock:        clock,
		adminGroup:   adminGroup,
		sessionTTL:   sessionTTL,
	}
}
