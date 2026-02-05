/**
 * Unified API composable that re-exports all domain-specific API functions.
 * Provides backwards compatibility for existing consumers.
 *
 * For new code, prefer importing domain-specific composables directly:
 * - useApiScenes() for scene operations
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
 * - useApiSavedSearches() for saved search templates
 * - useApiMarkers() for scene marker operations
 */
export const useApi = () => {
    const scenes = useApiScenes();
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
    const savedSearches = useApiSavedSearches();
    const markers = useApiMarkers();

    return {
        // Scene operations
        uploadScene: scenes.uploadScene,
        fetchScenes: scenes.fetchScenes,
        searchScenes: scenes.searchScenes,
        fetchFilterOptions: scenes.fetchFilterOptions,
        fetchScene: scenes.fetchScene,
        updateSceneDetails: scenes.updateSceneDetails,
        extractThumbnail: scenes.extractThumbnail,
        uploadThumbnail: scenes.uploadThumbnail,
        fetchSceneInteractions: scenes.fetchSceneInteractions,
        fetchSceneRating: scenes.fetchSceneRating,
        setSceneRating: scenes.setSceneRating,
        deleteSceneRating: scenes.deleteSceneRating,
        fetchSceneLike: scenes.fetchSceneLike,
        toggleSceneLike: scenes.toggleSceneLike,
        fetchJizzedCount: scenes.fetchJizzedCount,
        incrementJizzed: scenes.incrementJizzed,
        recordWatch: scenes.recordWatch,
        getResumePosition: scenes.getResumePosition,
        getSceneWatchHistory: scenes.getSceneWatchHistory,
        getUserWatchHistory: scenes.getUserWatchHistory,
        getUserWatchHistoryByDateRange: scenes.getUserWatchHistoryByDateRange,
        getUserWatchHistoryByTimeRange: scenes.getUserWatchHistoryByTimeRange,
        getDailyActivity: scenes.getDailyActivity,

        // Settings operations
        fetchSettings: settings.fetchSettings,
        updatePlayerSettings: settings.updatePlayerSettings,
        updateAppSettings: settings.updateAppSettings,
        updateTagSettings: settings.updateTagSettings,
        changePassword: settings.changePassword,
        changeUsername: settings.changeUsername,
        getParsingRules: settings.getParsingRules,
        updateParsingRules: settings.updateParsingRules,

        // Admin user operations
        fetchAdminUsers: admin.fetchAdminUsers,
        createUser: admin.createUser,
        updateUserRole: admin.updateUserRole,
        resetUserPassword: admin.resetUserPassword,
        deleteUser: admin.deleteUser,
        fetchRoles: admin.fetchRoles,
        fetchPermissions: admin.fetchPermissions,
        syncRolePermissions: admin.syncRolePermissions,
        getSearchStatus: admin.getSearchStatus,
        triggerReindex: admin.triggerReindex,

        // Job operations
        fetchJobs: jobs.fetchJobs,
        fetchPoolConfig: jobs.fetchPoolConfig,
        updatePoolConfig: jobs.updatePoolConfig,
        fetchProcessingConfig: jobs.fetchProcessingConfig,
        updateProcessingConfig: jobs.updateProcessingConfig,
        fetchTriggerConfig: jobs.fetchTriggerConfig,
        updateTriggerConfig: jobs.updateTriggerConfig,
        triggerScenePhase: jobs.triggerScenePhase,
        triggerBulkPhase: jobs.triggerBulkPhase,
        fetchRetryConfig: jobs.fetchRetryConfig,
        updateRetryConfig: jobs.updateRetryConfig,
        cancelJob: jobs.cancelJob,
        retryJob: jobs.retryJob,
        fetchRecentFailedJobs: jobs.fetchRecentFailedJobs,
        retryAllFailed: jobs.retryAllFailed,
        retryBatchJobs: jobs.retryBatchJobs,
        clearFailedJobs: jobs.clearFailedJobs,

        // Tag operations
        fetchTags: tags.fetchTags,
        createTag: tags.createTag,
        deleteTag: tags.deleteTag,
        fetchSceneTags: tags.fetchSceneTags,
        setSceneTags: tags.setSceneTags,

        // Actor operations
        fetchActors: actors.fetchActors,
        fetchActorByUUID: actors.fetchActorByUUID,
        fetchActorScenes: actors.fetchActorScenes,
        createActor: actors.createActor,
        updateActor: actors.updateActor,
        deleteActor: actors.deleteActor,
        uploadActorImage: actors.uploadActorImage,
        fetchSceneActors: actors.fetchSceneActors,
        setSceneActors: actors.setSceneActors,
        fetchActorInteractions: actors.fetchActorInteractions,
        setActorRating: actors.setActorRating,
        deleteActorRating: actors.deleteActorRating,
        toggleActorLike: actors.toggleActorLike,

        // Studio operations
        fetchStudios: studios.fetchStudios,
        fetchStudioByUUID: studios.fetchStudioByUUID,
        fetchStudioScenes: studios.fetchStudioScenes,
        createStudio: studios.createStudio,
        updateStudio: studios.updateStudio,
        deleteStudio: studios.deleteStudio,
        uploadStudioLogo: studios.uploadStudioLogo,
        fetchSceneStudio: studios.fetchSceneStudio,
        setSceneStudio: studios.setSceneStudio,
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
        getFolderSceneIDs: explorer.getFolderSceneIDs,

        // Saved search operations
        fetchSavedSearches: savedSearches.fetchSavedSearches,
        fetchSavedSearch: savedSearches.fetchSavedSearch,
        createSavedSearch: savedSearches.createSavedSearch,
        updateSavedSearch: savedSearches.updateSavedSearch,
        deleteSavedSearch: savedSearches.deleteSavedSearch,

        // Marker operations
        fetchMarkers: markers.fetchMarkers,
        createMarker: markers.createMarker,
        updateMarker: markers.updateMarker,
        deleteMarker: markers.deleteMarker,
        fetchLabelSuggestions: markers.fetchLabelSuggestions,
        fetchLabelGroups: markers.fetchLabelGroups,
        fetchMarkersByLabel: markers.fetchMarkersByLabel,
    };
};
