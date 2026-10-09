package user

import "errors"

var (
	ErrNotFound             error = errors.New("user not found")
	ErrWrongLoginOrPassword error = errors.New("wrong login or password")
	ErrNoAdminPrivilege     error = errors.New("user is not a panel admin")
)
