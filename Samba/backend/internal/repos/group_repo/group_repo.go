package group_repo

import "samba-admin/internal/db/ldap_db"

type directory interface {
	Search(request ldap_db.SearchRequest) ([]*ldap_db.Entry, error)
}

type Repo struct {
	baseDN string
	client directory
}

func New(baseDN string, client directory) *Repo {
	return &Repo{
		baseDN: baseDN,
		client: client,
	}
}
