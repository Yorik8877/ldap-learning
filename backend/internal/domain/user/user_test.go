package user_test

import (
	"errors"
	"strings"
	"testing"

	"ldap-admin/internal/domain/user"
)

func TestParseUID(t *testing.T) {
	valid := []string{"alice", "a", "john.doe", "user_1", "x-y", "a" + strings.Repeat("b", 63)}
	for _, raw := range valid {
		if _, err := user.ParseUID(raw); err != nil {
			t.Errorf("ParseUID(%q) error = %v, want nil", raw, err)
		}
	}
	invalid := []string{"", "Alice", "1alice", "a b", "a*)(uid=*", "uid,ou=x", "a" + strings.Repeat("b", 64)}
	for _, raw := range invalid {
		if _, err := user.ParseUID(raw); !errors.Is(err, user.ErrInvalid) {
			t.Errorf("ParseUID(%q) error = %v, want ErrInvalid", raw, err)
		}
	}
}

func TestNewTrimsNamesAndKeepsDNSpecialCharacters(t *testing.T) {
	created, err := user.New("jdoe", "  Doe, John  ", " Doe ", []string{" jdoe@example.com "})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if created.CommonName != "Doe, John" || created.Surname != "Doe" {
		t.Fatalf("New() names = %q / %q", created.CommonName, created.Surname)
	}
	if len(created.Emails) != 1 || created.Emails[0] != "jdoe@example.com" {
		t.Fatalf("New() emails = %v", created.Emails)
	}
}

func TestNewWithoutEmailsReturnsEmptySlice(t *testing.T) {
	created, err := user.New("jdoe", "John", "Doe", nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if created.Emails == nil || len(created.Emails) != 0 {
		t.Fatalf("New() emails = %#v, want empty non-nil slice", created.Emails)
	}
}

func TestNewRejectsInvalidData(t *testing.T) {
	cases := []struct {
		name       string
		uid        user.UID
		commonName string
		surname    string
		emails     []string
	}{
		{name: "invalid uid", uid: "Bad", commonName: "John", surname: "Doe"},
		{name: "blank common name", uid: "jdoe", commonName: "   ", surname: "Doe"},
		{name: "blank surname", uid: "jdoe", commonName: "John", surname: ""},
		{name: "too long surname", uid: "jdoe", commonName: "John", surname: strings.Repeat("я", 257)},
		{name: "not an email", uid: "jdoe", commonName: "John", surname: "Doe", emails: []string{"nope"}},
		{name: "email with display name", uid: "jdoe", commonName: "John", surname: "Doe", emails: []string{"John <j@example.com>"}},
		{name: "non-ascii local part", uid: "jdoe", commonName: "John", surname: "Doe", emails: []string{"иван@example.com"}},
		{name: "non-ascii domain", uid: "jdoe", commonName: "John", surname: "Doe", emails: []string{"ivan@почта.рф"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := user.New(testCase.uid, testCase.commonName, testCase.surname, testCase.emails)
			if !errors.Is(err, user.ErrInvalid) {
				t.Fatalf("New() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	if err := user.ValidatePassword("12345678"); err != nil {
		t.Fatalf("ValidatePassword(8 chars) error = %v", err)
	}
	if err := user.ValidatePassword("пароль12"); err != nil {
		t.Fatalf("ValidatePassword(8 runes) error = %v", err)
	}
	for _, password := range []string{"", "1234567"} {
		if err := user.ValidatePassword(password); !errors.Is(err, user.ErrInvalid) {
			t.Errorf("ValidatePassword(%q) error = %v, want ErrInvalid", password, err)
		}
	}
}
