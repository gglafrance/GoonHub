package api

import (
	"goonhub/internal/api/v1/handler"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, videoHandler *handler.VideoHandler) {
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			videos := v1.Group("/videos")
			{
				videos.POST("", videoHandler.UploadVideo)
				videos.GET("", videoHandler.ListVideos)
				videos.GET("/:id/reprocess", videoHandler.ReprocessVideo)
				videos.DELETE("/:id", videoHandler.DeleteVideo)
			}
		}
	}
}
