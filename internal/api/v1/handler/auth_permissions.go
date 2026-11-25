package handler

import (
	"net/http"

	"goonhub/internal/api/middleware"

	"github.com/gin-gonic/gin"
)

// MyPermissionsResponse is the body of GET /api/v1/auth/permissions.
type MyPermissionsResponse struct {
	Role        string   `json:"role"`
	Superuser   bool     `json:"superuser"`
	Permissions []string `json:"permissions"`
}

// MyPermissions returns the authenticated user's current role and effective permissions
// (resolved live by the auth middleware), so that the frontend can hide or disable what the
// user may not do. The API still enforces every permission on its own routes.
func MyPermissions() gin.HandlerFunc {
	return func(c *gin.Context) {
		access := middleware.GetAccess(c)
		if access == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
			return
		}

		c.JSON(http.StatusOK, MyPermissionsResponse{
			Role:        access.Role,
			Superuser:   access.Superuser,
			Permissions: access.Permissions(),
		})
	}
}
