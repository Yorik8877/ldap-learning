// Package user_repo хранит пользователей как записи inetOrgPerson в ou=people.
package user_repo

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/directory"
	"ldap-admin/internal/domain/session"
	"ldap-admin/internal/domain/user"
	"ldap-admin/internal/repos/tree_layout"
)

var userAttributes = []string{"uid", "cn", "sn", "mail"}

type Repo struct {
	client *ldap_db.Client
	layout tree_layout.Layout
}

func New(client *ldap_db.Client, layout tree_layout.Layout) *Repo {
	return &Repo{client: client, layout: layout}
}

func (r *Repo) List(ctx context.Context) ([]user.User, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN:     r.layout.PeopleDN(),
		Scope:      ldap_db.ScopeOneLevel,
		Filter:     ldap_db.FilterEquals("objectClass", "inetOrgPerson"),
		Attributes: userAttributes,
	})
	if errors.Is(err, ldap_db.ErrNoSuchObject) {
		return nil, fmt.Errorf("people branch %s is missing, apply seed.ldif: %w", r.layout.PeopleDN(), err)
	}
	if err != nil {
		return nil, mapError(err)
	}
	users := make([]user.User, 0, len(entries))
	for _, entry := range entries {
		// Записи, заведённые в обход админки и не проходящие доменные правила,
		// в список не попадают — их видно в браузере дерева.
		converted, err := toUser(entry)
		if err != nil {
			continue
		}
		users = append(users, converted)
	}
	slices.SortFunc(users, func(left, right user.User) int {
		return strings.Compare(string(left.UID), string(right.UID))
	})
	return users, nil
}

func (r *Repo) Get(ctx context.Context, uid user.UID) (user.User, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: r.layout.UserDN(uid), Scope: ldap_db.ScopeBase, Attributes: userAttributes,
	})
	if err != nil {
		return user.User{}, mapError(err)
	}
	if len(entries) == 0 {
		return user.User{}, user.ErrNotFound
	}
	return toUser(entries[0])
}

func (r *Repo) Create(ctx context.Context, created user.User) error {
	attributes := map[string][]string{
		"objectClass": {"inetOrgPerson"},
		"uid":         {string(created.UID)},
		"cn":          {created.CommonName},
		"sn":          {created.Surname},
	}
	if len(created.Emails) > 0 {
		attributes["mail"] = created.Emails
	}
	return mapError(r.client.Add(ctx, r.layout.UserDN(created.UID), attributes))
}

// Update заменяет атрибуты целиком (modify replace). Пустой список mail удаляет атрибут.
func (r *Repo) Update(ctx context.Context, updated user.User) error {
	return mapError(r.client.Modify(ctx, r.layout.UserDN(updated.UID), []ldap_db.Change{
		{Operation: ldap_db.ChangeReplace, Attribute: "cn", Values: []string{updated.CommonName}},
		{Operation: ldap_db.ChangeReplace, Attribute: "sn", Values: []string{updated.Surname}},
		{Operation: ldap_db.ChangeReplace, Attribute: "mail", Values: updated.Emails},
	}))
}

func (r *Repo) SetPassword(ctx context.Context, uid user.UID, password string) error {
	return mapError(r.client.SetPassword(ctx, r.layout.UserDN(uid), password))
}

func (r *Repo) Delete(ctx context.Context, uid user.UID) error {
	return mapError(r.client.Delete(ctx, r.layout.UserDN(uid)))
}

// Verify проверяет пароль bind'ом пользователя. Сервер отвечает одинаково на неверный
// пароль и несуществующий DN — и это правильно: ответ не подсказывает, есть ли такой uid.
func (r *Repo) Verify(ctx context.Context, uid user.UID, password string) error {
	err := r.client.VerifyPassword(ctx, r.layout.UserDN(uid), password)
	if errors.Is(err, ldap_db.ErrInvalidCredentials) {
		return session.ErrInvalidCredentials
	}
	return mapError(err)
}

func toUser(entry ldap_db.Entry) (user.User, error) {
	uid, err := user.ParseUID(entry.First("uid"))
	if err != nil {
		return user.User{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	converted, err := user.New(uid, entry.First("cn"), entry.First("sn"), entry.Values("mail"))
	if err != nil {
		return user.User{}, fmt.Errorf("entry %s breaks domain rules: %v", entry.DN, err)
	}
	return converted, nil
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ldap_db.ErrNoSuchObject):
		return user.ErrNotFound
	case errors.Is(err, ldap_db.ErrAlreadyExists):
		return user.ErrAlreadyExists
	case errors.Is(err, ldap_db.ErrObjectClassViolation), errors.Is(err, ldap_db.ErrInvalidSyntax):
		return fmt.Errorf("%w: rejected by the directory schema", user.ErrInvalid)
	case errors.Is(err, ldap_db.ErrUnavailable):
		return fmt.Errorf("%w: %w", directory.ErrUnavailable, err)
	default:
		return err
	}
}
