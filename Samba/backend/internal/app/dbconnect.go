package app

import (
	"fmt"
	"samba-admin/internal/config"
	"samba-admin/internal/db/ldap_db"
)

type LDAPConnProvider interface {
	Close() error
}

func initLDAP(cfg config.Config) (LDAPConnProvider, error) {
	const op string = "app.initLDAP"

	ldapClient, err := ldap_db.NewClient(
		cfg.LDAPURL,
		cfg.LDAPCAFile,
		cfg.BindDN,
		cfg.BindPassword,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return ldapClient, nil
}
