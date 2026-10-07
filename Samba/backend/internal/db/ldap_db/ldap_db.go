package ldap_db

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/go-ldap/ldap/v3"
)

type connection struct {
	conn *ldap.Conn
}

type ConnProvider interface {
	Close() error
}

func Connect(bindDN, bindPassword, url, certPath string) (ConnProvider, error) {
	const op string = "ldap_db.Connect"

	tlsCfg, err := readCertificate(certPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	conn, err := ldap.DialURL(url, ldap.DialWithTLSConfig(tlsCfg))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	err = conn.Bind(bindDN, bindPassword)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%s: failed to bind service account: %w", op, err)
	}

	return &connection{conn: conn}, nil
}

func readCertificate(certPath string) (*tls.Config, error) {
	const op string = "ldap_db.readCertificate"
	rootCertPool := x509.NewCertPool()
	pem, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to read certificate: %w", op, err)
	}

	ok := rootCertPool.AppendCertsFromPEM(pem)
	if !ok {
		return nil, fmt.Errorf("%s: failed to append certificate to pool", op)
	}

	return &tls.Config{
		RootCAs: rootCertPool,
	}, nil
}

func (c *connection) Close() error {
	return c.conn.Close()
}
