package group

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/user"
)

const maxDescriptionLength = 1024

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)

type Name string

func ParseName(raw string) (Name, error) {
	if !namePattern.MatchString(raw) {
		return "", fmt.Errorf("%w: group name must start with a lowercase letter and contain only a-z, 0-9, '.', '_', '-' (up to 64 characters)", ErrInvalid)
	}
	return Name(raw), nil
}

type Member struct {
	DN  directory.DN
	UID user.UID
}

type Group struct {
	Name        Name
	Description string
	Members     []Member
}

// New требует хотя бы одного участника: в groupOfUniqueNames атрибут uniqueMember
// обязателен, сервер не примет группу без него.
func New(name Name, description string, members []Member) (Group, error) {
	if _, err := ParseName(string(name)); err != nil {
		return Group{}, err
	}
	description = strings.TrimSpace(description)
	if utf8.RuneCountInString(description) > maxDescriptionLength {
		return Group{}, fmt.Errorf("%w: description is longer than %d characters", ErrInvalid, maxDescriptionLength)
	}
	if len(members) == 0 {
		return Group{}, fmt.Errorf("%w: a group needs at least one member", ErrInvalid)
	}
	if err := validateMembers(members); err != nil {
		return Group{}, err
	}
	return Group{Name: name, Description: description, Members: slices.Clone(members)}, nil
}

func (g Group) HasMember(uid user.UID) bool {
	return slices.ContainsFunc(g.Members, func(member Member) bool { return member.UID == uid })
}

func (g Group) IsSoleMember(uid user.UID) bool {
	return len(g.Members) == 1 && g.HasMember(uid)
}

func (g Group) CheckRemoval(uid user.UID) error {
	if !g.HasMember(uid) {
		return ErrNotMember
	}
	if len(g.Members) == 1 {
		return ErrLastMember
	}
	return nil
}

func validateMembers(members []Member) error {
	seen := make(map[Member]bool, len(members))
	for _, member := range members {
		if member.UID == "" && member.DN == "" {
			return fmt.Errorf("%w: a member needs a uid or a dn", ErrInvalid)
		}
		if seen[member] {
			return fmt.Errorf("%w: member %s is listed twice", ErrInvalid, describe(member))
		}
		seen[member] = true
	}
	return nil
}

func describe(member Member) string {
	if member.UID != "" {
		return string(member.UID)
	}
	return string(member.DN)
}
