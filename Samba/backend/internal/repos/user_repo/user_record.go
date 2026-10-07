package user_repo

// userRecord — пользователь так, как он лежит в AD. Наружу из репозитория не выходит.
type userRecord struct {
	DN                 string   `ldap:"dn"`
	Login              string   `ldap:"sAMAccountName"`
	FirstName          string   `ldap:"givenName"`
	LastName           string   `ldap:"sn"`
	DisplayName        string   `ldap:"displayName"`
	Email              string   `ldap:"mail"`
	UserAccountControl int64    `ldap:"userAccountControl"`
	Groups             []string `ldap:"memberOf"`
}

// Атрибуты, которые просим у сервера. Должны совпадать с тегами выше.
var userAttributes = []string{
	"sAMAccountName",
	"givenName",
	"sn",
	"displayName",
	"mail",
	"userAccountControl",
	"memberOf",
}
