package ldap_db

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

func CommonName(dn string) (string, error) {
	const op string = "ldap_db.CommonName"

	ldapDN, err := ldap.ParseDN(dn)
	if err != nil {
		return "", fmt.Errorf("%s: %w", op, err)
	}

	// ParseDN не считает ошибкой пустую строку и строку из пробелов (в том числе Unicode-пробелов):
	// он возвращает DN без единой части. Поэтому пустоту проверяем по результату разбора, а не по входу.
	if len(ldapDN.RDNs) == 0 {
		return "", fmt.Errorf("%s: %w", op, ErrEmptyDNGiven)
	}

	firstRDN := ldapDN.RDNs[0]
	for _, attribute := range firstRDN.Attributes {
		if strings.EqualFold(attribute.Type, "CN") {
			return attribute.Value, nil
		}
	}

	return "", fmt.Errorf("%s: %w", op, ErrNoCommonName)
}
