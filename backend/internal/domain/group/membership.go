package group

// Membership — участие пользователя в группе так, как его видит каталог. В отличие от Group
// доменные правила здесь не проверяются: группа могла быть заведена в обход админки
// (например, cn=Developers), а знать о членстве в ней всё равно нужно — иначе не отличить
// единственного участника и не показать группы пользователя.
type Membership struct {
	Name        Name
	MemberCount int
}

func (m Membership) IsSole() bool {
	return m.MemberCount == 1
}
