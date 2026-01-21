package api

import "github.com/gin-gonic/gin"

func RegisterRoutes(r *gin.Engine, videoHandler *VideoHandler) {
	api := r.Group("/api")
	{
		videos := api.Group("/videos")
		{
			videos.POST("", videoHandler.UploadVideo)
			videos.GET("", videoHandler.ListVideos)
		}
	}
}
