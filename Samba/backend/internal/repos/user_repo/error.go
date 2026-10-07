package user_repo

import "errors"

var (
	ErrUserNotFound        error = errors.New("user not found")
	ErrTooManyUsersByLogin error = errors.New("too many users by provided login")
)
