package session_test

import (
	"testing"
	"time"

	"ldap-admin/internal/domain/session"
)

func TestIsExpired(t *testing.T) {
	expiresAt := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	current := session.Session{ExpiresAt: expiresAt}

	if current.IsExpired(expiresAt.Add(-time.Second)) {
		t.Fatalf("session expired a second before its deadline")
	}
	if !current.IsExpired(expiresAt) {
		t.Fatalf("session must be expired exactly at its deadline")
	}
}
