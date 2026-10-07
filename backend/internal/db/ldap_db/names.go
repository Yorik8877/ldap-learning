package ldap_db

import (
	"fmt"
	"strings"

	"github.com/go-ldap/ldap/v3"
)

// FilterEquals строит фильтр равенства. Значение приходит снаружи: без экранирования
// символы `*`, `(` и `)` изменили бы смысл фильтра (LDAP-инъекция).
func FilterEquals(attribute, value string) string {
	return "(" + attribute + "=" + ldap.EscapeFilter(value) + ")"
}

func RDN(attribute, value string) string {
	return attribute + "=" + ldap.EscapeDN(value)
}

func Join(rdn, parentDN string) string {
	return rdn + "," + parentDN
}

// IsWithin сообщает, совпадает ли dn с base или лежит под ним. Регистр не важен:
// для LDAP «DC=Example» и «dc=example» — одно и то же имя.
func IsWithin(dn, base string) (bool, error) {
	parsedDN, err := parseDN(dn)
	if err != nil {
		return false, err
	}
	parsedBase, err := parseDN(base)
	if err != nil {
		return false, err
	}
	if len(parsedDN.RDNs) == 0 {
		return false, nil
	}
	return parsedBase.EqualFold(parsedDN) || parsedBase.AncestorOfFold(parsedDN), nil
}

func FirstRDN(dn string) (string, error) {
	parsed, err := parseDN(dn)
	if err != nil {
		return "", err
	}
	if len(parsed.RDNs) == 0 {
		return "", nil
	}
	return parsed.RDNs[0].String(), nil
}

// ChildValue возвращает значение из RDN вида attribute=value, если dn — прямой потомок
// parentDN. Так из DN участника группы получают uid пользователя.
func ChildValue(dn, attribute, parentDN string) (string, bool) {
	parsedDN, err := parseDN(dn)
	if err != nil {
		return "", false
	}
	parsedParent, err := parseDN(parentDN)
	if err != nil {
		return "", false
	}
	isDirectChild := len(parsedDN.RDNs) == len(parsedParent.RDNs)+1 && parsedParent.AncestorOfFold(parsedDN)
	if !isDirectChild {
		return "", false
	}
	firstRDN := parsedDN.RDNs[0].Attributes
	if len(firstRDN) != 1 || !strings.EqualFold(firstRDN[0].Type, attribute) {
		return "", false
	}
	return firstRDN[0].Value, true
}

func parseDN(dn string) (*ldap.DN, error) {
	parsed, err := ldap.ParseDN(dn)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidDN, err)
	}
	return parsed, nil
}
