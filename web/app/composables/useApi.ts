export const useApi = () => {
    const authStore = useAuthStore();

    // Common fetch options that include credentials for cookie-based auth
    const fetchOptions = (): RequestInit => ({
        credentials: 'include', // Send HTTP-only cookies
    });

    const getAuthHeaders = () => {
        const headers: Record<string, string> = {
            'Content-Type': 'application/json',
        };
        // No longer need to manually add Authorization header
        // as authentication is handled via HTTP-only cookies
        return headers;
    };

    const handleResponse = async (response: Response) => {
        if (response.status === 401) {
            authStore.logout();
            throw new Error('Unauthorized');
        }

        if (!response.ok) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }

        return response.json();
    };

    const uploadVideo = async (file: File, title?: string) => {
        const formData = new FormData();
        formData.append('video', file);
        if (title) {
            formData.append('title', title);
        }

        // No Authorization header needed - using cookies
        const response = await fetch('/api/v1/videos', {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const fetchVideos = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });

        const response = await fetch(`/api/v1/videos?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const searchVideos = async (searchParams: Record<string, string | number | undefined>) => {
        const params = new URLSearchParams();
        for (const [key, value] of Object.entries(searchParams)) {
            if (value !== undefined && value !== '' && value !== 0) {
                params.set(key, String(value));
            }
        }

        const response = await fetch(`/api/v1/videos?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const fetchFilterOptions = async () => {
        const response = await fetch('/api/v1/videos/filters', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchVideo = async (id: number) => {
        const response = await fetch(`/api/v1/videos/${id}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const fetchSettings = async () => {
        const response = await fetch('/api/v1/settings', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updatePlayerSettings = async (settings: {
        autoplay: boolean;
        default_volume: number;
        loop: boolean;
    }) => {
        const response = await fetch('/api/v1/settings/player', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const updateAppSettings = async (settings: {
        videos_per_page: number;
        default_sort_order: string;
    }) => {
        const response = await fetch('/api/v1/settings/app', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const updateTagSettings = async (settings: { default_tag_sort: string }) => {
        const response = await fetch('/api/v1/settings/tags', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(settings),
        });
        return handleResponse(response);
    };

    const changePassword = async (currentPassword: string, newPassword: string) => {
        const response = await fetch('/api/v1/settings/password', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
        });
        return handleResponse(response);
    };

    const changeUsername = async (username: string) => {
        const response = await fetch('/api/v1/settings/username', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ username }),
        });
        return handleResponse(response);
    };

    const fetchAdminUsers = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/users?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createUser = async (username: string, password: string, role: string) => {
        const response = await fetch('/api/v1/admin/users', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ username, password, role }),
        });
        return handleResponse(response);
    };

    const updateUserRole = async (userId: number, role: string) => {
        const response = await fetch(`/api/v1/admin/users/${userId}/role`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ role }),
        });
        return handleResponse(response);
    };

    const resetUserPassword = async (userId: number, newPassword: string) => {
        const response = await fetch(`/api/v1/admin/users/${userId}/password`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ new_password: newPassword }),
        });
        return handleResponse(response);
    };

    const deleteUser = async (userId: number) => {
        const response = await fetch(`/api/v1/admin/users/${userId}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchRoles = async () => {
        const response = await fetch('/api/v1/admin/roles', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchPermissions = async () => {
        const response = await fetch('/api/v1/admin/permissions', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const syncRolePermissions = async (roleId: number, permissionIds: number[]) => {
        const response = await fetch(`/api/v1/admin/roles/${roleId}/permissions`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ permission_ids: permissionIds }),
        });
        return handleResponse(response);
    };

    const fetchJobs = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/jobs?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchPoolConfig = async () => {
        const response = await fetch('/api/v1/admin/pool-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updatePoolConfig = async (config: {
        metadata_workers: number;
        thumbnail_workers: number;
        sprites_workers: number;
    }) => {
        const response = await fetch('/api/v1/admin/pool-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const fetchProcessingConfig = async () => {
        const response = await fetch('/api/v1/admin/processing-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateProcessingConfig = async (config: {
        max_frame_dimension_sm: number;
        max_frame_dimension_lg: number;
        frame_quality_sm: number;
        frame_quality_lg: number;
        frame_quality_sprites: number;
        sprites_concurrency: number;
    }) => {
        const response = await fetch('/api/v1/admin/processing-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const fetchTriggerConfig = async () => {
        const response = await fetch('/api/v1/admin/trigger-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateTriggerConfig = async (config: {
        phase: string;
        trigger_type: string;
        after_phase?: string | null;
        cron_expression?: string | null;
    }) => {
        const response = await fetch('/api/v1/admin/trigger-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const triggerVideoPhase = async (videoId: number, phase: string) => {
        const response = await fetch(`/api/v1/admin/videos/${videoId}/process/${phase}`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const triggerBulkPhase = async (phase: string, mode: string) => {
        const response = await fetch('/api/v1/admin/jobs/bulk', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ phase, mode }),
        });
        return handleResponse(response);
    };

    const extractThumbnail = async (videoId: number, timecode: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/thumbnail`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ timecode }),
        });
        return handleResponse(response);
    };

    const uploadThumbnail = async (videoId: number, file: File) => {
        const formData = new FormData();
        formData.append('thumbnail', file);

        // No Authorization header needed - using cookies
        const response = await fetch(`/api/v1/videos/${videoId}/thumbnail/upload`, {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchTags = async () => {
        const response = await fetch('/api/v1/tags', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createTag = async (name: string, color: string) => {
        const response = await fetch('/api/v1/tags', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, color }),
        });
        return handleResponse(response);
    };

    const deleteTag = async (id: number) => {
        const response = await fetch(`/api/v1/tags/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchVideoTags = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/tags`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setVideoTags = async (videoId: number, tagIds: number[]) => {
        const response = await fetch(`/api/v1/videos/${videoId}/tags`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ tag_ids: tagIds }),
        });
        return handleResponse(response);
    };

    const updateVideoDetails = async (
        videoId: number,
        title: string,
        description: string,
        releaseDate?: string | null,
    ) => {
        const payload: Record<string, unknown> = { title, description };
        if (releaseDate !== undefined) {
            payload.release_date = releaseDate;
        }
        const response = await fetch(`/api/v1/videos/${videoId}/details`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(payload),
        });
        return handleResponse(response);
    };

    const fetchVideoInteractions = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/interactions`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchVideoRating = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/rating`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setVideoRating = async (videoId: number, rating: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/rating`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ rating }),
        });
        return handleResponse(response);
    };

    const deleteVideoRating = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/rating`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        if (response.status === 401) {
            authStore.logout();
            throw new Error('Unauthorized');
        }
        if (!response.ok && response.status !== 204) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }
    };

    const fetchVideoLike = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/like`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const toggleVideoLike = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/like`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchJizzedCount = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/jizzed`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const incrementJizzed = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/jizzed`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const recordWatch = async (
        videoId: number,
        duration: number,
        position: number,
        completed: boolean,
    ) => {
        const response = await fetch(`/api/v1/videos/${videoId}/watch`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ duration, position, completed }),
        });
        return handleResponse(response);
    };

    const getResumePosition = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/resume`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getVideoWatchHistory = async (videoId: number, limit = 10) => {
        const params = new URLSearchParams({ limit: limit.toString() });
        const response = await fetch(`/api/v1/videos/${videoId}/history?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getUserWatchHistory = async (page = 1, limit = 20) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/history?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchStoragePaths = async () => {
        const response = await fetch('/api/v1/admin/storage-paths', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createStoragePath = async (name: string, path: string, isDefault: boolean) => {
        const response = await fetch('/api/v1/admin/storage-paths', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, path, is_default: isDefault }),
        });
        return handleResponse(response);
    };

    const updateStoragePath = async (
        id: number,
        name: string,
        path: string,
        isDefault: boolean,
    ) => {
        const response = await fetch(`/api/v1/admin/storage-paths/${id}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, path, is_default: isDefault }),
        });
        return handleResponse(response);
    };

    const deleteStoragePath = async (id: number) => {
        const response = await fetch(`/api/v1/admin/storage-paths/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const validateStoragePath = async (path: string) => {
        const response = await fetch('/api/v1/admin/storage-paths/validate', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ path }),
        });
        return handleResponse(response);
    };

    const startScan = async () => {
        const response = await fetch('/api/v1/admin/scan', {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const cancelScan = async () => {
        const response = await fetch('/api/v1/admin/scan/cancel', {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getScanStatus = async () => {
        const response = await fetch('/api/v1/admin/scan/status', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getScanHistory = async (page = 1, limit = 10) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/scan/history?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchDLQ = async (page = 1, limit = 50, status?: string) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        if (status) {
            params.set('status', status);
        }
        const response = await fetch(`/api/v1/admin/dlq?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const retryFromDLQ = async (jobId: string) => {
        const response = await fetch(`/api/v1/admin/dlq/${jobId}/retry`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const abandonDLQ = async (jobId: string) => {
        const response = await fetch(`/api/v1/admin/dlq/${jobId}/abandon`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchRetryConfig = async () => {
        const response = await fetch('/api/v1/admin/retry-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateRetryConfig = async (config: {
        phase: string;
        max_retries: number;
        initial_delay_seconds: number;
        max_delay_seconds: number;
        backoff_factor: number;
    }) => {
        const response = await fetch('/api/v1/admin/retry-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    // Actor CRUD
    const fetchActors = async (page = 1, limit = 20, query?: string) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        if (query) {
            params.set('q', query);
        }
        const response = await fetch(`/api/v1/actors?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchActorByUUID = async (uuid: string) => {
        const response = await fetch(`/api/v1/actors/${uuid}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchActorVideos = async (uuid: string, page = 1, limit = 20) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/actors/${uuid}/videos?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createActor = async (data: Record<string, unknown>) => {
        const response = await fetch('/api/v1/admin/actors', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const updateActor = async (id: number, data: Record<string, unknown>) => {
        const response = await fetch(`/api/v1/admin/actors/${id}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const deleteActor = async (id: number) => {
        const response = await fetch(`/api/v1/admin/actors/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const uploadActorImage = async (id: number, file: File) => {
        const formData = new FormData();
        formData.append('image', file);

        // No Authorization header needed - using cookies
        const response = await fetch(`/api/v1/admin/actors/${id}/image`, {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Actor-Video associations
    const fetchVideoActors = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/actors`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setVideoActors = async (videoId: number, actorIds: number[]) => {
        const response = await fetch(`/api/v1/videos/${videoId}/actors`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ actor_ids: actorIds }),
        });
        return handleResponse(response);
    };

    // Actor interactions (use UUID for routes)
    const fetchActorInteractions = async (actorUuid: string) => {
        const response = await fetch(`/api/v1/actors/${actorUuid}/interactions`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setActorRating = async (actorUuid: string, rating: number) => {
        const response = await fetch(`/api/v1/actors/${actorUuid}/rating`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ rating }),
        });
        return handleResponse(response);
    };

    const deleteActorRating = async (actorUuid: string) => {
        const response = await fetch(`/api/v1/actors/${actorUuid}/rating`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        if (response.status === 401) {
            authStore.logout();
            throw new Error('Unauthorized');
        }
        if (!response.ok && response.status !== 204) {
            const error = await response.json();
            throw new Error(error.error || 'Request failed');
        }
    };

    const toggleActorLike = async (actorUuid: string) => {
        const response = await fetch(`/api/v1/actors/${actorUuid}/like`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // PornDB integration
    const getPornDBStatus = async () => {
        const response = await fetch('/api/v1/admin/porndb/status', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const searchPornDBPerformers = async (query: string) => {
        const params = new URLSearchParams({ q: query });
        const response = await fetch(`/api/v1/admin/porndb/performers?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        const result = await handleResponse(response);
        return result.data || [];
    };

    const getPornDBPerformer = async (id: string) => {
        const response = await fetch(`/api/v1/admin/porndb/performers/${id}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        const result = await handleResponse(response);
        return result.data;
    };

    const searchPornDBScenes = async (params: {
        q?: string;
        title?: string;
        year?: number;
        site?: string;
    }) => {
        const searchParams = new URLSearchParams();
        if (params.q) searchParams.set('q', params.q);
        if (params.title) searchParams.set('title', params.title);
        if (params.year) searchParams.set('year', params.year.toString());
        if (params.site) searchParams.set('site', params.site);

        const response = await fetch(`/api/v1/admin/porndb/scenes?${searchParams}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        const result = await handleResponse(response);
        return result.data || [];
    };

    const getPornDBScene = async (id: string) => {
        const response = await fetch(`/api/v1/admin/porndb/scenes/${id}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        const result = await handleResponse(response);
        return result.data;
    };

    const applySceneMetadata = async (
        videoId: number,
        data: {
            title?: string;
            description?: string;
            studio?: string;
            thumbnail_url?: string;
            actor_ids?: number[];
            tag_names?: string[];
            release_date?: string;
            porndb_scene_id?: string;
        },
    ) => {
        const response = await fetch(`/api/v1/admin/videos/${videoId}/scene-metadata`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    return {
        uploadVideo,
        fetchVideos,
        searchVideos,
        fetchFilterOptions,
        fetchVideo,
        fetchSettings,
        updatePlayerSettings,
        updateAppSettings,
        updateTagSettings,
        changePassword,
        changeUsername,
        fetchAdminUsers,
        createUser,
        updateUserRole,
        resetUserPassword,
        deleteUser,
        fetchRoles,
        fetchPermissions,
        syncRolePermissions,
        fetchJobs,
        fetchPoolConfig,
        updatePoolConfig,
        fetchProcessingConfig,
        updateProcessingConfig,
        fetchTriggerConfig,
        updateTriggerConfig,
        triggerVideoPhase,
        triggerBulkPhase,
        extractThumbnail,
        uploadThumbnail,
        fetchTags,
        createTag,
        deleteTag,
        fetchVideoTags,
        setVideoTags,
        updateVideoDetails,
        fetchVideoInteractions,
        fetchVideoRating,
        setVideoRating,
        deleteVideoRating,
        fetchVideoLike,
        toggleVideoLike,
        fetchJizzedCount,
        incrementJizzed,
        recordWatch,
        getResumePosition,
        getVideoWatchHistory,
        getUserWatchHistory,
        fetchStoragePaths,
        createStoragePath,
        updateStoragePath,
        deleteStoragePath,
        validateStoragePath,
        startScan,
        cancelScan,
        getScanStatus,
        getScanHistory,
        fetchDLQ,
        retryFromDLQ,
        abandonDLQ,
        fetchRetryConfig,
        updateRetryConfig,
        fetchActors,
        fetchActorByUUID,
        fetchActorVideos,
        createActor,
        updateActor,
        deleteActor,
        uploadActorImage,
        fetchVideoActors,
        setVideoActors,
        fetchActorInteractions,
        setActorRating,
        deleteActorRating,
        toggleActorLike,
        getPornDBStatus,
        searchPornDBPerformers,
        getPornDBPerformer,
        searchPornDBScenes,
        getPornDBScene,
        applySceneMetadata,
    };
};
