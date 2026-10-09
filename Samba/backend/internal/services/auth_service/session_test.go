package auth_service_test

import (
	"errors"
	"testing"
	"time"

	"samba-admin/internal/domain/session"
)

func TestCurrentReturnsOpenSession(t *testing.T) {
	environment := newLoggedInEnvironment(t)
	environment.clock.now = startTime.Add(sessionTTL - time.Second)

	current, err := environment.service.Current(sessionID)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if current.Login != "alice" || current.DisplayName != "Alice Admin" {
		t.Fatalf("Current() = %+v, want alice / Alice Admin", current)
	}
}

func TestCurrentRemovesExpiredSession(t *testing.T) {
	environment := newLoggedInEnvironment(t)
	environment.clock.now = startTime.Add(sessionTTL)

	if _, err := environment.service.Current(sessionID); !errors.Is(err, session.ErrExpired) {
		t.Fatalf("Current() of expired session error = %v, want session.ErrExpired", err)
	}
	if _, err := environment.store.Find(sessionID); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("expired session is still in the store: Find() error = %v", err)
	}
}

func TestCurrentUnknownID(t *testing.T) {
	environment := newLoggedInEnvironment(t)

	if _, err := environment.service.Current("unknown"); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Current(unknown) error = %v, want session.ErrNotFound", err)
	}
}
