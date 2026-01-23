package api

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, videoHandler *handler.VideoHandler, authHandler *handler.AuthHandler, authService *core.AuthService, logger *logging.Logger, rateLimiter *middleware.IPRateLimiter) {
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			auth := v1.Group("/auth")
			{
				auth.POST("/login", middleware.RateLimitMiddleware(rateLimiter, logger.Logger), authHandler.Login)
			}

			videos := v1.Group("/videos")
			{
				videos.GET("/:id/stream", videoHandler.StreamVideo)
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
					videos.POST("", videoHandler.UploadVideo)
					videos.GET("", videoHandler.ListVideos)
					videos.GET("/:id", videoHandler.GetVideo)
					videos.GET("/:id/reprocess", videoHandler.ReprocessVideo)
					videos.DELETE("/:id", videoHandler.DeleteVideo)
				}
			}
		}
	}
}
