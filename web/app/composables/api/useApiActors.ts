import type { ActorMergePreview, ActorResolution } from '~/types/actor';

/**
 * Actor-related API operations: CRUD, associations, and interactions.
 */
export const useApiActors = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    // Actor CRUD
    const fetchActors = async (
        page = 1,
        limit = 20,
        query?: string,
        sort?: string,
        genders?: string[],
    ) => {
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
        if (genders && genders.length > 0) {
            params.set('gender', genders.join(','));
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

    const fetchActorScenes = async (
        uuid: string,
        page = 1,
        limit = 20,
        query?: string,
        sort?: string,
        actorName?: string,
        seed?: number,
        cardFields?: string,
    ) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        if (actorName) params.set('actors', actorName);
        if (query) params.set('q', query);
        if (sort) params.set('sort', sort);
        if (sort === 'random' && seed) params.set('seed', seed.toString());
        if (cardFields) params.set('card_fields', cardFields);

        // Size filters and sorts need files:view_size (403 otherwise)
        stripRestrictedSceneParams(params, hasMyPermission);

        const response = await fetch(`/api/v1/scenes?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    // strict: a name or alias already used by another actor answers 409 with the existing
    // actor (see getEntityConflict). Without it, duplicate names are allowed and used aliases
    // are dropped (automatic flows such as PornDB matching).
    const createActor = async (
        data: Record<string, unknown>,
        options: { strict?: boolean } = {},
    ) => {
        const query = options.strict ? '?strict=true' : '';
        const response = await fetch(`/api/v1/admin/actors${query}`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const updateActor = async (
        id: number,
        data: Record<string, unknown>,
        options: { strict?: boolean } = {},
    ) => {
        const query = options.strict ? '?strict=true' : '';
        const response = await fetch(`/api/v1/admin/actors/${id}${query}`, {
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

    const fetchAllActorSceneIDs = async (uuid: string, actorName: string): Promise<number[]> => {
        const allIds: number[] = [];
        let page = 1;
        const limit = 100;
        let hasMore = true;

        while (hasMore) {
            const response = await fetchActorScenes(
                uuid,
                page,
                limit,
                undefined,
                undefined,
                actorName,
            );
            const sceneIds = response.data.map((scene: { id: number }) => scene.id);
            allIds.push(...sceneIds);
            hasMore = response.data.length === limit && allIds.length < response.total;
            page++;
        }
        return allIds;
    };

    // Exact name-or-alias match (case-insensitive); null when no actor has this name
    const resolveActor = async (name: string): Promise<ActorResolution | null> => {
        const params = new URLSearchParams({ name });
        const response = await fetch(`/api/v1/actors/resolve?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        if (response.status === 404) return null;
        return handleResponse(response);
    };

    // Merge: the source actor is merged into the target actor (kept)
    const fetchActorMergePreview = async (
        targetUuid: string,
        sourceUuid: string,
    ): Promise<ActorMergePreview> => {
        const params = new URLSearchParams({ source_uuid: sourceUuid });
        const response = await fetch(`/api/v1/actors/${targetUuid}/merge-preview?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const mergeActor = async (
        targetUuid: string,
        sourceUuid: string,
    ): Promise<ActorMergePreview> => {
        const response = await fetch(`/api/v1/actors/${targetUuid}/merge`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ source_uuid: sourceUuid }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchActors,
        resolveActor,
        fetchActorMergePreview,
        mergeActor,
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
