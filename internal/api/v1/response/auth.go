package response

type AuthResponse struct {
	Token string      `json:"token"`
	User  UserSummary `json:"user"`
}

type UserSummary struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}
