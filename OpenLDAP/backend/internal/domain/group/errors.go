package group

import (
	"errors"
	"strings"
)

var (
	ErrNotFound      = errors.New("group not found")
	ErrAlreadyExists = errors.New("group already exists")
	ErrInvalid       = errors.New("invalid group data")
	ErrLastMember    = errors.New("cannot remove the last member of a group")
	ErrNotMember     = errors.New("user is not a member of the group")
	ErrAlreadyMember = errors.New("user is already a member of the group")
	ErrProtected     = errors.New("this group is protected")
)

// SoleMemberError — пользователь единственный участник этих групп. Удалять его нельзя:
// сервер удалил бы запись, а в группах осталась бы ссылка на несуществующий DN.
type SoleMemberError struct {
	Groups []Name
}

func (e *SoleMemberError) Error() string {
	names := make([]string, 0, len(e.Groups))
	for _, name := range e.Groups {
		names = append(names, string(name))
	}
	return "user is the only member of groups: " + strings.Join(names, ", ")
}
