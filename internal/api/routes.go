package api

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, videoHandler *handler.VideoHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, jobHandler *handler.JobHandler, sseHandler *handler.SSEHandler, authService *core.AuthService, rbacService *core.RBACService, logger *logging.Logger, rateLimiter *middleware.IPRateLimiter) {
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			// SSE endpoint (auth via query param, not middleware)
			v1.GET("/events", sseHandler.Stream)

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
					videos.PUT("/:id/thumbnail", middleware.RequirePermission(rbacService, "videos:upload"), videoHandler.ExtractThumbnail)
					videos.POST("/:id/thumbnail/upload", middleware.RequirePermission(rbacService, "videos:upload"), videoHandler.UploadThumbnail)
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
					admin.GET("/jobs", jobHandler.ListJobs)
					admin.GET("/pool-config", jobHandler.GetPoolConfig)
					admin.PUT("/pool-config", jobHandler.UpdatePoolConfig)
					admin.GET("/processing-config", jobHandler.GetProcessingConfig)
					admin.PUT("/processing-config", jobHandler.UpdateProcessingConfig)
					admin.GET("/trigger-config", jobHandler.GetTriggerConfig)
					admin.PUT("/trigger-config", jobHandler.UpdateTriggerConfig)
					admin.POST("/videos/:id/process/:phase", jobHandler.TriggerPhase)
				}
			}
		}
	}

	// Public video streaming endpoint (outside /api for better access)
	r.GET("/api/v1/videos/:id/stream", videoHandler.StreamVideo)
}
