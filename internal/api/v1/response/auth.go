package response

// AuthResponse is returned after successful login
// SECURITY: Token is only transmitted via HTTP-only cookie, never in response body
type AuthResponse struct {
	User UserSummary `json:"user"`
}

type UserSummary struct {
	ID       uint   `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}
