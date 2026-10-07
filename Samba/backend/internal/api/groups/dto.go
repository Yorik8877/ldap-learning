package groups

type CreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type SummaryResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"memberCount"`
}

type MemberResponse struct {
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
}

type DetailsResponse struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Members     []MemberResponse `json:"members"`
}
