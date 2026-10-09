// Package group — группы безопасности панели. Name — cn группы.
package group

// Summary — группа в списке: без участников, только их число.
type Summary struct {
	Name        string
	Description string
	MemberCount int
}

// Group — группа с участниками.
type Group struct {
	Name        string
	Description string
	Members     []Member
}

// Member — участник группы. Login — sAMAccountName, у вложенной группы — её имя.
type Member struct {
	Login       string
	DisplayName string
}
