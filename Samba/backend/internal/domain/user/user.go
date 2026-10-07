package user

type User struct {
	Login       string
	FirstName   string
	LastName    string
	DisplayName string
	Email       string
	Enabled     bool
	Groups      []string
}
