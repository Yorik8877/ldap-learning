// Package tree_layout знает, где в дереве лежат пользователи и группы, и строит их DN.
// Это единственное место, где зашита раскладка каталога.
package tree_layout

import (
	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
)

type Layout struct {
	baseDN   string
	peopleDN string
	groupsDN string
}

func New(baseDN string) Layout {
	return Layout{
		baseDN:   baseDN,
		peopleDN: ldap_db.Join(ldap_db.RDN("ou", "people"), baseDN),
		groupsDN: ldap_db.Join(ldap_db.RDN("ou", "groups"), baseDN),
	}
}

func (l Layout) BaseDN() string   { return l.baseDN }
func (l Layout) PeopleDN() string { return l.peopleDN }
func (l Layout) GroupsDN() string { return l.groupsDN }

func (l Layout) UserDN(uid user.UID) string {
	return ldap_db.Join(ldap_db.RDN("uid", string(uid)), l.peopleDN)
}

func (l Layout) GroupDN(name group.Name) string {
	return ldap_db.Join(ldap_db.RDN("cn", string(name)), l.groupsDN)
}

func (l Layout) UIDOf(dn string) (user.UID, bool) {
	value, found := ldap_db.ChildValue(dn, "uid", l.peopleDN)
	if !found {
		return "", false
	}
	uid, err := user.ParseUID(value)
	if err != nil {
		return "", false
	}
	return uid, true
}
