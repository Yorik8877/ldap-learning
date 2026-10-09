package group_repo

import (
	"fmt"
	"samba-admin/internal/db/ldap_db"
	"samba-admin/internal/domain/group"
	"samba-admin/internal/domain/user"
)

func (r *Repo) List() ([]group.Summary, error) {
	const op string = "group_repo.List"

	records, err := r.client.Search(ldap_db.SearchRequest{
		BaseDN: "OU=Groups," + r.baseDN,
		Scope:  ldap_db.ScopeOneLevel,
		Filter: ldap_db.FilterEquals("objectClass", "group"),
		// Attributes: userAttributes,
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	summary := make([]group.Summary, 0, len(records))
	for _, record := range records {
		var gr groupRecord

		err = record.Unmarshal(&gr)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", op, err)
		}

		membersCount := len(gr.Members)

		groupSummary := group.Summary{
			Name:        gr.CN,
			Description: gr.Description,
			MemberCount: membersCount,
		}
		summary = append(summary, groupSummary)
	}

	return summary, nil
}

func (r *Repo) FindByName(name string) (group.Group, error) {
	const op string = "group_repo.FindByName"

	records, err := r.client.Search(ldap_db.SearchRequest{
		BaseDN: "OU=Groups," + r.baseDN,
		Scope:  ldap_db.ScopeOneLevel,
		Filter: ldap_db.FilterEquals("cn", name),
		// Attributes: userAttributes,
	})
	if err != nil {
		return group.Group{}, fmt.Errorf("%s: %w", op, err)
	}

	lengthOfRecords := len(records)
	if lengthOfRecords < 1 {
		return group.Group{}, fmt.Errorf("%s: %w", op, user.ErrNotFound)
	}

	if lengthOfRecords > 1 {
		return group.Group{}, fmt.Errorf("%s: %w", op, ErrTooManyGroupsByName)
	}

	return group.Group{}, nil
}
