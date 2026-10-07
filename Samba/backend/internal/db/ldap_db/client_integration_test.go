//go:build integration

// Интеграционные тесты клиента против Samba из Samba/docker-compose.yml.
// Запуск: make test-integration. Без запущенной Samba или без certs/ca.pem тесты пропускаются.
package ldap_db

import (
	"errors"
	"io/fs"
	"os"
	"sync"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

const concurrentCallers = 20

type standSettings struct {
	url          string
	caFile       string
	bindDN       string
	bindPassword string
}

func settingOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func loadStandSettings() standSettings {
	return standSettings{
		url:          settingOr("LDAP_URL", "ldaps://localhost:636"),
		caFile:       settingOr("LDAP_CA_FILE", "../../../certs/ca.pem"),
		bindDN:       settingOr("LDAP_BIND_DN", "CN=svc-panel,CN=Users,DC=corp,DC=example,DC=com"),
		bindPassword: settingOr("LDAP_BIND_PASSWORD", "Svc-Panel-Passw0rd"),
	}
}

func newStandClient(t *testing.T) *Client {
	t.Helper()
	settings := loadStandSettings()
	client, err := NewClient(settings.url, settings.caFile, settings.bindDN, settings.bindPassword)
	if errors.Is(err, fs.ErrNotExist) {
		t.Skipf("CA file %s not found, run Samba/seed.sh: %v", settings.caFile, err)
	}
	if ldap.IsErrorWithCode(err, ldap.ErrorNetwork) {
		t.Skipf("Samba is not reachable at %s: %v", settings.url, err)
	}
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

func authorizationID(t *testing.T, connection *ldap.Conn) string {
	t.Helper()
	result, err := connection.WhoAmI(nil)
	if err != nil {
		t.Fatalf("WhoAmI() error = %v", err)
	}
	return result.AuthzID
}

func TestConnectionIsReusedWhileAlive(t *testing.T) {
	client := newStandClient(t)

	first, err := client.connection()
	if err != nil {
		t.Fatalf("connection() error = %v", err)
	}
	second, err := client.connection()
	if err != nil {
		t.Fatalf("connection() error = %v", err)
	}

	if first != second {
		t.Fatalf("connection() replaced a live connection")
	}
}

// Двадцать горутин одновременно просят соединение после того, как старое умерло:
// переподключение должно случиться ровно одно. Ловит проверку c.conn вне мьютекса
// (с -race) и потерю соединения, если новое не сохраняется в c.conn.
func TestConcurrentCallersShareOneReconnect(t *testing.T) {
	client := newStandClient(t)
	dead, err := client.connection()
	if err != nil {
		t.Fatalf("connection() error = %v", err)
	}
	dead.Close()

	var wait sync.WaitGroup
	received := make([]*ldap.Conn, concurrentCallers)
	for callerIndex := range received {
		wait.Add(1)
		go func() {
			defer wait.Done()
			connection, err := client.connection()
			if err != nil {
				t.Errorf("connection() error = %v", err)
			}
			received[callerIndex] = connection
		}()
	}
	wait.Wait()

	for _, connection := range received {
		if connection != received[0] {
			t.Fatalf("concurrent callers got different connections: reconnected more than once")
		}
	}
	if received[0] == dead {
		t.Fatalf("connection() returned the dead connection")
	}
	if identity := authorizationID(t, received[0]); identity != `u:CORP\svc-panel` {
		t.Fatalf("reconnected as %q, want u:CORP\\svc-panel", identity)
	}
}

// Неудачный bind при переподключении не должен оставлять мьютекс занятым:
// раньше dial закрывал соединение через c.Close() и брал уже захваченный c.mu.
func TestFailedReconnectReleasesLock(t *testing.T) {
	client := newStandClient(t)
	dead, err := client.connection()
	if err != nil {
		t.Fatalf("connection() error = %v", err)
	}
	dead.Close()

	correctPassword := client.bindPassword
	client.bindPassword = "wrong-password"
	_, err = client.connection()
	if !ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
		t.Fatalf("connection() with wrong password error = %v, want code 49", err)
	}

	client.bindPassword = correctPassword
	if _, err := client.connection(); err != nil {
		t.Fatalf("connection() after failed reconnect error = %v", err)
	}
}
