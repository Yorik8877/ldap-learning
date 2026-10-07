// Package config читает настройки из переменных окружения LDAP_ADMIN_*.
// Конфиг грузится один раз в composition root и дальше передаётся параметрами.
package config

import (
	"errors"
	"fmt"
	"time"
)

const envPrefix = "LDAP_ADMIN_"

type Config struct {
	HTTPAddress  string
	LDAPURL      string
	BindDN       string
	BindPassword string
	BaseDN       string
	AdminsGroup  string
	SessionTTL   time.Duration
}

func Load(lookup func(string) (string, bool)) (Config, error) {
	bindPassword, _ := lookup(envPrefix + "LDAP_BIND_PASSWORD")
	if bindPassword == "" {
		return Config{}, errors.New(envPrefix + "LDAP_BIND_PASSWORD is required")
	}
	rawTTL := valueOr(lookup, "SESSION_TTL", "8h")
	sessionTTL, err := time.ParseDuration(rawTTL)
	if err != nil || sessionTTL <= 0 {
		return Config{}, fmt.Errorf("%sSESSION_TTL must be a positive duration, got %q", envPrefix, rawTTL)
	}
	return Config{
		HTTPAddress:  valueOr(lookup, "HTTP_ADDRESS", ":8080"),
		LDAPURL:      valueOr(lookup, "LDAP_URL", "ldap://localhost:389"),
		BindDN:       valueOr(lookup, "LDAP_BIND_DN", "cn=admin,dc=example,dc=com"),
		BindPassword: bindPassword,
		BaseDN:       valueOr(lookup, "LDAP_BASE_DN", "dc=example,dc=com"),
		AdminsGroup:  valueOr(lookup, "ADMINS_GROUP", "admins"),
		SessionTTL:   sessionTTL,
	}, nil
}

func valueOr(lookup func(string) (string, bool), name, fallback string) string {
	if value, present := lookup(envPrefix + name); present && value != "" {
		return value
	}
	return fallback
}
