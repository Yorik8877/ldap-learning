package user

import (
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MinPasswordLength = 8
	maxNameLength     = 256
)

// uidPattern ограничивает uid безопасным подмножеством: такой uid можно подставить
// в DN и фильтр без сюрпризов, и он однозначен без учёта регистра.
var uidPattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)

type UID string

func ParseUID(raw string) (UID, error) {
	if !uidPattern.MatchString(raw) {
		return "", fmt.Errorf("%w: uid must start with a lowercase letter and contain only a-z, 0-9, '.', '_', '-' (up to 64 characters)", ErrInvalid)
	}
	return UID(raw), nil
}

type User struct {
	UID        UID
	CommonName string
	Surname    string
	Emails     []string
}

func New(uid UID, commonName, surname string, emails []string) (User, error) {
	if _, err := ParseUID(string(uid)); err != nil {
		return User{}, err
	}
	commonName = strings.TrimSpace(commonName)
	surname = strings.TrimSpace(surname)
	if err := validateName("common name", commonName); err != nil {
		return User{}, err
	}
	if err := validateName("surname", surname); err != nil {
		return User{}, err
	}
	normalizedEmails, err := normalizeEmails(emails)
	if err != nil {
		return User{}, err
	}
	return User{UID: uid, CommonName: commonName, Surname: surname, Emails: normalizedEmails}, nil
}

func ValidatePassword(password string) error {
	if utf8.RuneCountInString(password) < MinPasswordLength {
		return fmt.Errorf("%w: password must be at least %d characters", ErrInvalid, MinPasswordLength)
	}
	return nil
}

func validateName(field, value string) error {
	if value == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalid, field)
	}
	if utf8.RuneCountInString(value) > maxNameLength {
		return fmt.Errorf("%w: %s is longer than %d characters", ErrInvalid, field, maxNameLength)
	}
	return nil
}

func normalizeEmails(emails []string) ([]string, error) {
	normalized := make([]string, 0, len(emails))
	for _, email := range emails {
		trimmed := strings.TrimSpace(email)
		address, err := mail.ParseAddress(trimmed)
		if err != nil || address.Address != trimmed {
			return nil, fmt.Errorf("%w: %q is not a valid email address", ErrInvalid, trimmed)
		}
		normalized = append(normalized, trimmed)
	}
	return normalized, nil
}
