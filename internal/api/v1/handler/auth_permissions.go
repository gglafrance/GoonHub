package handler

import (
	"net/http"

	"goonhub/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// RolePermissionsProvider lists the permissions granted to a role (implemented by core.RBACService).
type RolePermissionsProvider interface {
	PermissionsForRole(role string) []string
}

// MyPermissionsResponse is the body of GET /api/v1/auth/permissions.
type MyPermissionsResponse struct {
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// MyPermissions returns the permissions of the authenticated user's role, so that the
// frontend can hide or disable the actions the user is not allowed to perform. The API
// still enforces every permission on its own routes.
func MyPermissions(provider RolePermissionsProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		payload, err := middleware.GetUserFromContext(c)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			return
		}

		c.JSON(http.StatusOK, MyPermissionsResponse{
			Role:        payload.Role,
			Permissions: provider.PermissionsForRole(payload.Role),
		})
	}
}
