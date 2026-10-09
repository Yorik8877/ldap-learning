package session

import (
	"time"
)

type Session struct {
	ID          string
	Login       string
	DisplayName string
	ExpiresAt   time.Time
}

func (s Session) IsExpired(now time.Time) bool {
	return !now.Before(s.ExpiresAt)
}
