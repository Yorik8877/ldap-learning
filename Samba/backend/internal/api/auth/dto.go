package auth

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type CurrentUserResponse struct {
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
}
