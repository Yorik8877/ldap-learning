package ldap_db

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestTranslateMapsResultCodes(t *testing.T) {
	err := translate(ldap.NewError(ldap.LDAPResultNoSuchObject, errors.New("no such object")))

	if !errors.Is(err, ErrNoSuchObject) {
		t.Fatalf("translate() = %v, want ErrNoSuchObject", err)
	}
}

func TestTranslateTreatsErrorWithoutResultCodeAsUnavailable(t *testing.T) {
	err := translate(errors.New("unable to read LDAP response packet: EOF"))

	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("translate() = %v, want ErrUnavailable", err)
	}
}

func TestServiceBindErrorSeparatesRefusalFromOutage(t *testing.T) {
	refused := serviceBindError(ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("invalid credentials")))
	dropped := serviceBindError(errors.New("unable to read LDAP response packet: EOF"))

	if !errors.Is(refused, ErrServiceBind) || errors.Is(refused, ErrInvalidCredentials) {
		t.Fatalf("refused bind = %v, want ErrServiceBind only", refused)
	}
	if !errors.Is(dropped, ErrUnavailable) {
		t.Fatalf("dropped connection = %v, want ErrUnavailable", dropped)
	}
}
