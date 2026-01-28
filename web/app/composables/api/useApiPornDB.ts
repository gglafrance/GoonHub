/**
 * PornDB integration API operations: search, performers, scenes, metadata.
 */
export const useApiPornDB = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

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
        getPornDBStatus,
        searchPornDBPerformers,
        getPornDBPerformer,
        searchPornDBScenes,
        getPornDBScene,
        applySceneMetadata,
    };
};
