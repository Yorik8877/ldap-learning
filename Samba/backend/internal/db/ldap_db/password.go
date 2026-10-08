package ldap_db

import "fmt"

func (c *Client) VerifyPassword(bindDN, password string) error {
	const op string = "ldap_db.VerifyPassword"

	conn, err := c.dial(bindDN, password)
	if err != nil {
		return fmt.Errorf("%s: failed to dial: %w", op, translateError(err))
	}
	defer conn.Close()

	return nil
}
