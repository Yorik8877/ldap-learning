package group_repo

import "errors"

var (
	ErrTooManyGroupsByName error = errors.New("too many groups by provided name")
)
