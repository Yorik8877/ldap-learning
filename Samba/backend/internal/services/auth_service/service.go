package auth_service

import "samba-admin/internal/domain/user"

type UserAuthenticator interface {
	Authenticate(login, password string) (user.User, error)
}

type Service struct {
	users      UserAuthenticator
	adminGroup string
}

func New(users UserAuthenticator, adminGroup string) *Service {
	return &Service{
		users:      users,
		adminGroup: adminGroup,
	}
}
