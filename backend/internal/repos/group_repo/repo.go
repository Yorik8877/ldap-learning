// Package group_repo хранит группы как записи groupOfUniqueNames в ou=groups.
// Выбран именно этот класс: оверлей memberof образа osixia настроен на него и ведёт
// у участников атрибут memberOf.
package group_repo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/group"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/tree_layout"
)

const membersAttribute = "uniqueMember"

var groupAttributes = []string{"cn", "description", membersAttribute}

type Repo struct {
	client *ldap_db.Client
	layout tree_layout.Layout
}

func New(client *ldap_db.Client, layout tree_layout.Layout) *Repo {
	return &Repo{client: client, layout: layout}
}

func (r *Repo) List(ctx context.Context) ([]group.Group, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN:     r.layout.GroupsDN(),
		Scope:      ldap_db.ScopeOneLevel,
		Filter:     ldap_db.FilterEquals("objectClass", "groupOfUniqueNames"),
		Attributes: groupAttributes,
	})
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return nil, fmt.Errorf("groups branch %s is missing, apply seed.ldif: %w", r.layout.GroupsDN(), err)
	}
	if err != nil {
		return nil, mapError(err)
	}
	groups := make([]group.Group, 0, len(entries))
	for _, entry := range entries {
		converted, err := r.toGroup(entry)
		if err != nil {
			continue
		}
		groups = append(groups, converted)
	}
	slices.SortFunc(groups, func(left, right group.Group) int {
		return strings.Compare(string(left.Name), string(right.Name))
	})
	return groups, nil
}

func (r *Repo) Get(ctx context.Context, name group.Name) (group.Group, error) {
	return r.getByDN(ctx, r.layout.GroupDN(name))
}

func (r *Repo) Create(ctx context.Context, created group.Group) error {
	memberDNs := make([]string, 0, len(created.Members))
	for _, member := range created.Members {
		memberDNs = append(memberDNs, r.memberDN(member))
	}
	attributes := map[string][]string{
		"objectClass":    {"groupOfUniqueNames"},
		"cn":             {string(created.Name)},
		membersAttribute: memberDNs,
	}
	if created.Description != "" {
		attributes["description"] = []string{created.Description}
	}
	return mapError(r.client.Add(ctx, r.layout.GroupDN(created.Name), attributes))
}

func (r *Repo) Delete(ctx context.Context, name group.Name) error {
	return mapError(r.client.Delete(ctx, r.layout.GroupDN(name)))
}

// AddMember добавляет одно значение (modify add), не трогая остальных участников.
func (r *Repo) AddMember(ctx context.Context, name group.Name, uid user.UID) error {
	err := r.client.Modify(ctx, r.layout.GroupDN(name), []ldap_db.Change{
		{Operation: ldap_db.ChangeAdd, Attribute: membersAttribute, Values: []string{r.layout.UserDN(uid)}},
	})
	if errors.Is(err, ldap_db.ErrValueExists) {
		return group.ErrAlreadyMember
	}
	return mapError(err)
}

func (r *Repo) RemoveMember(ctx context.Context, name group.Name, uid user.UID) error {
	err := r.client.Modify(ctx, r.layout.GroupDN(name), []ldap_db.Change{
		{Operation: ldap_db.ChangeDelete, Attribute: membersAttribute, Values: []string{r.layout.UserDN(uid)}},
	})
	switch {
	case errors.Is(err, ldap_db.ErrNoSuchValue):
		return group.ErrNotMember
	case errors.Is(err, ldap_db.ErrObjectClassViolation):
		return group.ErrLastMember
	default:
		return mapError(err)
	}
}

// GroupsOf читает memberOf пользователя: этот служебный атрибут ведёт оверлей memberof,
// и в обычном поиске он не возвращается — его надо запросить по имени.
func (r *Repo) GroupsOf(ctx context.Context, uid user.UID) ([]group.Group, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: r.layout.UserDN(uid), Scope: ldap_db.ScopeBase, Attributes: []string{"memberOf"},
	})
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return nil, user.ErrNotFound
	}
	if err != nil {
		return nil, mapError(err)
	}
	if len(entries) == 0 {
		return nil, user.ErrNotFound
	}
	return r.groupsByDN(ctx, entries[0].Values("memberOf"))
}

// IsMember использует операцию Compare: сервер отвечает «да/нет», не возвращая записей.
// Если группы нет, участников у неё тоже нет.
func (r *Repo) IsMember(ctx context.Context, name group.Name, uid user.UID) (bool, error) {
	matched, err := r.client.Compare(ctx, r.layout.GroupDN(name), membersAttribute, r.layout.UserDN(uid))
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return false, nil
	}
	return matched, mapError(err)
}

func (r *Repo) groupsByDN(ctx context.Context, groupDNs []string) ([]group.Group, error) {
	groups := make([]group.Group, 0, len(groupDNs))
	for _, groupDN := range groupDNs {
		found, err := r.getByDN(ctx, groupDN)
		if errors.Is(err, group.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		groups = append(groups, found)
	}
	return groups, nil
}

func (r *Repo) getByDN(ctx context.Context, dn string) (group.Group, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: dn, Scope: ldap_db.ScopeBase, Attributes: groupAttributes,
	})
	if err != nil {
		return group.Group{}, mapError(err)
	}
	if len(entries) == 0 {
		return group.Group{}, group.ErrNotFound
	}
	return r.toGroup(entries[0])
}

func (r *Repo) toGroup(entry ldap_db.Entry) (group.Group, error) {
	name, err := group.ParseName(entry.First("cn"))
	if err != nil {
		return group.Group{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	memberDNs := entry.Values(membersAttribute)
	members := make([]group.Member, 0, len(memberDNs))
	for _, memberDN := range memberDNs {
		uid, _ := r.layout.UIDOf(memberDN)
		members = append(members, group.Member{DN: directory.DN(memberDN), UID: uid})
	}
	converted, err := group.New(name, entry.First("description"), members)
	if err != nil {
		return group.Group{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	return converted, nil
}

func (r *Repo) memberDN(member group.Member) string {
	if member.UID != "" {
		return r.layout.UserDN(member.UID)
	}
	return string(member.DN)
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ldap_db.ErrNoSuchObject):
		return group.ErrNotFound
	case errors.Is(err, ldap_db.ErrAlreadyExists):
		return group.ErrAlreadyExists
	case errors.Is(err, ldap_db.ErrObjectClassViolation):
		return fmt.Errorf("%w: rejected by the directory schema", group.ErrInvalid)
	case errors.Is(err, ldap_db.ErrUnavailable):
		return fmt.Errorf("%w: %w", directory.ErrUnavailable, err)
	default:
		return err
	}
}
