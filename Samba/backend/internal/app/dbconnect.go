package app

import (
	"fmt"
	"samba-admin/internal/config"
	"samba-admin/internal/db/ldap_db"
)

func mustInitLDAP(cfg *config.Config) (ldap_db.ConnProvider, error) {
	const op string = "main.mustInitLDAP"
	connLDAP, err := ldap_db.Connect(
		cfg.BindDN,
		cfg.BindPassword,
		cfg.LDAPURL,
		cfg.LDAPCAFile,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return connLDAP, nil
}
