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

    return {
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
    };
};
