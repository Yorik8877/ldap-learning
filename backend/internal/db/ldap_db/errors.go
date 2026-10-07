package ldap_db

import (
	"errors"
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

var (
	ErrNoSuchObject         = errors.New("ldap: no such object")
	ErrAlreadyExists        = errors.New("ldap: entry already exists")
	ErrObjectClassViolation = errors.New("ldap: object class violation")
	ErrValueExists          = errors.New("ldap: attribute value already exists")
	ErrNoSuchValue          = errors.New("ldap: no such attribute value")
	ErrInvalidCredentials   = errors.New("ldap: invalid credentials")
	ErrUnavailable          = errors.New("ldap: directory unavailable")
	ErrServiceBind          = errors.New("ldap: service account bind failed")
	ErrInvalidDN            = errors.New("ldap: invalid dn")
)

var resultCodeErrors = map[uint16]error{
	ldap.LDAPResultNoSuchAttribute:        ErrNoSuchValue,
	ldap.LDAPResultAttributeOrValueExists: ErrValueExists,
	ldap.LDAPResultNoSuchObject:           ErrNoSuchObject,
	ldap.LDAPResultInvalidCredentials:     ErrInvalidCredentials,
	ldap.LDAPResultBusy:                   ErrUnavailable,
	ldap.LDAPResultUnavailable:            ErrUnavailable,
	ldap.LDAPResultObjectClassViolation:   ErrObjectClassViolation,
	ldap.LDAPResultEntryAlreadyExists:     ErrAlreadyExists,
	ldap.ErrorNetwork:                     ErrUnavailable,
}

// translate превращает ошибку go-ldap в sentinel этого пакета, чтобы слой репозиториев
// не зависел от кодов результата LDAP. Незнакомые коды проходят как есть.
func translate(err error) error {
	if err == nil {
		return nil
	}
	var ldapError *ldap.Error
	if !errors.As(err, &ldapError) {
		// Без кода результата — значит, ответа сервера не было: go-ldap так сообщает
		// об оборванном посреди запроса соединении («unable to read LDAP response packet»).
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	sentinel, known := resultCodeErrors[ldapError.ResultCode]
	if !known {
		return err
	}
	return fmt.Errorf("%w: %v", sentinel, err)
}
