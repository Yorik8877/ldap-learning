package ldap_db

import (
	"errors"

	"github.com/go-ldap/ldap/v3"
)

var (
	ErrInvalidCredentials error = errors.New("invalid credentials")
	ErrNoSuchObject       error = errors.New("no such object")
	ErrUnavailable        error = errors.New("samba is unavailable")

	ErrUnknownScope error = errors.New("unknown scope was given")

	ErrEmptyDNGiven error = errors.New("empty dn was given")
	ErrNoCommonName error = errors.New("no common name")
)

func translateError(err error) error {
	var ldapError *ldap.Error
	if !errors.As(err, &ldapError) {
		return err // не LDAP-ошибка — отдаём как есть
	}

	switch ldapError.ResultCode {
	case ldap.LDAPResultInvalidCredentials:
		return ErrInvalidCredentials
	case ldap.LDAPResultNoSuchObject:
		return ErrNoSuchObject
	case ldap.ErrorNetwork:
		return ErrUnavailable
	}

	return err
}
