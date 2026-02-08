package api

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, sceneHandler *handler.SceneHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, jobHandler *handler.JobHandler, poolConfigHandler *handler.PoolConfigHandler, processingConfigHandler *handler.ProcessingConfigHandler, triggerConfigHandler *handler.TriggerConfigHandler, dlqHandler *handler.DLQHandler, retryConfigHandler *handler.RetryConfigHandler, sseHandler *handler.SSEHandler, tagHandler *handler.TagHandler, actorHandler *handler.ActorHandler, studioHandler *handler.StudioHandler, interactionHandler *handler.InteractionHandler, actorInteractionHandler *handler.ActorInteractionHandler, studioInteractionHandler *handler.StudioInteractionHandler, searchHandler *handler.SearchHandler, watchHistoryHandler *handler.WatchHistoryHandler, storagePathHandler *handler.StoragePathHandler, scanHandler *handler.ScanHandler, explorerHandler *handler.ExplorerHandler, pornDBHandler *handler.PornDBHandler, savedSearchHandler *handler.SavedSearchHandler, homepageHandler *handler.HomepageHandler, markerHandler *handler.MarkerHandler, importHandler *handler.ImportHandler, streamStatsHandler *handler.StreamStatsHandler, playlistHandler *handler.PlaylistHandler, shareHandler *handler.ShareHandler, authService *core.AuthService, rbacService *core.RBACService, logger *logging.Logger, rateLimiter *middleware.IPRateLimiter) {
	api := r.Group("/api")
	{
		v1 := api.Group("/v1")
		{
			// SSE endpoint (auth via query param, not middleware)
			v1.GET("/events", sseHandler.Stream)

			// Public share endpoints (no auth required)
			shares := v1.Group("/shares")
			{
				shares.GET("/:token", shareHandler.ResolveShareLink)
				shares.GET("/:token/stream", shareHandler.StreamShareLink)
			}

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

				scenes := protected.Group("/scenes")
				{
					scenes.POST("", middleware.RequirePermission(rbacService, "scenes:upload"), sceneHandler.UploadScene)
					scenes.GET("", middleware.RequirePermission(rbacService, "scenes:view"), sceneHandler.ListScenes)
					scenes.GET("/filters", middleware.RequirePermission(rbacService, "scenes:view"), sceneHandler.GetFilterOptions)
					scenes.GET("/:id", middleware.RequirePermission(rbacService, "scenes:view"), sceneHandler.GetScene)
					scenes.GET("/:id/reprocess", middleware.RequirePermission(rbacService, "scenes:reprocess"), sceneHandler.ReprocessScene)
					scenes.PUT("/:id/thumbnail", middleware.RequirePermission(rbacService, "scenes:upload"), sceneHandler.ExtractThumbnail)
					scenes.POST("/:id/thumbnail/upload", middleware.RequirePermission(rbacService, "scenes:upload"), sceneHandler.UploadThumbnail)
					scenes.PUT("/:id/details", middleware.RequirePermission(rbacService, "scenes:upload"), sceneHandler.UpdateSceneDetails)
					scenes.DELETE("/:id", middleware.RequirePermission(rbacService, "scenes:trash"), sceneHandler.DeleteScene)
					scenes.GET("/:id/tags", middleware.RequirePermission(rbacService, "scenes:view"), tagHandler.GetSceneTags)
					scenes.PUT("/:id/tags", middleware.RequirePermission(rbacService, "scenes:upload"), tagHandler.SetSceneTags)
					scenes.GET("/:id/interactions", interactionHandler.GetInteractions)
					scenes.GET("/:id/rating", interactionHandler.GetRating)
					scenes.PUT("/:id/rating", interactionHandler.SetRating)
					scenes.DELETE("/:id/rating", interactionHandler.DeleteRating)
					scenes.GET("/:id/like", interactionHandler.GetLike)
					scenes.POST("/:id/like", interactionHandler.ToggleLike)
					scenes.GET("/:id/jizzed", interactionHandler.GetJizzed)
					scenes.POST("/:id/jizzed", interactionHandler.ToggleJizzed)
					scenes.POST("/:id/watch", middleware.RequirePermission(rbacService, "scenes:view"), watchHistoryHandler.RecordWatch)
					scenes.GET("/:id/resume", middleware.RequirePermission(rbacService, "scenes:view"), watchHistoryHandler.GetResumePosition)
					scenes.GET("/:id/history", middleware.RequirePermission(rbacService, "scenes:view"), watchHistoryHandler.GetSceneHistory)
					scenes.GET("/:id/actors", middleware.RequirePermission(rbacService, "scenes:view"), actorHandler.GetSceneActors)
					scenes.PUT("/:id/actors", middleware.RequirePermission(rbacService, "scenes:upload"), actorHandler.SetSceneActors)
					scenes.GET("/:id/studio", middleware.RequirePermission(rbacService, "scenes:view"), studioHandler.GetSceneStudio)
					scenes.PUT("/:id/studio", middleware.RequirePermission(rbacService, "scenes:upload"), studioHandler.SetSceneStudio)
					scenes.GET("/:id/related", middleware.RequirePermission(rbacService, "scenes:view"), sceneHandler.GetRelatedScenes)
					scenes.GET("/:id/markers", middleware.RequirePermission(rbacService, "scenes:view"), markerHandler.ListMarkers)
					scenes.POST("/:id/markers", middleware.RequirePermission(rbacService, "scenes:view"), markerHandler.CreateMarker)
					scenes.PUT("/:id/markers/:markerID", middleware.RequirePermission(rbacService, "scenes:view"), markerHandler.UpdateMarker)
					scenes.DELETE("/:id/markers/:markerID", middleware.RequirePermission(rbacService, "scenes:view"), markerHandler.DeleteMarker)
					scenes.POST("/:id/shares", middleware.RequirePermission(rbacService, "scenes:view"), shareHandler.CreateShareLink)
					scenes.GET("/:id/shares", middleware.RequirePermission(rbacService, "scenes:view"), shareHandler.ListShareLinks)
				}

				// Share link deletion (protected, not under /scenes/:id)
				protected.DELETE("/shares/:id", shareHandler.DeleteShareLink)

				history := protected.Group("/history")
				{
					history.GET("", watchHistoryHandler.GetUserHistory)
					history.GET("/by-date", watchHistoryHandler.GetUserHistoryByDateRange)
					history.GET("/activity", watchHistoryHandler.GetDailyActivity)
				}

				tags := protected.Group("/tags")
				{
					tags.GET("", tagHandler.ListTags)
					tags.POST("", tagHandler.CreateTag)
					tags.DELETE("/:id", tagHandler.DeleteTag)
				}

				actors := protected.Group("/actors")
				{
					actors.GET("", actorHandler.ListActors)
					actors.GET("/:uuid", actorHandler.GetActorByUUID)
					actors.GET("/:uuid/scenes", actorHandler.GetActorScenes)
					actors.GET("/:uuid/interactions", actorInteractionHandler.GetInteractions)
					actors.PUT("/:uuid/rating", actorInteractionHandler.SetRating)
					actors.DELETE("/:uuid/rating", actorInteractionHandler.DeleteRating)
					actors.POST("/:uuid/like", actorInteractionHandler.ToggleLike)
				}

				studios := protected.Group("/studios")
				{
					studios.GET("", studioHandler.ListStudios)
					studios.GET("/:uuid", studioHandler.GetStudioByUUID)
					studios.GET("/:uuid/scenes", studioHandler.GetStudioScenes)
					studios.GET("/:uuid/interactions", studioInteractionHandler.GetInteractions)
					studios.PUT("/:uuid/rating", studioInteractionHandler.SetRating)
					studios.DELETE("/:uuid/rating", studioInteractionHandler.DeleteRating)
					studios.POST("/:uuid/like", studioInteractionHandler.ToggleLike)
				}

				explorer := protected.Group("/explorer")
				{
					explorer.GET("/storage-paths", explorerHandler.GetStoragePaths)
					explorer.GET("/folders/:storagePathID/*path", explorerHandler.GetFolderContents)
					explorer.POST("/bulk/tags", explorerHandler.BulkUpdateTags)
					explorer.POST("/bulk/actors", explorerHandler.BulkUpdateActors)
					explorer.POST("/bulk/studio", explorerHandler.BulkUpdateStudio)
					explorer.DELETE("/bulk/scenes", middleware.RequirePermission(rbacService, "scenes:delete"), explorerHandler.BulkDeleteScenes)
					explorer.POST("/folder/scene-ids", explorerHandler.GetFolderSceneIDs)
					explorer.POST("/search", explorerHandler.SearchInFolder)
					explorer.POST("/scenes/match-info", explorerHandler.GetScenesMatchInfo)
				}

				settings := protected.Group("/settings")
				{
					settings.GET("", settingsHandler.GetSettings)
					settings.PUT("", settingsHandler.UpdateAllSettings)
					settings.PUT("/password", settingsHandler.ChangePassword)
					settings.PUT("/username", settingsHandler.ChangeUsername)
					settings.GET("/parsing-rules", settingsHandler.GetParsingRules)
					settings.PUT("/parsing-rules", settingsHandler.UpdateParsingRules)
				}

				homepage := protected.Group("/homepage")
				{
					homepage.GET("", homepageHandler.GetHomepageData)
					homepage.GET("/sections/:id", homepageHandler.GetSectionData)
				}

				savedSearches := protected.Group("/saved-searches")
				{
					savedSearches.GET("", savedSearchHandler.List)
					savedSearches.GET("/:uuid", savedSearchHandler.GetByUUID)
					savedSearches.POST("", savedSearchHandler.Create)
					savedSearches.PUT("/:uuid", savedSearchHandler.Update)
					savedSearches.DELETE("/:uuid", savedSearchHandler.Delete)
				}

				playlists := protected.Group("/playlists")
				{
					playlists.GET("", playlistHandler.List)
					playlists.GET("/:uuid", playlistHandler.GetByUUID)
					playlists.POST("", middleware.RequirePermission(rbacService, "playlists:create"), playlistHandler.Create)
					playlists.PUT("/:uuid", middleware.RequirePermission(rbacService, "playlists:edit"), playlistHandler.Update)
					playlists.DELETE("/:uuid", middleware.RequirePermission(rbacService, "playlists:delete"), playlistHandler.Delete)
					playlists.POST("/:uuid/scenes", middleware.RequirePermission(rbacService, "playlists:edit"), playlistHandler.AddScenes)
					playlists.DELETE("/:uuid/scenes/:sceneId", middleware.RequirePermission(rbacService, "playlists:edit"), playlistHandler.RemoveScene)
					playlists.POST("/:uuid/scenes/remove", middleware.RequirePermission(rbacService, "playlists:edit"), playlistHandler.RemoveScenes)
					playlists.PUT("/:uuid/scenes/reorder", middleware.RequirePermission(rbacService, "playlists:edit"), playlistHandler.ReorderScenes)
					playlists.GET("/:uuid/tags", playlistHandler.GetTags)
					playlists.PUT("/:uuid/tags", middleware.RequirePermission(rbacService, "playlists:edit"), playlistHandler.SetTags)
					playlists.POST("/:uuid/like", middleware.RequirePermission(rbacService, "playlists:view_public"), playlistHandler.ToggleLike)
					playlists.GET("/:uuid/like", playlistHandler.GetLikeStatus)
					playlists.GET("/:uuid/progress", playlistHandler.GetProgress)
					playlists.PUT("/:uuid/progress", playlistHandler.UpdateProgress)
				}

				markers := protected.Group("/markers")
				{
					markers.GET("", markerHandler.ListLabelGroups)
					markers.GET("/all", markerHandler.ListAllMarkers)
					markers.GET("/labels", markerHandler.ListLabelSuggestions)
					markers.GET("/by-label", markerHandler.ListMarkersByLabel)
					markers.GET("/label-tags", markerHandler.GetLabelTags)
					markers.PUT("/label-tags", markerHandler.SetLabelTags)
					markers.GET("/:markerID/tags", markerHandler.GetMarkerTags)
					markers.PUT("/:markerID/tags", markerHandler.SetMarkerTags)
					markers.POST("/:markerID/tags", markerHandler.AddMarkerTags)
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
					admin.GET("/pool-config", poolConfigHandler.GetPoolConfig)
					admin.PUT("/pool-config", poolConfigHandler.UpdatePoolConfig)
					admin.GET("/processing-config", processingConfigHandler.GetProcessingConfig)
					admin.PUT("/processing-config", processingConfigHandler.UpdateProcessingConfig)
					admin.GET("/trigger-config", triggerConfigHandler.GetTriggerConfig)
					admin.PUT("/trigger-config", triggerConfigHandler.UpdateTriggerConfig)
					admin.POST("/scenes/:id/process/:phase", jobHandler.TriggerPhase)
					admin.PUT("/scenes/:id/scene-metadata", sceneHandler.ApplySceneMetadata)
					admin.POST("/jobs/bulk", jobHandler.TriggerBulkPhase)
					admin.POST("/jobs/retry-all-failed", jobHandler.RetryAllFailed)
					admin.POST("/jobs/retry-batch", jobHandler.RetryBatch)
					admin.DELETE("/jobs/failed", jobHandler.ClearFailed)
					admin.POST("/jobs/:id/cancel", jobHandler.CancelJob)
					admin.POST("/jobs/:id/retry", jobHandler.RetryJob)
					admin.GET("/jobs/recent-failed", jobHandler.ListRecentFailed)
					admin.GET("/dlq", dlqHandler.ListDLQ)
					admin.POST("/dlq/:job_id/retry", dlqHandler.RetryFromDLQ)
					admin.POST("/dlq/:job_id/abandon", dlqHandler.AbandonDLQ)
					admin.GET("/retry-config", retryConfigHandler.GetRetryConfig)
					admin.PUT("/retry-config", retryConfigHandler.UpdateRetryConfig)
					admin.GET("/search/status", searchHandler.GetStatus)
					admin.POST("/search/reindex", searchHandler.ReindexAll)
					admin.GET("/search/config", searchHandler.GetSearchConfig)
					admin.PUT("/search/config", searchHandler.UpdateSearchConfig)
					admin.GET("/storage-paths", storagePathHandler.List)
					admin.POST("/storage-paths", storagePathHandler.Create)
					admin.PUT("/storage-paths/:id", storagePathHandler.Update)
					admin.DELETE("/storage-paths/:id", storagePathHandler.Delete)
					admin.POST("/storage-paths/validate", storagePathHandler.ValidatePath)
					admin.POST("/scan", scanHandler.StartScan)
					admin.POST("/scan/cancel", scanHandler.CancelScan)
					admin.GET("/scan/status", scanHandler.GetStatus)
					admin.GET("/scan/history", scanHandler.GetHistory)
					admin.POST("/actors", actorHandler.CreateActor)
					admin.PUT("/actors/:id", actorHandler.UpdateActor)
					admin.DELETE("/actors/:id", actorHandler.DeleteActor)
					admin.POST("/actors/:id/image", actorHandler.UploadActorImage)

					// Studios management
					admin.POST("/studios", studioHandler.CreateStudio)
					admin.PUT("/studios/:id", studioHandler.UpdateStudio)
					admin.DELETE("/studios/:id", studioHandler.DeleteStudio)
					admin.POST("/studios/:id/logo", studioHandler.UploadStudioLogo)

					// PornDB integration
					admin.GET("/porndb/status", pornDBHandler.GetStatus)
					admin.GET("/porndb/performers", pornDBHandler.SearchPerformers)
					admin.GET("/porndb/performers/:id", pornDBHandler.GetPerformer)
					admin.GET("/porndb/performer-sites/:id", pornDBHandler.GetPerformerSite)
					admin.GET("/porndb/scenes", pornDBHandler.SearchScenes)
					admin.GET("/porndb/scenes/:id", pornDBHandler.GetScene)
					admin.GET("/porndb/sites", pornDBHandler.SearchSites)
					admin.GET("/porndb/sites/:id", pornDBHandler.GetSite)

					// Import endpoints
					admin.POST("/import/scenes", importHandler.ImportScene)
					admin.POST("/import/markers", importHandler.ImportMarker)

					// Stream statistics
					admin.GET("/stream-stats", streamStatsHandler.GetStreamStats)

					// Trash management
					admin.GET("/trash", adminHandler.ListTrash)
					admin.POST("/trash/:id/restore", adminHandler.RestoreScene)
					admin.DELETE("/trash/:id", adminHandler.PermanentDeleteScene)
					admin.DELETE("/trash", adminHandler.EmptyTrash)

					// App settings
					admin.GET("/app-settings", adminHandler.GetAppSettings)
					admin.PUT("/app-settings", adminHandler.UpdateAppSettings)
				}
			}
		}
	}

	// Public scene streaming endpoint (outside /api for better access)
	r.GET("/api/v1/scenes/:id/stream", sceneHandler.StreamScene)
}
