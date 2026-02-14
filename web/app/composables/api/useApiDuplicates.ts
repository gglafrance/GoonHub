export const useApiDuplicates = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const listGroups = async (page: number, limit: number, status?: string, sortBy?: string) => {
        const params = new URLSearchParams({ page: String(page), limit: String(limit) });
        if (status) params.set('status', status);
        if (sortBy) params.set('sort_by', sortBy);
        const response = await fetch(`/api/v1/admin/duplicates?${params}`, {
            ...fetchOptions(),
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const getGroup = async (id: number) => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}`, {
            ...fetchOptions(),
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const getStats = async () => {
        const response = await fetch('/api/v1/admin/duplicates/stats', {
            ...fetchOptions(),
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const resolveGroup = async (id: number, bestSceneId: number, mergeMetadata: boolean) => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}/resolve`, {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ best_scene_id: bestSceneId, merge_metadata: mergeMetadata }),
        });
        return handleResponse(response);
    };

    const dismissGroup = async (id: number) => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}/dismiss`, {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const setBest = async (id: number, sceneId: number) => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}/best`, {
            ...fetchOptions(),
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_id: sceneId }),
        });
        return handleResponse(response);
    };

    const getConfig = async () => {
        const response = await fetch('/api/v1/admin/duplicates/config', {
            ...fetchOptions(),
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const updateConfig = async (config: Record<string, unknown>) => {
        const response = await fetch('/api/v1/admin/duplicates/config', {
            ...fetchOptions(),
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    return {
        listGroups,
        getGroup,
        getStats,
        resolveGroup,
        dismissGroup,
        setBest,
        getConfig,
        updateConfig,
    };
};
