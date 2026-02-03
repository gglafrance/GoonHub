/**
 * Actor-related API operations: CRUD, associations, and interactions.
 */
export const useApiActors = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    // Actor CRUD
    const fetchActors = async (page = 1, limit = 20, query?: string, sort?: string) => {
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

    const fetchActorScenes = async (uuid: string, page = 1, limit = 20) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/actors/${uuid}/scenes?${params}`, {
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
        await handleResponseWithNoContent(response);
    };

    const uploadActorImage = async (id: number, file: File) => {
        const formData = new FormData();
        formData.append('image', file);

        const response = await fetch(`/api/v1/admin/actors/${id}/image`, {
            method: 'POST',
            body: formData,
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // Actor-Scene associations
    const fetchSceneActors = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/actors`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setSceneActors = async (sceneId: number, actorIds: number[]) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/actors`, {
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
        await handleResponseWithNoContent(response);
    };

    const toggleActorLike = async (actorUuid: string) => {
        const response = await fetch(`/api/v1/actors/${actorUuid}/like`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchAllActorSceneIDs = async (uuid: string): Promise<number[]> => {
        const allIds: number[] = [];
        let page = 1;
        const limit = 100;
        let hasMore = true;

        while (hasMore) {
            const response = await fetchActorScenes(uuid, page, limit);
            const sceneIds = response.data.map((scene: { id: number }) => scene.id);
            allIds.push(...sceneIds);
            hasMore = response.data.length === limit && allIds.length < response.total;
            page++;
        }
        return allIds;
    };

    return {
        fetchActors,
        fetchActorByUUID,
        fetchActorScenes,
        createActor,
        updateActor,
        deleteActor,
        uploadActorImage,
        fetchSceneActors,
        setSceneActors,
        fetchActorInteractions,
        setActorRating,
        deleteActorRating,
        toggleActorLike,
        fetchAllActorSceneIDs,
    };
};
