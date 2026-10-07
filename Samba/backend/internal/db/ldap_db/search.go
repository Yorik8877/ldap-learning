package ldap_db

import (
	"fmt"

	"github.com/go-ldap/ldap/v3"
)

func (c *Client) Search(request SearchRequest) ([]*Entry, error) {
	const op string = "ldap_db.Search"

	libSearchRequest, err := request.toLibraryRequest()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	conn, err := c.connection()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	libSearchResult, err := conn.Search(libSearchRequest)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, translateError(err))
	}

	entries := make([]*Entry, 0, len(libSearchResult.Entries))
	for _, libEntry := range libSearchResult.Entries {
		entries = append(entries, &Entry{entry: libEntry})
	}

	return entries, nil
}

func FilterEquals(attribute, value string) string {
	return "(" + attribute + "=" + ldap.EscapeFilter(value) + ")"
}
