package user_repo

import (
	"fmt"
	"samba-admin/internal/domain/user"
)

func (r *Repo) Authenticate(login, password string) (user.User, error) {
	const op string = "user_repo.Authenticate"

	record, err := r.findRecordByLogin(login)
	if err != nil {
		return user.User{}, fmt.Errorf("%s: %w", op, err)
	}

	err = r.client.VerifyPassword(record.DN, password)
	if err != nil {
		return user.User{}, fmt.Errorf("%s: %w", op, r.translateError(err))
	}

	return record.convertToDomain()
}
