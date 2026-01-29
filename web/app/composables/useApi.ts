/**
 * Unified API composable that re-exports all domain-specific API functions.
 * Provides backwards compatibility for existing consumers.
 *
 * For new code, prefer importing domain-specific composables directly:
 * - useApiVideos() for video operations
 * - useApiSettings() for user settings
 * - useApiAdmin() for user/role management
 * - useApiJobs() for job history and config
 * - useApiTags() for tag operations
 * - useApiActors() for actor operations
 * - useApiStudios() for studio operations
 * - useApiPornDB() for PornDB integration
 * - useApiStorage() for storage paths and scanning
 * - useApiDLQ() for dead letter queue operations
 * - useApiExplorer() for folder browsing and bulk editing
 */
export const useApi = () => {
    const videos = useApiVideos();
    const settings = useApiSettings();
    const admin = useApiAdmin();
    const jobs = useApiJobs();
    const tags = useApiTags();
    const actors = useApiActors();
    const studios = useApiStudios();
    const porndb = useApiPornDB();
    const storage = useApiStorage();
    const dlq = useApiDLQ();
    const explorer = useApiExplorer();

    return {
        // Video operations
        uploadVideo: videos.uploadVideo,
        fetchVideos: videos.fetchVideos,
        searchVideos: videos.searchVideos,
        fetchFilterOptions: videos.fetchFilterOptions,
        fetchVideo: videos.fetchVideo,
        updateVideoDetails: videos.updateVideoDetails,
        extractThumbnail: videos.extractThumbnail,
        uploadThumbnail: videos.uploadThumbnail,
        fetchVideoInteractions: videos.fetchVideoInteractions,
        fetchVideoRating: videos.fetchVideoRating,
        setVideoRating: videos.setVideoRating,
        deleteVideoRating: videos.deleteVideoRating,
        fetchVideoLike: videos.fetchVideoLike,
        toggleVideoLike: videos.toggleVideoLike,
        fetchJizzedCount: videos.fetchJizzedCount,
        incrementJizzed: videos.incrementJizzed,
        recordWatch: videos.recordWatch,
        getResumePosition: videos.getResumePosition,
        getVideoWatchHistory: videos.getVideoWatchHistory,
        getUserWatchHistory: videos.getUserWatchHistory,

        // Settings operations
        fetchSettings: settings.fetchSettings,
        updatePlayerSettings: settings.updatePlayerSettings,
        updateAppSettings: settings.updateAppSettings,
        updateTagSettings: settings.updateTagSettings,
        changePassword: settings.changePassword,
        changeUsername: settings.changeUsername,

        // Admin user operations
        fetchAdminUsers: admin.fetchAdminUsers,
        createUser: admin.createUser,
        updateUserRole: admin.updateUserRole,
        resetUserPassword: admin.resetUserPassword,
        deleteUser: admin.deleteUser,
        fetchRoles: admin.fetchRoles,
        fetchPermissions: admin.fetchPermissions,
        syncRolePermissions: admin.syncRolePermissions,

        // Job operations
        fetchJobs: jobs.fetchJobs,
        fetchPoolConfig: jobs.fetchPoolConfig,
        updatePoolConfig: jobs.updatePoolConfig,
        fetchProcessingConfig: jobs.fetchProcessingConfig,
        updateProcessingConfig: jobs.updateProcessingConfig,
        fetchTriggerConfig: jobs.fetchTriggerConfig,
        updateTriggerConfig: jobs.updateTriggerConfig,
        triggerVideoPhase: jobs.triggerVideoPhase,
        triggerBulkPhase: jobs.triggerBulkPhase,
        fetchRetryConfig: jobs.fetchRetryConfig,
        updateRetryConfig: jobs.updateRetryConfig,

        // Tag operations
        fetchTags: tags.fetchTags,
        createTag: tags.createTag,
        deleteTag: tags.deleteTag,
        fetchVideoTags: tags.fetchVideoTags,
        setVideoTags: tags.setVideoTags,

        // Actor operations
        fetchActors: actors.fetchActors,
        fetchActorByUUID: actors.fetchActorByUUID,
        fetchActorVideos: actors.fetchActorVideos,
        createActor: actors.createActor,
        updateActor: actors.updateActor,
        deleteActor: actors.deleteActor,
        uploadActorImage: actors.uploadActorImage,
        fetchVideoActors: actors.fetchVideoActors,
        setVideoActors: actors.setVideoActors,
        fetchActorInteractions: actors.fetchActorInteractions,
        setActorRating: actors.setActorRating,
        deleteActorRating: actors.deleteActorRating,
        toggleActorLike: actors.toggleActorLike,

        // Studio operations
        fetchStudios: studios.fetchStudios,
        fetchStudioByUUID: studios.fetchStudioByUUID,
        fetchStudioVideos: studios.fetchStudioVideos,
        createStudio: studios.createStudio,
        updateStudio: studios.updateStudio,
        deleteStudio: studios.deleteStudio,
        uploadStudioLogo: studios.uploadStudioLogo,
        fetchVideoStudio: studios.fetchVideoStudio,
        setVideoStudio: studios.setVideoStudio,
        fetchStudioInteractions: studios.fetchStudioInteractions,
        setStudioRating: studios.setStudioRating,
        deleteStudioRating: studios.deleteStudioRating,
        toggleStudioLike: studios.toggleStudioLike,

        // PornDB operations
        getPornDBStatus: porndb.getPornDBStatus,
        searchPornDBPerformers: porndb.searchPornDBPerformers,
        getPornDBPerformer: porndb.getPornDBPerformer,
        searchPornDBScenes: porndb.searchPornDBScenes,
        getPornDBScene: porndb.getPornDBScene,
        searchPornDBSites: porndb.searchPornDBSites,
        getPornDBSite: porndb.getPornDBSite,
        applySceneMetadata: porndb.applySceneMetadata,

        // Storage operations
        fetchStoragePaths: storage.fetchStoragePaths,
        createStoragePath: storage.createStoragePath,
        updateStoragePath: storage.updateStoragePath,
        deleteStoragePath: storage.deleteStoragePath,
        validateStoragePath: storage.validateStoragePath,
        startScan: storage.startScan,
        cancelScan: storage.cancelScan,
        getScanStatus: storage.getScanStatus,
        getScanHistory: storage.getScanHistory,

        // DLQ operations
        fetchDLQ: dlq.fetchDLQ,
        retryFromDLQ: dlq.retryFromDLQ,
        abandonDLQ: dlq.abandonDLQ,

        // Explorer operations
        getExplorerStoragePaths: explorer.getStoragePaths,
        getExplorerFolderContents: explorer.getFolderContents,
        bulkUpdateTags: explorer.bulkUpdateTags,
        bulkUpdateActors: explorer.bulkUpdateActors,
        bulkUpdateStudio: explorer.bulkUpdateStudio,
        getFolderVideoIDs: explorer.getFolderVideoIDs,
    };
};
