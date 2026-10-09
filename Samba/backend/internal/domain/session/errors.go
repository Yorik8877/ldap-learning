package session

import "errors"

var (
	ErrNotFound error = errors.New("session not found or expired")
)
