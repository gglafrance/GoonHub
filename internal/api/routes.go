package api

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, videoHandler *handler.VideoHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, authService *core.AuthService, rbacService *core.RBACService, logger *logging.Logger, rateLimiter *middleware.IPRateLimiter) {
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			auth := v1.Group("/auth")
			{
				auth.POST("/login", middleware.RateLimitMiddleware(rateLimiter, logger.Logger), authHandler.Login)
			}

			protected := v1.Group("")
			protected.Use(middleware.AuthMiddleware(authService))
			{
				auth := protected.Group("/auth")
				{
					auth.GET("/me", authHandler.Me)
					auth.POST("/logout", authHandler.Logout)
				}

				videos := protected.Group("/videos")
				{
					videos.POST("", middleware.RequirePermission(rbacService, "videos:upload"), videoHandler.UploadVideo)
					videos.GET("", middleware.RequirePermission(rbacService, "videos:view"), videoHandler.ListVideos)
					videos.GET("/:id", middleware.RequirePermission(rbacService, "videos:view"), videoHandler.GetVideo)
					videos.GET("/:id/reprocess", middleware.RequirePermission(rbacService, "videos:reprocess"), videoHandler.ReprocessVideo)
					videos.DELETE("/:id", middleware.RequirePermission(rbacService, "videos:delete"), videoHandler.DeleteVideo)
				}

				settings := protected.Group("/settings")
				{
					settings.GET("", settingsHandler.GetSettings)
					settings.PUT("/player", settingsHandler.UpdatePlayerSettings)
					settings.PUT("/app", settingsHandler.UpdateAppSettings)
					settings.PUT("/password", settingsHandler.ChangePassword)
					settings.PUT("/username", settingsHandler.ChangeUsername)
				}

				admin := protected.Group("/admin")
				admin.Use(middleware.RequireRole("admin"))
				{
					admin.GET("/users", adminHandler.ListUsers)
					admin.POST("/users", adminHandler.CreateUser)
					admin.PUT("/users/:id/role", adminHandler.UpdateUserRole)
					admin.PUT("/users/:id/password", adminHandler.ResetUserPassword)
					admin.DELETE("/users/:id", adminHandler.DeleteUser)
					admin.GET("/roles", adminHandler.ListRoles)
					admin.GET("/permissions", adminHandler.ListPermissions)
					admin.PUT("/roles/:id/permissions", adminHandler.SyncRolePermissions)
				}
			}
		}
	}

	// Public video streaming endpoint (outside /api for better access)
	r.GET("/api/v1/videos/:id/stream", videoHandler.StreamVideo)
}
