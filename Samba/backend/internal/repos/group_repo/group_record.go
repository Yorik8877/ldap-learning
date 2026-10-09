package group_repo

import (
	"samba-admin/internal/domain/group"
)

type groupRecord struct {
	DN          string   `ldap:"dn"`
	CN          string   `ldap:"cn"`
	Description string   `ldap:"description"`
	Members     []string `ldap:"member"`
}

func (gr *groupRecord) convertToDomain() (group.Group, error) {
	const op string = "group_repo.convertToDomain"

	return group.Group{}, nil
}
