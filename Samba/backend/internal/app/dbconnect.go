package app

import (
	"fmt"
	"samba-admin/internal/config"
	"samba-admin/internal/db/ldap_db"
)

func initLDAPClient(cfg config.Config) (*ldap_db.Client, error) {
	const op string = "app.initLDAPClient"

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
