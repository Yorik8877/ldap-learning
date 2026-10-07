package ldap_db

import (
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/go-ldap/ldap/v3"
)

type Client struct {
	mu           sync.Mutex
	ldapURL      string
	certPath     string
	bindDN       string
	bindPassword string
	conn         *ldap.Conn
}

func NewClient(
	ldapURL string,
	certPath string,
	bindDN string,
	bindPassword string,
) (*Client, error) {
	const op string = "ldap_db.NewClient"
	c := &Client{
		ldapURL:      ldapURL,
		certPath:     certPath,
		bindDN:       bindDN,
		bindPassword: bindPassword,
	}
	err := c.connect()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return c, nil
}

func (c *Client) connect() error {
	const op string = "ldap_db.connect"

	rootCertPool, err := readCertificate(c.certPath)
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	conn, err := ldap.DialURL(c.ldapURL, ldap.DialWithTLSConfig(&tls.Config{
		RootCAs: rootCertPool,
	}))
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	err = conn.Bind(c.bindDN, c.bindPassword)
	if err != nil {
		conn.Close()
		return fmt.Errorf("%s: failed to bind service account: %w", op, err)
	}

	c.conn = conn

	return nil
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}
