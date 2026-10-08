package user_repo

import (
	"errors"
	"samba-admin/internal/db/ldap_db"
	"samba-admin/internal/domain/user"
)

var (
	ErrTooManyUsersByLogin error = errors.New("too many users by provided login")
)

func (r *Repo) translateError(err error) error {
	if errors.Is(err, ldap_db.ErrInvalidCredentials) {
		return user.ErrWrongLoginOrPassword
	}

	return err
}
