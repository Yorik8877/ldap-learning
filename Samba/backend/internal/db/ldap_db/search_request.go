package ldap_db

import "github.com/go-ldap/ldap/v3"

type Scope int

const (
	ScopeBase     Scope = iota // только сама запись (ldap.ScopeBaseObject)
	ScopeOneLevel              // прямые потомки (ldap.ScopeSingleLevel)
	ScopeSubtree               // всё поддерево (ldap.ScopeWholeSubtree)
)

type SearchRequest struct {
	BaseDN     string
	Scope      Scope
	Filter     string
	Attributes []string
}

func (sr *SearchRequest) toLibraryRequest() (*ldap.SearchRequest, error) {
	libScope, err := sr.Scope.toLibraryScope()
	if err != nil {
		return nil, err
	}
	return ldap.NewSearchRequest(
		sr.BaseDN,
		libScope,
		ldap.NeverDerefAliases, // В AD алиасов нет
		0,
		0,
		false,
		sr.Filter,
		sr.Attributes,
		nil,
	), nil
}

func (s Scope) toLibraryScope() (int, error) {
	switch s {
	case ScopeBase:
		return ldap.ScopeBaseObject, nil
	case ScopeOneLevel:
		return ldap.ScopeSingleLevel, nil
	case ScopeSubtree:
		return ldap.ScopeWholeSubtree, nil
	}

	return -1, ErrUnknownScope
}
