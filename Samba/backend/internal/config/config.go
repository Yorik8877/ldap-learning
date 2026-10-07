// Package config читает настройки из переменных окружения.
// Конфиг грузится один раз в composition root и дальше передаётся параметрами.
package config

import (
	"errors"
	"fmt"
	"time"
)

type Config struct {
	HTTPAddress  string
	LDAPURL      string
	LDAPCAFile   string
	BindDN       string
	BindPassword string
	BaseDN       string
	AdminGroup   string
	SessionTTL   time.Duration
}

func Load(lookup func(string) (string, bool)) (Config, error) {
	bindPassword, _ := lookup("LDAP_BIND_PASSWORD")
	if bindPassword == "" {
		return Config{}, errors.New("LDAP_BIND_PASSWORD is required")
	}
	rawTTL := valueOr(lookup, "SESSION_TTL", "8h")
	sessionTTL, err := time.ParseDuration(rawTTL)
	if err != nil || sessionTTL <= 0 {
		return Config{}, fmt.Errorf("SESSION_TTL must be a positive duration, got %q", rawTTL)
	}
	return Config{
		HTTPAddress:  valueOr(lookup, "HTTP_ADDR", ":8081"),
		LDAPURL:      valueOr(lookup, "LDAP_URL", "ldaps://localhost:636"),
		LDAPCAFile:   valueOr(lookup, "LDAP_CA_FILE", "certs/ca.pem"),
		BindDN:       valueOr(lookup, "LDAP_BIND_DN", "CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com"),
		BindPassword: bindPassword,
		BaseDN:       valueOr(lookup, "LDAP_BASE_DN", "DC=corp,DC=example,DC=com"),
		AdminGroup:   valueOr(lookup, "ADMIN_GROUP", "PanelAdmins"),
		SessionTTL:   sessionTTL,
	}, nil
}

func valueOr(lookup func(string) (string, bool), name, fallback string) string {
	if value, present := lookup(name); present && value != "" {
		return value
	}
	return fallback
}
