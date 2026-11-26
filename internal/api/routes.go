package api

import (
	"goonhub/internal/api/middleware"
	"goonhub/internal/api/v1/handler"
	"goonhub/internal/core"
	"goonhub/internal/infrastructure/logging"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r *gin.Engine, sceneHandler *handler.SceneHandler, authHandler *handler.AuthHandler, settingsHandler *handler.SettingsHandler, adminHandler *handler.AdminHandler, jobHandler *handler.JobHandler, poolConfigHandler *handler.PoolConfigHandler, processingConfigHandler *handler.ProcessingConfigHandler, triggerConfigHandler *handler.TriggerConfigHandler, dlqHandler *handler.DLQHandler, retryConfigHandler *handler.RetryConfigHandler, sseHandler *handler.SSEHandler, tagHandler *handler.TagHandler, actorHandler *handler.ActorHandler, studioHandler *handler.StudioHandler, interactionHandler *handler.InteractionHandler, actorInteractionHandler *handler.ActorInteractionHandler, studioInteractionHandler *handler.StudioInteractionHandler, searchHandler *handler.SearchHandler, watchHistoryHandler *handler.WatchHistoryHandler, storagePathHandler *handler.StoragePathHandler, scanHandler *handler.ScanHandler, explorerHandler *handler.ExplorerHandler, pornDBHandler *handler.PornDBHandler, savedSearchHandler *handler.SavedSearchHandler, homepageHandler *handler.HomepageHandler, markerHandler *handler.MarkerHandler, importHandler *handler.ImportHandler, streamStatsHandler *handler.StreamStatsHandler, playlistHandler *handler.PlaylistHandler, shareHandler *handler.ShareHandler, duplicateHandler *handler.DuplicateHandler, libraryHealthHandler *handler.LibraryHealthHandler, workerHandler *handler.WorkerHandler, socialHandler *handler.SocialHandler, presenceHandler *handler.PresenceHandler, togetherHandler *handler.TogetherHandler, authService *core.AuthService, rbacService *core.RBACService, logger *logging.Logger, rateLimiter *middleware.IPRateLimiter) {
	perm := middleware.RequirePermission
	anyPerm := middleware.RequireAnyPermission

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
				shares.GET("/:token/transcode", shareHandler.TranscodeShareLink)
				shares.HEAD("/:token/transcode", shareHandler.TranscodeShareLink)
			}

			auth := v1.Group("/auth")
			{
				auth.POST("/login", middleware.RateLimitMiddleware(rateLimiter, logger.Logger), authHandler.Login)
			}

			// Remote worker protocol (docs/REMOTE_WORKERS.md): per-worker bearer tokens, not
			// user sessions. The signed media URL carries its own authorization.
			if workerHandler != nil {
				v1.GET("/workers/media/:job_id", workerHandler.Gate(), workerHandler.Media)
				v1.HEAD("/workers/media/:job_id", workerHandler.Gate(), workerHandler.Media)
				w := v1.Group("/workers")
				w.Use(workerHandler.Auth())
				{
					w.POST("/register", workerHandler.Register)
					w.POST("/claim", workerHandler.Claim)
					w.POST("/heartbeat", workerHandler.Heartbeat)
					w.POST("/release-all", workerHandler.ReleaseAll)
					w.POST("/jobs/:job_id/complete", workerHandler.Complete)
					w.POST("/jobs/:job_id/fail", workerHandler.Fail)
					w.POST("/jobs/:job_id/release", workerHandler.Release)
					w.POST("/jobs/:job_id/media-url", workerHandler.MediaURL)
					w.PUT("/jobs/:job_id/artifacts/:name", workerHandler.UploadArtifact)
					w.GET("/jobs/:job_id/artifacts/:name", workerHandler.ArtifactStatus)
				}
			}

			protected := v1.Group("")
			protected.Use(middleware.AuthMiddleware(authService, rbacService))
			{
				auth := protected.Group("/auth")
				{
					auth.GET("/me", authHandler.Me)
					auth.GET("/permissions", handler.MyPermissions())
					auth.POST("/logout", authHandler.Logout)
				}

				scenes := protected.Group("/scenes")
				{
					scenes.POST("", perm(core.PermScenesUpload), sceneHandler.UploadScene)
					scenes.GET("", perm(core.PermScenesView), sceneHandler.ListScenes)
					scenes.GET("/filters", perm(core.PermScenesView), sceneHandler.GetFilterOptions)
					scenes.GET("/source-sites", perm(core.PermScenesView), sceneHandler.ListSourceSites)
					scenes.GET("/:id", perm(core.PermScenesView), sceneHandler.GetScene)
					scenes.GET("/:id/reprocess", perm(core.PermScenesReprocess), sceneHandler.ReprocessScene)
					scenes.PUT("/:id/thumbnail", perm(core.PermScenesEdit), sceneHandler.ExtractThumbnail)
					scenes.POST("/:id/thumbnail/upload", perm(core.PermScenesEdit), sceneHandler.UploadThumbnail)
					scenes.PUT("/:id/details", perm(core.PermScenesEdit), sceneHandler.UpdateSceneDetails)
					scenes.DELETE("/:id", perm(core.PermScenesTrash), sceneHandler.DeleteScene)
					scenes.GET("/:id/tags", perm(core.PermScenesView), tagHandler.GetSceneTags)
					scenes.PUT("/:id/tags", perm(core.PermScenesEdit), tagHandler.SetSceneTags)
					scenes.GET("/:id/interactions", interactionHandler.GetInteractions)
					scenes.GET("/:id/rating", interactionHandler.GetRating)
					scenes.PUT("/:id/rating", interactionHandler.SetRating)
					scenes.DELETE("/:id/rating", interactionHandler.DeleteRating)
					scenes.GET("/:id/like", interactionHandler.GetLike)
					scenes.POST("/:id/like", interactionHandler.ToggleLike)
					scenes.GET("/:id/jizzed", interactionHandler.GetJizzed)
					scenes.POST("/:id/jizzed", interactionHandler.ToggleJizzed)
					scenes.POST("/:id/watch", perm(core.PermScenesView), watchHistoryHandler.RecordWatch)
					scenes.GET("/:id/resume", perm(core.PermScenesView), watchHistoryHandler.GetResumePosition)
					scenes.GET("/:id/history", perm(core.PermScenesView), watchHistoryHandler.GetSceneHistory)
					scenes.GET("/:id/heatmap", perm(core.PermScenesView), watchHistoryHandler.GetReplayHeatmap)
					scenes.POST("/:id/heatmap", perm(core.PermScenesView), watchHistoryHandler.RecordReplayHeatmap)
					scenes.GET("/:id/actors", perm(core.PermScenesView), actorHandler.GetSceneActors)
					scenes.PUT("/:id/actors", perm(core.PermScenesEdit), actorHandler.SetSceneActors)
					scenes.GET("/:id/studio", perm(core.PermScenesView), studioHandler.GetSceneStudio)
					scenes.PUT("/:id/studio", perm(core.PermScenesEdit), studioHandler.SetSceneStudio)
					scenes.GET("/:id/related", perm(core.PermScenesView), sceneHandler.GetRelatedScenes)
					scenes.GET("/:id/similar", perm(core.PermScenesView), duplicateHandler.GetSimilarScenes)
					scenes.GET("/:id/copies", perm(core.PermScenesView), perm(core.PermFilesViewPaths), scanHandler.ListSceneCopies)
					scenes.GET("/:id/markers", perm(core.PermScenesView), markerHandler.ListMarkers)
					scenes.POST("/:id/markers", perm(core.PermMarkersCreate), markerHandler.CreateMarker)
					scenes.PUT("/:id/markers/:markerID", perm(core.PermMarkersCreate), markerHandler.UpdateMarker)
					scenes.DELETE("/:id/markers/:markerID", perm(core.PermMarkersCreate), markerHandler.DeleteMarker)
					scenes.POST("/:id/shares", perm(core.PermScenesShare), shareHandler.CreateShareLink)
					scenes.GET("/:id/shares", perm(core.PermScenesView), shareHandler.ListShareLinks)
				}

				// Goon2gether: watch lobbies, chat and emotes (docs/GOON2GETHER.md)
				if togetherHandler != nil {
					tg := protected.Group("/together")
					tg.Use(perm(core.PermTogetherUse))
					{
						tg.GET("/lobbies", togetherHandler.ListLobbies)
						tg.POST("/lobbies", perm(core.PermTogetherHost), togetherHandler.CreateLobby)
						tg.GET("/lobbies/:id", togetherHandler.GetLobby)
						tg.DELETE("/lobbies/:id", togetherHandler.CloseLobby)
						tg.POST("/lobbies/:id/join", togetherHandler.JoinLobby)
						tg.POST("/lobbies/:id/leave", togetherHandler.LeaveLobby)
						tg.POST("/lobbies/:id/invite-code", togetherHandler.RotateInviteCode)
						tg.POST("/lobbies/:id/queue", perm(core.PermScenesView), togetherHandler.AddToQueue)
						tg.GET("/lobbies/:id/ws", togetherHandler.Socket)
						tg.POST("/join", togetherHandler.JoinByCode)
						tg.GET("/emotes", togetherHandler.ListEmotes)
						tg.POST("/emotes", perm(core.PermEmotesManage), togetherHandler.UploadEmote)
						tg.PUT("/emotes/:id", perm(core.PermEmotesManage), togetherHandler.RenameEmote)
						tg.DELETE("/emotes/:id", perm(core.PermEmotesManage), togetherHandler.DeleteEmote)
					}
				}

				// Share link deletion (protected, not under /scenes/:id)
				protected.DELETE("/shares/:id", perm(core.PermScenesShare), shareHandler.DeleteShareLink)

				history := protected.Group("/history")
				history.Use(perm(core.PermScenesView))
				{
					history.GET("", watchHistoryHandler.GetUserHistory)
					history.GET("/by-date", watchHistoryHandler.GetUserHistoryByDateRange)
					history.GET("/activity", watchHistoryHandler.GetDailyActivity)
				}

				tags := protected.Group("/tags")
				{
					tags.GET("", tagHandler.ListTags)
					tags.POST("", perm(core.PermTagsManage), tagHandler.CreateTag)
					tags.GET("/duplicates", perm(core.PermTagsManage), tagHandler.ListDuplicateTags)
					tags.POST("/merge", perm(core.PermTagsManage), tagHandler.MergeTags)
					tags.PUT("/:id", perm(core.PermTagsManage), tagHandler.UpdateTag)
					tags.DELETE("/:id", perm(core.PermTagsManage), tagHandler.DeleteTag)
				}

				actors := protected.Group("/actors")
				{
					actors.GET("", actorHandler.ListActors)
					actors.GET("/resolve", actorHandler.ResolveActor)
					actors.GET("/:uuid", actorHandler.GetActorByUUID)
					actors.GET("/:uuid/merge-preview", perm(core.PermActorsMerge), actorHandler.MergeActorPreview)
					actors.POST("/:uuid/merge", perm(core.PermActorsMerge), actorHandler.MergeActor)
					actors.GET("/:uuid/scenes", perm(core.PermScenesView), actorHandler.GetActorScenes)
					actors.GET("/:uuid/interactions", actorInteractionHandler.GetInteractions)
					actors.PUT("/:uuid/rating", actorInteractionHandler.SetRating)
					actors.DELETE("/:uuid/rating", actorInteractionHandler.DeleteRating)
					actors.POST("/:uuid/like", actorInteractionHandler.ToggleLike)
				}

				studios := protected.Group("/studios")
				{
					studios.GET("", studioHandler.ListStudios)
					studios.GET("/resolve", studioHandler.ResolveStudio)
					studios.GET("/:uuid", studioHandler.GetStudioByUUID)
					studios.GET("/:uuid/merge-preview", perm(core.PermStudiosMerge), studioHandler.MergeStudioPreview)
					studios.POST("/:uuid/merge", perm(core.PermStudiosMerge), studioHandler.MergeStudio)
					studios.GET("/:uuid/scenes", perm(core.PermScenesView), studioHandler.GetStudioScenes)
					studios.GET("/:uuid/interactions", studioInteractionHandler.GetInteractions)
					studios.PUT("/:uuid/rating", studioInteractionHandler.SetRating)
					studios.DELETE("/:uuid/rating", studioInteractionHandler.DeleteRating)
					studios.POST("/:uuid/like", studioInteractionHandler.ToggleLike)
				}

				explorer := protected.Group("/explorer")
				explorer.Use(perm(core.PermExplorerAccess))
				{
					explorer.GET("/storage-paths", explorerHandler.GetStoragePaths)
					explorer.GET("/folders/:storagePathID/*path", explorerHandler.GetFolderContents)
					explorer.POST("/bulk/tags", perm(core.PermScenesEdit), explorerHandler.BulkUpdateTags)
					explorer.POST("/bulk/actors", perm(core.PermScenesEdit), explorerHandler.BulkUpdateActors)
					explorer.POST("/bulk/studio", perm(core.PermScenesEdit), explorerHandler.BulkUpdateStudio)
					explorer.DELETE("/bulk/scenes", perm(core.PermScenesDelete), explorerHandler.BulkDeleteScenes)
					explorer.POST("/folder/scene-ids", explorerHandler.GetFolderSceneIDs)
					explorer.POST("/search", explorerHandler.SearchInFolder)
					explorer.POST("/scenes/match-info", explorerHandler.GetScenesMatchInfo)
				}

				settings := protected.Group("/settings")
				{
					settings.GET("", settingsHandler.GetSettings)
					settings.PUT("", settingsHandler.UpdateAllSettings)
					settings.PATCH("", settingsHandler.PatchSettings)
					settings.PUT("/password", settingsHandler.ChangePassword)
					settings.PUT("/username", settingsHandler.ChangeUsername)
					settings.GET("/parsing-rules", settingsHandler.GetParsingRules)
					settings.PUT("/parsing-rules", settingsHandler.UpdateParsingRules)
				}

				homepage := protected.Group("/homepage")
				homepage.Use(perm(core.PermScenesView))
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
					playlists.GET("/:uuid", perm(core.PermScenesView), playlistHandler.GetByUUID)
					playlists.POST("", perm(core.PermPlaylistsCreate), playlistHandler.Create)
					playlists.PUT("/:uuid", perm(core.PermPlaylistsEdit), playlistHandler.Update)
					playlists.DELETE("/:uuid", perm(core.PermPlaylistsDelete), playlistHandler.Delete)
					playlists.POST("/:uuid/scenes", perm(core.PermPlaylistsEdit), playlistHandler.AddScenes)
					playlists.DELETE("/:uuid/scenes/:sceneId", perm(core.PermPlaylistsEdit), playlistHandler.RemoveScene)
					playlists.POST("/:uuid/scenes/remove", perm(core.PermPlaylistsEdit), playlistHandler.RemoveScenes)
					playlists.PUT("/:uuid/scenes/reorder", perm(core.PermPlaylistsEdit), playlistHandler.ReorderScenes)
					// Marker playlists (docs/MARKER_PLAYLISTS.md)
					playlists.POST("/:uuid/markers", perm(core.PermPlaylistsEdit), playlistHandler.AddMarkers)
					playlists.POST("/:uuid/markers/from-filter", perm(core.PermPlaylistsEdit), playlistHandler.AddMarkersFromFilter)
					playlists.DELETE("/:uuid/markers/:markerId", perm(core.PermPlaylistsEdit), playlistHandler.RemoveMarker)
					playlists.POST("/:uuid/markers/remove", perm(core.PermPlaylistsEdit), playlistHandler.RemoveMarkers)
					playlists.PUT("/:uuid/markers/reorder", perm(core.PermPlaylistsEdit), playlistHandler.ReorderMarkers)
					playlists.GET("/:uuid/tags", playlistHandler.GetTags)
					playlists.PUT("/:uuid/tags", perm(core.PermPlaylistsEdit), playlistHandler.SetTags)
					playlists.POST("/:uuid/like", perm(core.PermPlaylistsPublic), playlistHandler.ToggleLike)
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
					markers.PUT("/label-tags", perm(core.PermMarkerLabelsManage), markerHandler.SetLabelTags)
					markers.GET("/:markerID/tags", markerHandler.GetMarkerTags)
					markers.PUT("/:markerID/tags", perm(core.PermMarkersCreate), markerHandler.SetMarkerTags)
					markers.POST("/:markerID/tags", perm(core.PermMarkersCreate), markerHandler.AddMarkerTags)
				}

				// Server-wide marker labels (docs/MARKER_LABELS.md); everyone lists them through
				// GET /markers/labels
				markerLabels := protected.Group("/marker-labels", perm(core.PermMarkerLabelsManage))
				{
					markerLabels.GET("", markerHandler.ListMarkerLabels)
					markerLabels.POST("", markerHandler.CreateMarkerLabel)
					markerLabels.POST("/merge", markerHandler.MergeMarkerLabels)
					markerLabels.POST("/delete", markerHandler.DeleteMarkerLabels)
					markerLabels.PUT("/:labelID", markerHandler.UpdateMarkerLabel)
					markerLabels.DELETE("/:labelID", markerHandler.DeleteMarkerLabel)
					markerLabels.GET("/:labelID/tags", markerHandler.GetMarkerLabelTags)
					markerLabels.PUT("/:labelID/tags", markerHandler.SetMarkerLabelTags)
				}

				// Social graph and privacy (docs/SOCIAL.md): reading needs a signed-in user (privacy
				// is checked by the services), creating a relation needs social:use. Undoing one
				// (unfollow, decline, cancel, remove a friend) is always allowed.
				social := protected.Group("/social")
				{
					social.GET("/me", socialHandler.Me)
					social.GET("/users", socialHandler.SearchUsers)
					social.GET("/users/:username", socialHandler.GetUser)
					social.GET("/users/:username/friends", socialHandler.ListFriends)
					social.GET("/users/:username/followers", socialHandler.ListFollowers)
					social.GET("/users/:username/following", socialHandler.ListFollowing)
					social.POST("/follows/:userId", perm(core.PermSocialUse), socialHandler.Follow)
					social.DELETE("/follows/:userId", socialHandler.Unfollow)
					social.DELETE("/friends/:userId", socialHandler.RemoveFriend)
					social.GET("/friend-requests", socialHandler.ListFriendRequests)
					social.POST("/friend-requests", perm(core.PermSocialUse), socialHandler.SendFriendRequest)
					social.POST("/friend-requests/:userId/accept", perm(core.PermSocialUse), socialHandler.AcceptFriendRequest)
					social.POST("/friend-requests/:userId/decline", socialHandler.DeclineFriendRequest)
					social.DELETE("/friend-requests/:userId", socialHandler.CancelFriendRequest)
					social.GET("/privacy", socialHandler.GetPrivacy)
					social.PUT("/privacy", socialHandler.UpdatePrivacy)
					// Presence (docs/PRESENCE.md): what I may see of my contacts (privacy checked
					// by the service), and the reports of my own tabs
					social.GET("/presence", presenceHandler.Snapshot)
					social.GET("/presence/users/:userId", presenceHandler.GetUser)
					social.POST("/presence/heartbeat", presenceHandler.Heartbeat)
					social.PUT("/presence/activity", presenceHandler.UpdateActivity)
					social.DELETE("/presence/activity", presenceHandler.ClearActivity)
					social.GET("/presence/settings", presenceHandler.GetSettings)
					social.PUT("/presence/settings", presenceHandler.UpdateSettings)
				}

				markerFilters := protected.Group("/marker-filters")
				{
					markerFilters.GET("", markerHandler.ListSavedFilters)
					markerFilters.POST("", markerHandler.CreateSavedFilter)
					markerFilters.PUT("/:uuid", markerHandler.UpdateSavedFilter)
					markerFilters.DELETE("/:uuid", markerHandler.DeleteSavedFilter)
				}

				// Admin API: each area needs its own permission (docs/RBAC.md); a superuser
				// passes every check.
				admin := protected.Group("/admin")
				{
					// Users, roles and permissions
					admin.GET("/users", perm(core.PermUsersManage), adminHandler.ListUsers)
					admin.POST("/users", perm(core.PermUsersManage), adminHandler.CreateUser)
					admin.PUT("/users/:id/role", perm(core.PermUsersManage), adminHandler.UpdateUserRole)
					admin.PUT("/users/:id/share-markers", perm(core.PermUsersManage), adminHandler.UpdateUserShareMarkers)
					admin.PUT("/users/:id/password", perm(core.PermUsersManage), adminHandler.ResetUserPassword)
					admin.GET("/users/:id/permissions", perm(core.PermUsersManage), adminHandler.GetUserPermissions)
					admin.PUT("/users/:id/permissions", perm(core.PermUsersManage), adminHandler.SetUserPermissions)
					admin.DELETE("/users/:id", perm(core.PermUsersManage), adminHandler.DeleteUser)
					admin.GET("/permissions", anyPerm(core.PermRolesManage, core.PermUsersManage), adminHandler.ListPermissions)
					admin.GET("/roles", anyPerm(core.PermRolesManage, core.PermUsersManage), adminHandler.ListRoles)
					admin.POST("/roles", perm(core.PermRolesManage), adminHandler.CreateRole)
					admin.PUT("/roles/:id", perm(core.PermRolesManage), adminHandler.UpdateRole)
					admin.DELETE("/roles/:id", perm(core.PermRolesManage), adminHandler.DeleteRole)
					admin.PUT("/roles/:id/permissions", perm(core.PermRolesManage), adminHandler.SetRolePermissions)
					admin.GET("/roles/:id/effective", perm(core.PermRolesManage), adminHandler.GetRoleEffective)

					// Jobs and processing
					jobs := admin.Group("", perm(core.PermJobsManage))
					jobs.GET("/jobs", jobHandler.ListJobs)
					jobs.GET("/pool-config", poolConfigHandler.GetPoolConfig)
					jobs.PUT("/pool-config", poolConfigHandler.UpdatePoolConfig)
					jobs.GET("/job-schedule", poolConfigHandler.GetJobSchedule)
					jobs.PUT("/job-schedule", poolConfigHandler.UpdateJobSchedule)
					jobs.GET("/processing-config", processingConfigHandler.GetProcessingConfig)
					jobs.PUT("/processing-config", processingConfigHandler.UpdateProcessingConfig)
					jobs.GET("/trigger-config", triggerConfigHandler.GetTriggerConfig)
					jobs.PUT("/trigger-config", triggerConfigHandler.UpdateTriggerConfig)
					jobs.POST("/scenes/:id/process/:phase", jobHandler.TriggerPhase)
					jobs.GET("/transcode-cache", sceneHandler.GetTranscodeCache)
					jobs.DELETE("/transcode-cache", sceneHandler.ClearTranscodeCache)
					jobs.POST("/jobs/bulk", jobHandler.TriggerBulkPhase)
					jobs.POST("/jobs/retry-all-failed", jobHandler.RetryAllFailed)
					jobs.POST("/jobs/retry-batch", jobHandler.RetryBatch)
					jobs.DELETE("/jobs/failed", jobHandler.ClearFailed)
					jobs.POST("/jobs/cancel-bulk", jobHandler.CancelBulk)
					jobs.GET("/jobs/failing", jobHandler.ListFailing)
					jobs.POST("/jobs/failing/bulk", jobHandler.BulkFailing)
					jobs.POST("/jobs/failing/:id/retry", jobHandler.RetryFailing)
					jobs.POST("/jobs/failing/:id/clear", jobHandler.ClearFailing)
					jobs.GET("/jobs/history-purge", jobHandler.GetHistoryPurge)
					jobs.POST("/jobs/history-purge", jobHandler.PurgeHistory)
					jobs.POST("/jobs/:id/cancel", jobHandler.CancelJob)
					jobs.POST("/jobs/:id/retry", jobHandler.RetryJob)
					jobs.GET("/jobs/recent-failed", jobHandler.ListRecentFailed)
					if workerHandler != nil {
						jobs.GET("/workers", workerHandler.AdminList)
						jobs.POST("/workers", workerHandler.AdminCreate)
						jobs.PATCH("/workers/:id", workerHandler.AdminUpdate)
						jobs.DELETE("/workers/:id", workerHandler.AdminDelete)
						jobs.POST("/workers/:id/token", workerHandler.AdminRotateToken)
						jobs.GET("/worker-routing", workerHandler.AdminGetRouting)
						jobs.PUT("/worker-routing", workerHandler.AdminUpdateRouting)
					}
					jobs.GET("/dlq", dlqHandler.ListDLQ)
					jobs.POST("/dlq/:job_id/retry", dlqHandler.RetryFromDLQ)
					jobs.POST("/dlq/:job_id/abandon", dlqHandler.AbandonDLQ)
					jobs.GET("/retry-config", retryConfigHandler.GetRetryConfig)
					jobs.PUT("/retry-config", retryConfigHandler.UpdateRetryConfig)

					// App settings and search
					settingsAdmin := admin.Group("", perm(core.PermSettingsManage))
					settingsAdmin.GET("/search/status", searchHandler.GetStatus)
					settingsAdmin.POST("/search/reindex", searchHandler.ReindexAll)
					settingsAdmin.GET("/search/config", searchHandler.GetSearchConfig)
					settingsAdmin.PUT("/search/config", searchHandler.UpdateSearchConfig)
					settingsAdmin.GET("/app-settings", adminHandler.GetAppSettings)
					settingsAdmin.PUT("/app-settings", adminHandler.UpdateAppSettings)

					// Storage paths and library scans
					storage := admin.Group("", perm(core.PermStorageManage))
					storage.GET("/storage-paths", storagePathHandler.List)
					storage.POST("/storage-paths", storagePathHandler.Create)
					storage.PUT("/storage-paths/:id", storagePathHandler.Update)
					storage.DELETE("/storage-paths/:id", storagePathHandler.Delete)
					storage.POST("/storage-paths/validate", storagePathHandler.ValidatePath)
					storage.POST("/scan", scanHandler.StartScan)
					storage.POST("/scan/cancel", scanHandler.CancelScan)
					storage.GET("/scan/status", scanHandler.GetStatus)
					storage.GET("/scan/history", scanHandler.GetHistory)

					// Actors management
					actorsAdmin := admin.Group("", perm(core.PermActorsManage))
					actorsAdmin.POST("/actors", actorHandler.CreateActor)
					actorsAdmin.PUT("/actors/:id", actorHandler.UpdateActor)
					actorsAdmin.DELETE("/actors/:id", actorHandler.DeleteActor)
					actorsAdmin.POST("/actors/:id/image", actorHandler.UploadActorImage)

					// Studios management
					studiosAdmin := admin.Group("", perm(core.PermStudiosManage))
					studiosAdmin.POST("/studios", studioHandler.CreateStudio)
					studiosAdmin.PUT("/studios/:id", studioHandler.UpdateStudio)
					studiosAdmin.DELETE("/studios/:id", studioHandler.DeleteStudio)
					studiosAdmin.POST("/studios/:id/logo", studioHandler.UploadStudioLogo)

					// PornDB integration
					porndb := admin.Group("", perm(core.PermPornDBUse))
					porndb.PUT("/scenes/:id/scene-metadata", sceneHandler.ApplySceneMetadata)
					porndb.GET("/porndb/status", pornDBHandler.GetStatus)
					porndb.GET("/porndb/performers", pornDBHandler.SearchPerformers)
					porndb.GET("/porndb/performers/:id", pornDBHandler.GetPerformer)
					porndb.GET("/porndb/performer-sites/:id", pornDBHandler.GetPerformerSite)
					porndb.GET("/porndb/scenes", pornDBHandler.SearchScenes)
					porndb.GET("/porndb/scenes/:id", pornDBHandler.GetScene)
					porndb.GET("/porndb/sites", pornDBHandler.SearchSites)
					porndb.GET("/porndb/sites/:id", pornDBHandler.GetSite)

					// Import endpoints
					imports := admin.Group("", perm(core.PermImportUse))
					imports.GET("/import/capabilities", importHandler.Capabilities)
					imports.POST("/import/scenes", importHandler.ImportScene)
					imports.POST("/import/scenes/summary", importHandler.SceneSummary)
					imports.POST("/import/markers", importHandler.ImportMarker)

					// Stream statistics
					admin.GET("/stream-stats", perm(core.PermStatsViewStreams), streamStatsHandler.GetStreamStats)

					// Trash management
					trash := admin.Group("", perm(core.PermTrashManage))
					trash.GET("/trash", adminHandler.ListTrash)
					trash.GET("/trash/ids", adminHandler.ListTrashIDs)
					trash.GET("/trash/facets", adminHandler.TrashFacets)
					trash.POST("/trash/restore-bulk", adminHandler.RestoreTrashBulk)
					trash.POST("/trash/delete-bulk", adminHandler.DeleteTrashBulk)
					trash.POST("/trash/:id/restore", adminHandler.RestoreScene)
					trash.DELETE("/trash/:id", adminHandler.PermanentDeleteScene)
					trash.DELETE("/trash", adminHandler.EmptyTrash)

					// Duplicate detection
					dups := admin.Group("", perm(core.PermDuplicatesManage))
					dups.GET("/duplicates", duplicateHandler.ListGroups)
					dups.GET("/duplicates/selectable-ids", duplicateHandler.SelectableIDs)
					dups.GET("/duplicates/copies", scanHandler.ListCopies)
					dups.GET("/duplicates/copies/ids", scanHandler.CopyIDs)
					dups.POST("/duplicates/copies/forget-bulk", scanHandler.ForgetCopiesBulk)
					dups.POST("/duplicates/copies/delete-files-bulk", scanHandler.DeleteCopyFilesBulk)
					dups.POST("/duplicates/copies/:id/make-main", scanHandler.MakeCopyMain)
					dups.POST("/duplicates/copies/:id/separate", scanHandler.SeparateCopy)
					dups.POST("/duplicates/copies/:id/forget", scanHandler.ForgetCopy)
					dups.POST("/duplicates/copies/:id/delete-file", scanHandler.DeleteCopyFile)
					dups.GET("/duplicates/stats", duplicateHandler.GetStats)
					dups.GET("/duplicates/config", duplicateHandler.GetConfig)
					dups.PUT("/duplicates/config", duplicateHandler.UpdateConfig)
					dups.POST("/duplicates/auto-select", duplicateHandler.AutoSelectBest)
					dups.POST("/duplicates/resolve-bulk", duplicateHandler.ResolveBulk)
					dups.POST("/duplicates/trash-all-bulk", duplicateHandler.TrashAllBulk)
					dups.POST("/duplicates/:id/cancel-deletion", duplicateHandler.CancelDeletion)
					dups.GET("/duplicates/:id", duplicateHandler.GetGroup)
					dups.POST("/duplicates/:id/resolve", duplicateHandler.ResolveGroup)
					dups.POST("/duplicates/:id/dismiss", duplicateHandler.DismissGroup)
					dups.POST("/duplicates/:id/trash-all", duplicateHandler.TrashAll)
					dups.POST("/duplicates/:id/exclude", duplicateHandler.ExcludeMembers)
					dups.PUT("/duplicates/:id/best", duplicateHandler.SetBest)
					// Fold duplicate markers (e.g. left by earlier merges), dry run by default
					dups.POST("/markers/deduplicate", markerHandler.DeduplicateMarkers)

					// Library health (media files are only read: stat, never written)
					health := admin.Group("", perm(core.PermLibraryHealth))
					health.GET("/library-health", libraryHealthHandler.GetSummary)
					health.GET("/library-health/scenes", libraryHealthHandler.ListScenes)
					health.GET("/library-health/preview", libraryHealthHandler.PreviewAction)
					health.POST("/library-health/tasks", libraryHealthHandler.StartTask)
					health.POST("/library-health/tasks/cancel", libraryHealthHandler.CancelTask)
					health.GET("/library-health/tasks/status", libraryHealthHandler.GetTaskStatus)
				}
			}
		}
	}

	// Public scene streaming endpoint (outside /api for better access)
	r.GET("/api/v1/scenes/:id/stream", sceneHandler.StreamScene)
	// Same access as the stream, for the files browsers can't play (HEAD: slot available?)
	r.GET("/api/v1/scenes/:id/transcode", sceneHandler.TranscodeScene)
	r.HEAD("/api/v1/scenes/:id/transcode", sceneHandler.TranscodeScene)
}
