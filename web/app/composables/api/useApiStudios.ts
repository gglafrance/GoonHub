import type { StudioMergePreview, StudioResolution } from '~/types/studio';

/**
 * Studio-related API operations: CRUD, associations, and interactions.
 */
export const useApiStudios = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    // Studio CRUD
    const fetchStudios = async (page = 1, limit = 20, query?: string, sort?: string) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        if (query) {
            params.set('q', query);
        }
        if (sort) {
            params.set('sort', sort);
        }
        const response = await fetch(`/api/v1/studios?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchStudioByUUID = async (uuid: string) => {
        const response = await fetch(`/api/v1/studios/${uuid}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchStudioScenes = async (
        uuid: string,
        page = 1,
        limit = 20,
        query?: string,
        sort?: string,
        studioName?: string,
        seed?: number,
    ) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        if (studioName) params.set('studio', studioName);
        if (query) params.set('q', query);
        if (sort) params.set('sort', sort);
        if (sort === 'random' && seed) params.set('seed', seed.toString());

        const response = await fetch(`/api/v1/scenes?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // strict: a name or alias already used by another studio answers 409 with the existing
    // studio (see getEntityConflict). Without it, duplicate names are allowed and used aliases
    // are dropped (automatic flows such as PornDB matching).
    const createStudio = async (
        data: Record<string, unknown>,
        options: { strict?: boolean } = {},
    ) => {
        const query = options.strict ? '?strict=true' : '';
        const response = await fetch(`/api/v1/admin/studios${query}`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const updateStudio = async (
        id: number,
        data: Record<string, unknown>,
        options: { strict?: boolean } = {},
    ) => {
        const query = options.strict ? '?strict=true' : '';
        const response = await fetch(`/api/v1/admin/studios/${id}${query}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const deleteStudio = async (id: number) => {
        const response = await fetch(`/api/v1/admin/studios/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        await handleResponseWithNoContent(response);
    };

    const uploadStudioLogo = async (id: number, file: File) => {
        const formData = new FormData();
        formData.append('logo', file);

        const response = await fetch(`/api/v1/admin/studios/${id}/logo`, {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Scene-Studio association (one-to-many: scene has one studio)
    const fetchSceneStudio = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/studio`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setSceneStudio = async (sceneId: number, studioId: number | null) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/studio`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ studio_id: studioId }),
        });
        return handleResponse(response);
    };

    // Studio interactions (use UUID for routes)
    const fetchStudioInteractions = async (studioUuid: string) => {
        const response = await fetch(`/api/v1/studios/${studioUuid}/interactions`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setStudioRating = async (studioUuid: string, rating: number) => {
        const response = await fetch(`/api/v1/studios/${studioUuid}/rating`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ rating }),
        });
        return handleResponse(response);
    };

    const deleteStudioRating = async (studioUuid: string) => {
        const response = await fetch(`/api/v1/studios/${studioUuid}/rating`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        await handleResponseWithNoContent(response);
    };

    const toggleStudioLike = async (studioUuid: string) => {
        const response = await fetch(`/api/v1/studios/${studioUuid}/like`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchAllStudioSceneIDs = async (uuid: string, studioName: string): Promise<number[]> => {
        const allIds: number[] = [];
        let page = 1;
        const limit = 100;
        let hasMore = true;

        while (hasMore) {
            const response = await fetchStudioScenes(
                uuid,
                page,
                limit,
                undefined,
                undefined,
                studioName,
            );
            const sceneIds = response.data.map((scene: { id: number }) => scene.id);
            allIds.push(...sceneIds);
            hasMore = response.data.length === limit && allIds.length < response.total;
            page++;
        }
        return allIds;
    };

    // Exact name-or-alias match (case-insensitive); null when no studio has this name
    const resolveStudio = async (name: string): Promise<StudioResolution | null> => {
        const params = new URLSearchParams({ name });
        const response = await fetch(`/api/v1/studios/resolve?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        if (response.status === 404) return null;
        return handleResponse(response);
    };

    // Merge: the source studio is merged into the target studio (kept)
    const fetchStudioMergePreview = async (
        targetUuid: string,
        sourceUuid: string,
    ): Promise<StudioMergePreview> => {
        const params = new URLSearchParams({ source_uuid: sourceUuid });
        const response = await fetch(`/api/v1/studios/${targetUuid}/merge-preview?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const mergeStudio = async (
        targetUuid: string,
        sourceUuid: string,
    ): Promise<StudioMergePreview> => {
        const response = await fetch(`/api/v1/studios/${targetUuid}/merge`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ source_uuid: sourceUuid }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchStudios,
        resolveStudio,
        fetchStudioMergePreview,
        mergeStudio,
        fetchStudioByUUID,
        fetchStudioScenes,
        createStudio,
        updateStudio,
        deleteStudio,
        uploadStudioLogo,
        fetchSceneStudio,
        setSceneStudio,
        fetchStudioInteractions,
        setStudioRating,
        deleteStudioRating,
        toggleStudioLike,
        fetchAllStudioSceneIDs,
    };
};
