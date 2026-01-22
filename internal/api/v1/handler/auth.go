package handler

import (
	"goonhub/internal/api/v1/request"
	"goonhub/internal/api/v1/response"
	"goonhub/internal/core"
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	AuthService *core.AuthService
	UserService *core.UserService
}

func NewAuthHandler(authService *core.AuthService, userService *core.UserService) *AuthHandler {
	return &AuthHandler{
		AuthService: authService,
		UserService: userService,
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
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	resp := response.AuthResponse{
		Token: token,
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
