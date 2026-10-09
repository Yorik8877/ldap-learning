package auth_service

import (
	"fmt"
)

func (svc *Service) Logout(id string) error {
	const op string = "auth_service.Logout"

	if err := svc.sessionStore.Delete(id); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}
