package session

import "errors"

var (
	ErrNotFound error = errors.New("session not found")
	ErrExpired  error = errors.New("session is expired")
)
