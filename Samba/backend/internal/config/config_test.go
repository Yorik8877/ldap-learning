package config_test

import (
	"testing"
	"time"

	"samba-admin/internal/config"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	loaded, err := config.Load(lookupFrom(map[string]string{"LDAP_BIND_PASSWORD": "secret"}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	want := config.Config{
		HTTPAddress:  ":8081",
		LDAPURL:      "ldaps://localhost:636",
		LDAPCAFile:   "certs/ca.pem",
		BindDN:       "CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com",
		BindPassword: "secret",
		BaseDN:       "DC=corp,DC=example,DC=com",
		AdminGroup:   "PanelAdmins",
		SessionTTL:   8 * time.Hour,
	}
	if loaded != want {
		t.Fatalf("Load() = %+v, want %+v", loaded, want)
	}
}

func TestLoadReadsOverrides(t *testing.T) {
	loaded, err := config.Load(lookupFrom(map[string]string{
		"LDAP_BIND_PASSWORD": "secret",
		"HTTP_ADDR":          ":9090",
		"ADMIN_GROUP":        "Operators",
		"SESSION_TTL":        "30m",
	}))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.HTTPAddress != ":9090" || loaded.AdminGroup != "Operators" || loaded.SessionTTL != 30*time.Minute {
		t.Fatalf("Load() = %+v, overrides not applied", loaded)
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
			"LDAP_BIND_PASSWORD": "secret",
			"SESSION_TTL":        ttl,
		}))
		if err == nil {
			t.Errorf("Load() accepted session TTL %q", ttl)
		}
	}
}
