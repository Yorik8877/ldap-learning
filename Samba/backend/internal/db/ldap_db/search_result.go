package ldap_db

import "github.com/go-ldap/ldap/v3"

// Запись из LDAP
type Entry struct {
	entry *ldap.Entry
}

func (e Entry) DN() string {
	return e.entry.DN
}

func (e Entry) Unmarshal(target any) error {
	return e.entry.Unmarshal(target)
}
