package users

type CreateRequest struct {
	Login       string `json:"login"`
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type UpdateRequest struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
}

type PasswordRequest struct {
	Password string `json:"password"`
}

// EnabledRequest.Enabled — указатель, чтобы отличить «поле не передано» от false.
type EnabledRequest struct {
	Enabled *bool `json:"enabled"`
}

type SummaryResponse struct {
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
	Email       string `json:"email"`
	Enabled     bool   `json:"enabled"`
}

type DetailsResponse struct {
	Login       string   `json:"login"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	DisplayName string   `json:"displayName"`
	Email       string   `json:"email"`
	Enabled     bool     `json:"enabled"`
	Groups      []string `json:"groups"`
}
