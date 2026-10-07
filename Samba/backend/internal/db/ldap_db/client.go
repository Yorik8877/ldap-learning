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
	tlsCfg       *tls.Config
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
	rootCertPool, err := readCertificate(c.certPath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	c.tlsCfg = &tls.Config{
		RootCAs: rootCertPool,
	}

	_, err = c.connection()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return c, nil
}

func (c *Client) dial(bindDN, bindPassword string) (*ldap.Conn, error) {
	const op string = "ldap_db.dial"

	conn, err := ldap.DialURL(c.ldapURL, ldap.DialWithTLSConfig(c.tlsCfg))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	err = conn.Bind(bindDN, bindPassword)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("%s: failed to bind: %w", op, err)
	}

	return conn, nil
}

func (c *Client) connection() (*ldap.Conn, error) {
	const op string = "ldap_db.connection"

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && !c.conn.IsClosing() {
		return c.conn, nil
	}

	conn, err := c.dial(c.bindDN, c.bindPassword)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to dial: %w", op, translateError(err))
	}

	c.conn = conn

	return conn, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}
