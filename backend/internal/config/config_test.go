package config_test

import (
	"testing"
	"time"

	"ldap-admin/internal/config"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	loaded, err := config.Load(lookupFrom(map[string]string{"LDAP_ADMIN_LDAP_BIND_PASSWORD": "admin"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := config.Config{
		HTTPAddress:  ":8080",
		LDAPURL:      "ldap://localhost:389",
		BindDN:       "cn=admin,dc=example,dc=com",
		BindPassword: "admin",
		BaseDN:       "dc=example,dc=com",
		AdminsGroup:  "admins",
		SessionTTL:   8 * time.Hour,
	}
	if loaded != want {
		t.Fatalf("Load() = %+v, want %+v", loaded, want)
	}
}

func TestLoadRequiresBindPassword(t *testing.T) {
	if _, err := config.Load(lookupFrom(map[string]string{})); err == nil {
		t.Fatalf("Load() without bind password succeeded")
	}
}

func TestLoadRejectsBadSessionTTL(t *testing.T) {
	for _, ttl := range []string{"soon", "0s", "-1h"} {
		_, err := config.Load(lookupFrom(map[string]string{
			"LDAP_ADMIN_LDAP_BIND_PASSWORD": "admin",
			"LDAP_ADMIN_SESSION_TTL":        ttl,
		}))
		if err == nil {
			t.Errorf("Load() accepted session TTL %q", ttl)
		}
	}
}
