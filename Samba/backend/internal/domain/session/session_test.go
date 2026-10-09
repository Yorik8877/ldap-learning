package session_test

import (
	"testing"
	"time"

	"samba-admin/internal/domain/session"
)

func TestIsExpired(t *testing.T) {
	expiresAt := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	current := session.Session{ExpiresAt: expiresAt}

	testCases := []struct {
		caseName    string
		now         time.Time
		wantExpired bool
	}{
		{"an hour before deadline", expiresAt.Add(-time.Hour), false},
		{"a second before deadline", expiresAt.Add(-time.Second), false},
		{"exactly at deadline", expiresAt, true},
		{"an hour after deadline", expiresAt.Add(time.Hour), true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.caseName, func(t *testing.T) {
			if expired := current.IsExpired(testCase.now); expired != testCase.wantExpired {
				t.Fatalf("IsExpired(%s) = %v, want %v", testCase.now, expired, testCase.wantExpired)
			}
		})
	}
}
