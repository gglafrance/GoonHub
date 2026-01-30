package handler

import (
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Cookie configuration constants
const (
	AuthCookieName = "goonhub_auth"
	AuthCookiePath = "/"
)

type AuthHandler struct {
	AuthService   *core.AuthService
	UserService   *core.UserService
	TokenDuration time.Duration
	SecureCookies bool // Set to true in production (HTTPS only)
}

func NewAuthHandler(authService *core.AuthService, userService *core.UserService) *AuthHandler {
	return &AuthHandler{
		AuthService:   authService,
		UserService:   userService,
		TokenDuration: 24 * time.Hour, // Default, should match config
		SecureCookies: false,          // Will be set based on environment
	}
}

// NewAuthHandlerWithConfig creates an auth handler with explicit configuration
func NewAuthHandlerWithConfig(authService *core.AuthService, userService *core.UserService, tokenDuration time.Duration, secureCookies bool) *AuthHandler {
	return &AuthHandler{
		AuthService:   authService,
		UserService:   userService,
		TokenDuration: tokenDuration,
		SecureCookies: secureCookies,
	}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req request.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	token, user, err := h.AuthService.Login(req.Username, req.Password)
	if err != nil {
		// SECURITY: Return generic error to prevent user enumeration and timing attacks
		// Do not expose internal error details (lockout status, user existence, etc.)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Set HTTP-only secure cookie
	// SECURITY: Token is ONLY transmitted via cookie, never in response body
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AuthCookieName,
		Value:    token,
		Path:     AuthCookiePath,
		MaxAge:   int(h.TokenDuration.Seconds()),
		HttpOnly: true,                    // Prevent JavaScript access (XSS protection)
		Secure:   h.SecureCookies,         // Only send over HTTPS in production
		SameSite: http.SameSiteStrictMode, // CSRF protection
	})

	resp := response.AuthResponse{
		User: response.UserSummary{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
		},
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	userPayload, ok := user.(*core.UserPayload)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user data"})
		return
	}

	resp := response.UserSummary{
		ID:       userPayload.UserID,
		Username: userPayload.Username,
		Role:     userPayload.Role,
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	// Try to get token from cookie first, then from Authorization header
	token := ""
	if cookie, err := c.Cookie(AuthCookieName); err == nil && cookie != "" {
		token = cookie
	} else {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			token = strings.TrimPrefix(authHeader, "Bearer ")
			if token == authHeader {
				token = "" // Not a Bearer token
			}
		}
	}

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No token provided"})
		return
	}

	if err := h.AuthService.RevokeToken(token, "user logout"); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke token"})
		return
	}

	// Clear the auth cookie
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     AuthCookieName,
		Value:    "",
		Path:     AuthCookiePath,
		MaxAge:   -1, // Delete cookie immediately
		HttpOnly: true,
		Secure:   h.SecureCookies,
		SameSite: http.SameSiteStrictMode,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
