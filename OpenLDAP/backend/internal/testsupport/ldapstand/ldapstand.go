// Package ldapstand даёт интеграционным тестам LDAP из docker-compose.yml: клиент
// сервисного аккаунта и временную ветку дерева, которая удаляется после теста.
package ldapstand

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"slices"
	"testing"

	"ldap-admin/internal/db/ldap_db"
)

const (
	defaultURL          = "ldap://localhost:389"
	defaultBindDN       = "cn=admin,dc=example,dc=com"
	defaultBindPassword = "admin"
	defaultBaseDN       = "dc=example,dc=com"
	suffixBytes         = 4
)

type Stand struct {
	Client *ldap_db.Client
	Config ldap_db.Config
	// Base — временная ветка ou=test-<random> под base DN каталога.
	Base string
}

func Connect(t *testing.T) Stand {
	t.Helper()
	config := ldap_db.Config{
		URL:          valueOr("LDAP_ADMIN_TEST_LDAP_URL", defaultURL),
		BindDN:       valueOr("LDAP_ADMIN_TEST_LDAP_BIND_DN", defaultBindDN),
		BindPassword: valueOr("LDAP_ADMIN_TEST_LDAP_BIND_PASSWORD", defaultBindPassword),
	}
	directoryBase := valueOr("LDAP_ADMIN_TEST_LDAP_BASE_DN", defaultBaseDN)
	client := ldap_db.New(config)

	_, err := client.Search(t.Context(), ldap_db.SearchRequest{BaseDN: directoryBase, Scope: ldap_db.ScopeBase})
	if errors.Is(err, ldap_db.ErrUnavailable) {
		t.Skipf("LDAP is not reachable at %s: %v", config.URL, err)
	}
	if err != nil {
		t.Fatalf("probe LDAP: %v", err)
	}

	name := "test-" + randomSuffix(t)
	base := ldap_db.Join(ldap_db.RDN("ou", name), directoryBase)
	stand := Stand{Client: client, Config: config, Base: base}
	addEntry(t, client, base, map[string][]string{"objectClass": {"organizationalUnit"}, "ou": {name}})
	t.Cleanup(func() { removeSubtree(t, client, base) })
	return stand
}

func (s Stand) CreateOU(t *testing.T, name string) string {
	t.Helper()
	dn := ldap_db.Join(ldap_db.RDN("ou", name), s.Base)
	addEntry(t, s.Client, dn, map[string][]string{"objectClass": {"organizationalUnit"}, "ou": {name}})
	return dn
}

func (s Stand) CreatePerson(t *testing.T, parentDN, uid string) string {
	t.Helper()
	dn := ldap_db.Join(ldap_db.RDN("uid", uid), parentDN)
	addEntry(t, s.Client, dn, map[string][]string{
		"objectClass": {"inetOrgPerson"}, "uid": {uid}, "cn": {uid}, "sn": {uid},
	})
	return dn
}

func (s Stand) CreateGroup(t *testing.T, parentDN, name string, memberDNs ...string) string {
	t.Helper()
	dn := ldap_db.Join(ldap_db.RDN("cn", name), parentDN)
	addEntry(t, s.Client, dn, map[string][]string{
		"objectClass": {"groupOfUniqueNames"}, "cn": {name}, "uniqueMember": memberDNs,
	})
	return dn
}

func addEntry(t *testing.T, client *ldap_db.Client, dn string, attributes map[string][]string) {
	t.Helper()
	if err := client.Add(t.Context(), dn, attributes); err != nil {
		t.Fatalf("add %s: %v", dn, err)
	}
}

// removeSubtree удаляет ветку снизу вверх: LDAP не удаляет запись, у которой есть потомки.
// DN потомка всегда длиннее DN предка, поэтому сортировки по длине достаточно.
// t.Context() к моменту Cleanup уже отменён, поэтому контекст — фоновый.
func removeSubtree(t *testing.T, client *ldap_db.Client, base string) {
	ctx := context.Background()
	entries, err := client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: base, Scope: ldap_db.ScopeSubtree, Attributes: []string{"1.1"},
	})
	if err != nil {
		t.Logf("cleanup search %s: %v", base, err)
		return
	}
	slices.SortFunc(entries, func(left, right ldap_db.Entry) int {
		return cmp.Compare(len(right.DN), len(left.DN))
	})
	for _, entry := range entries {
		if err := client.Delete(ctx, entry.DN); err != nil {
			t.Logf("cleanup delete %s: %v", entry.DN, err)
		}
	}
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	buffer := make([]byte, suffixBytes)
	if _, err := rand.Read(buffer); err != nil {
		t.Fatalf("random suffix: %v", err)
	}
	return hex.EncodeToString(buffer)
}

func valueOr(name, fallback string) string {
	if value, present := os.LookupEnv(name); present && value != "" {
		return value
	}
	return fallback
}
