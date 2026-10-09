package auth_service

import (
	"errors"
	"fmt"
	"samba-admin/internal/domain/user"
	"slices"
	"strings"
)

func (svc *Service) Login(login, password string) (user.User, error) {
	const op string = "auth_service.Login"
	authUser, err := svc.users.Authenticate(login, password)
	if err != nil {
		return user.User{}, fmt.Errorf("%s: %w", op, svc.translateLoginError(err))
	}

	if ok := svc.hasAdminGroup(authUser.Groups); !ok {
		return user.User{}, fmt.Errorf("%s: %w", op, user.ErrNoAdminPrivilege)
	}

	return authUser, nil
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
