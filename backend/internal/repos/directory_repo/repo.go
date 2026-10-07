// Package directory_repo читает произвольные записи дерева для браузера каталога.
package directory_repo

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"ldap-admin/internal/db/ldap_db"
	"ldap-admin/internal/domain/directory"
)

const (
	// "*" — все обычные атрибуты, "+" — все служебные (RFC 3673). Две выборки вместо
	// одной нужны, чтобы знать, какие атрибуты ведёт сервер.
	allUserAttributes        = "*"
	allOperationalAttributes = "+"
	passwordAttribute        = "userPassword"
)

type Repo struct {
	client *ldap_db.Client
	baseDN string
}

func New(client *ldap_db.Client, baseDN string) *Repo {
	return &Repo{client: client, baseDN: baseDN}
}

func (r *Repo) Children(ctx context.Context, dn *directory.DN) ([]directory.Node, error) {
	parent, err := r.resolve(dn)
	if err != nil {
		return nil, err
	}
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: parent, Scope: ldap_db.ScopeOneLevel, Attributes: []string{"objectClass", "hasSubordinates"},
	})
	if err != nil {
		return nil, mapError(err)
	}
	nodes := make([]directory.Node, 0, len(entries))
	for _, entry := range entries {
		node, err := toNode(entry)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}
	slices.SortFunc(nodes, func(left, right directory.Node) int { return strings.Compare(left.RDN, right.RDN) })
	return nodes, nil
}

func (r *Repo) Entry(ctx context.Context, dn *directory.DN) (directory.Entry, error) {
	target, err := r.resolve(dn)
	if err != nil {
		return directory.Entry{}, err
	}
	attributes, err := r.readAttributes(ctx, target, allUserAttributes)
	if err != nil {
		return directory.Entry{}, err
	}
	operational, err := r.readAttributes(ctx, target, allOperationalAttributes)
	if err != nil {
		return directory.Entry{}, err
	}
	maps.DeleteFunc(attributes, func(name string, _ []string) bool {
		return strings.EqualFold(name, passwordAttribute)
	})
	return directory.Entry{DN: directory.DN(target), Attributes: attributes, OperationalAttributes: operational}, nil
}

func (r *Repo) readAttributes(ctx context.Context, dn, selector string) (map[string][]string, error) {
	entries, err := r.client.Search(ctx, ldap_db.SearchRequest{
		BaseDN: dn, Scope: ldap_db.ScopeBase, Attributes: []string{selector},
	})
	if err != nil {
		return nil, mapError(err)
	}
	if len(entries) == 0 {
		return nil, directory.ErrNotFound
	}
	return entries[0].Attributes, nil
}

// resolve не выпускает браузер за пределы каталога приложения.
func (r *Repo) resolve(dn *directory.DN) (string, error) {
	if dn == nil {
		return r.baseDN, nil
	}
	within, err := ldap_db.IsWithin(string(*dn), r.baseDN)
	if errors.Is(err, ldap_db.ErrInvalidDN) {
		return "", directory.ErrInvalidDN
	}
	if err != nil {
		return "", err
	}
	if !within {
		return "", directory.ErrOutsideBase
	}
	return string(*dn), nil
}

func toNode(entry ldap_db.Entry) (directory.Node, error) {
	rdn, err := ldap_db.FirstRDN(entry.DN)
	if err != nil {
		return directory.Node{}, fmt.Errorf("entry %s: %v", entry.DN, err)
	}
	return directory.Node{
		DN:            directory.DN(entry.DN),
		RDN:           rdn,
		ObjectClasses: entry.Values("objectClass"),
		HasChildren:   strings.EqualFold(entry.First("hasSubordinates"), "TRUE"),
	}, nil
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ldap_db.ErrNoSuchObject):
		return directory.ErrNotFound
	case errors.Is(err, ldap_db.ErrInvalidDN):
		return directory.ErrInvalidDN
	case errors.Is(err, ldap_db.ErrUnavailable):
		return fmt.Errorf("%w: %w", directory.ErrUnavailable, err)
	default:
		return err
	}
}
