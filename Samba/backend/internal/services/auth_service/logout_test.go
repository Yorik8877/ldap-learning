package auth_service_test

import (
	"errors"
	"testing"

	"samba-admin/internal/domain/session"
)

func TestLogoutClosesSession(t *testing.T) {
	environment := newLoggedInEnvironment(t)

	if err := environment.service.Logout(sessionID); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if _, err := environment.service.Current(sessionID); !errors.Is(err, session.ErrNotFound) {
		t.Fatalf("Current() after Logout() error = %v, want session.ErrNotFound", err)
	}
}

// fmt.Errorf("%w", nil) — это не nil, а ошибка с текстом %!w(<nil>): успешный выход не должен её возвращать.
func TestLogoutUnknownIDIsNotError(t *testing.T) {
	environment := newLoggedInEnvironment(t)

	if err := environment.service.Logout("unknown"); err != nil {
		t.Fatalf("Logout(unknown) error = %v, want nil", err)
	}
}
