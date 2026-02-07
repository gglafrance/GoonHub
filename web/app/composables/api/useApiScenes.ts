/**
 * Scene-related API operations: CRUD, search, streaming, filters, interactions.
 */
export const useApiScenes = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    const uploadScene = async (file: File, title?: string) => {
        const formData = new FormData();
        formData.append('scene', file);
        if (title) {
            formData.append('title', title);
        }

        const response = await fetch('/api/v1/scenes', {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const fetchScenes = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });

        const settingsStore = useSettingsStore();
        if (settingsStore.cardFieldsParam) {
            params.set('card_fields', settingsStore.cardFieldsParam);
        }

        const response = await fetch(`/api/v1/scenes?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const searchScenes = async (searchParams: Record<string, string | number | undefined>) => {
        const params = new URLSearchParams();
        for (const [key, value] of Object.entries(searchParams)) {
            if (value !== undefined && value !== '' && value !== 0) {
                params.set(key, String(value));
            }
        }

        const settingsStore = useSettingsStore();
        if (settingsStore.cardFieldsParam) {
            params.set('card_fields', settingsStore.cardFieldsParam);
        }

        const response = await fetch(`/api/v1/scenes?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const fetchFilterOptions = async () => {
        const response = await fetch('/api/v1/scenes/filters', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchScene = async (id: number) => {
        const response = await fetch(`/api/v1/scenes/${id}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const updateSceneDetails = async (
        sceneId: number,
        title: string,
        description: string,
        releaseDate?: string | null,
    ) => {
        const payload: Record<string, unknown> = { title, description };
        if (releaseDate !== undefined) {
            payload.release_date = releaseDate;
        }
        const response = await fetch(`/api/v1/scenes/${sceneId}/details`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(payload),
        });
        return handleResponse(response);
    };

    const extractThumbnail = async (sceneId: number, timecode: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/thumbnail`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ timecode }),
        });
        return handleResponse(response);
    };

    const uploadThumbnail = async (sceneId: number, file: File) => {
        const formData = new FormData();
        formData.append('thumbnail', file);

        const response = await fetch(`/api/v1/scenes/${sceneId}/thumbnail/upload`, {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Scene interactions
    const fetchSceneInteractions = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/interactions`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchSceneRating = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/rating`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setSceneRating = async (sceneId: number, rating: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/rating`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ rating }),
        });
        return handleResponse(response);
    };

    const deleteSceneRating = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/rating`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        await handleResponseWithNoContent(response);
    };

    const fetchSceneLike = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/like`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const toggleSceneLike = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/like`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchJizzedCount = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/jizzed`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const incrementJizzed = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/jizzed`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Watch tracking
    const recordWatch = async (
        sceneId: number,
        duration: number,
        position: number,
        completed: boolean,
    ) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/watch`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ duration, position, completed }),
        });
        return handleResponse(response);
    };

    const getResumePosition = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/resume`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getSceneWatchHistory = async (sceneId: number, limit = 10) => {
        const params = new URLSearchParams({ limit: limit.toString() });
        const response = await fetch(`/api/v1/scenes/${sceneId}/history?${params}`, {
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

    const getUserWatchHistoryByDateRange = async (rangeDays = 30, limit = 2000) => {
        const params = new URLSearchParams({
            range: rangeDays.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/history/by-date?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getDailyActivity = async (rangeDays = 30) => {
        const params = new URLSearchParams({
            range: rangeDays.toString(),
        });
        const response = await fetch(`/api/v1/history/activity?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getUserWatchHistoryByTimeRange = async (since: string, until: string, limit = 2000) => {
        const params = new URLSearchParams({
            since,
            until,
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/history/by-date?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchRelatedScenes = async (sceneId: number, limit = 12) => {
        const params = new URLSearchParams({ limit: limit.toString() });
        const response = await fetch(`/api/v1/scenes/${sceneId}/related?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const deleteScene = async (sceneId: number, permanent = false) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}`, {
            method: 'DELETE',
            headers: { ...getAuthHeaders(), 'Content-Type': 'application/json' },
            body: JSON.stringify({ permanent }),
            ...fetchOptions(),
        });

        if (permanent) {
            return handleResponseWithNoContent(response);
        }
        return handleResponse(response);
    };

    return {
        uploadScene,
        fetchScenes,
        searchScenes,
        fetchFilterOptions,
        fetchScene,
        updateSceneDetails,
        extractThumbnail,
        uploadThumbnail,
        fetchSceneInteractions,
        fetchSceneRating,
        setSceneRating,
        deleteSceneRating,
        fetchSceneLike,
        toggleSceneLike,
        fetchJizzedCount,
        incrementJizzed,
        recordWatch,
        getResumePosition,
        getSceneWatchHistory,
        getUserWatchHistory,
        getUserWatchHistoryByDateRange,
        getUserWatchHistoryByTimeRange,
        getDailyActivity,
        fetchRelatedScenes,
        deleteScene,
    };
};
