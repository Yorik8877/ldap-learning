package auth_service

import (
	"errors"
	"fmt"
	"samba-admin/internal/domain/session"
	"samba-admin/internal/domain/user"
	"slices"
	"strings"
)

func (svc *Service) Login(login, password string) (session.Session, error) {
	const op string = "auth_service.Login"
	authUser, err := svc.users.Authenticate(login, password)
	if err != nil {
		return session.Session{}, fmt.Errorf("%s: %w", op, svc.translateLoginError(err))
	}

	if ok := svc.hasAdminGroup(authUser.Groups); !ok {
		return session.Session{}, fmt.Errorf("%s: %w", op, user.ErrNoAdminPrivilege)
	}

	sessionID, err := svc.idGenerator.NewID()
	if err != nil {
		return session.Session{}, fmt.Errorf("%s: %w", op, err)
	}

	openedSession := session.Session{
		ID:          sessionID,
		Login:       authUser.Login,
		DisplayName: authUser.DisplayName,
		ExpiresAt:   svc.clock.Now().Add(svc.sessionTTL),
	}

	err = svc.sessionStore.Save(openedSession)
	if err != nil {
		return session.Session{}, fmt.Errorf("%s: %w", op, err)
	}

	return openedSession, nil
}

func (svc *Service) hasAdminGroup(groups []string) bool {
	return slices.ContainsFunc(groups, func(group string) bool {
		return strings.EqualFold(group, svc.adminGroup)
	})
}

func (svc *Service) translateLoginError(err error) error {
	if errors.Is(err, user.ErrNotFound) {
		return user.ErrWrongLoginOrPassword
	}

	return err
}
