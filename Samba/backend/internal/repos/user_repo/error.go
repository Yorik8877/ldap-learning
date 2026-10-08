package user_repo

import (
	"errors"
	"samba-admin/internal/db/ldap_db"
)

var (
	ErrUserNotFound        error = errors.New("user not found")
	ErrTooManyUsersByLogin error = errors.New("too many users by provided login")

	ErrWrongLoginOrPassword error = errors.New("wrong login or password")
)

func (r *Repo) translateError(err error) error {
	if errors.Is(err, ldap_db.ErrInvalidCredentials) {
		return ErrWrongLoginOrPassword
	}

	return err
}
