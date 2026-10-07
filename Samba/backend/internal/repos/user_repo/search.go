package user_repo

import (
	"fmt"
	"samba-admin/internal/db/ldap_db"
	"samba-admin/internal/domain/user"
)

func (r *Repo) FindByLogin(login string) (user.User, error) {
	const op string = "user_repo.FindByLogin"

	records, err := r.client.Search(ldap_db.SearchRequest{
		BaseDN:     "OU=Staff," + r.baseDN,
		Scope:      ldap_db.ScopeOneLevel,
		Filter:     ldap_db.FilterEquals("sAMAccountName", login),
		Attributes: userAttributes,
	})
	if err != nil {
		return user.User{}, fmt.Errorf("%s: %w", op, err)
	}

	lengthOfRecords := len(records)
	if lengthOfRecords < 1 {
		return user.User{}, fmt.Errorf("%s: %w", op, ErrUserNotFound)
	}

	if lengthOfRecords > 1 {
		return user.User{}, fmt.Errorf("%s: %w", op, ErrTooManyUsersByLogin)
	}

	var ur userRecord
	err = records[0].Unmarshal(&ur)

	if err != nil {
		return user.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return ur.convertToDomain()
}
