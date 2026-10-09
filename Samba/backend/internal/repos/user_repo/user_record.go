package user_repo

import (
	"fmt"
	"samba-admin/internal/db/ldap_db"
	"samba-admin/internal/domain/user"
)

// userRecord — пользователь так, как он лежит в AD. Наружу из репозитория не выходит.
type userRecord struct {
	DN                 string   `ldap:"dn"`
	Login              string   `ldap:"sAMAccountName"`
	FirstName          string   `ldap:"givenName"`
	LastName           string   `ldap:"sn"`
	DisplayName        string   `ldap:"displayName"`
	Email              string   `ldap:"mail"`
	UserAccountControl int64    `ldap:"userAccountControl"`
	Groups             []string `ldap:"memberOf"`
}

// Атрибуты, которые просим у сервера. Должны совпадать с тегами выше.
var userAttributes = []string{
	"sAMAccountName",
	"givenName",
	"sn",
	"displayName",
	"mail",
	"userAccountControl",
	"memberOf",
}

const accountDisabledFlag = 2

func (ur *userRecord) convertToDomain() (user.User, error) {
	const op string = "user_repo.convertToDomain"

	groups, err := groupNames(ur.Groups)
	if err != nil {
		return user.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return user.User{
		Login:       ur.Login,
		FirstName:   ur.FirstName,
		LastName:    ur.LastName,
		DisplayName: ur.DisplayName,
		Email:       ur.Email,
		Enabled:     ur.UserAccountControl&accountDisabledFlag == 0,
		Groups:      groups,
	}, nil
}

func groupNames(dns []string) ([]string, error) {
	const op string = "user_repo.groupNames"

	names := make([]string, 0, len(dns))
	for _, dn := range dns {
		name, err := ldap_db.CommonName(dn)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}

		names = append(names, name)
	}
	return names, nil
}
