import type {
    AutoSelectResult,
    BulkResolveResult,
    BulkTrashAllResult,
    ResolveGroupResult,
    TrashAllResult,
} from '~/types/duplicates';

export const useApiDuplicates = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    // kind: identical, containment, partial or unverified (empty: every group)
    const listGroups = async (
        page: number,
        limit: number,
        status?: string,
        sortBy?: string,
        kind?: string,
    ) => {
        const params = new URLSearchParams({ page: String(page), limit: String(limit) });
        if (status) params.set('status', status);
        if (kind) params.set('kind', kind);
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

    const resolveGroup = async (
        id: number,
        bestSceneId: number,
        mergeMetadata: boolean,
    ): Promise<ResolveGroupResult> => {
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

    const autoSelectBest = async (groupIds: number[]): Promise<AutoSelectResult> => {
        const response = await fetch('/api/v1/admin/duplicates/auto-select', {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ group_ids: groupIds }),
        });
        return handleResponse(response);
    };

    const resolveBulk = async (
        groupIds: number[],
        autoSelect: boolean,
        mergeMetadata: boolean,
    ): Promise<BulkResolveResult> => {
        const response = await fetch('/api/v1/admin/duplicates/resolve-bulk', {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({
                group_ids: groupIds,
                auto_select: autoSelect,
                merge_metadata: mergeMetadata,
            }),
        });
        return handleResponse(response);
    };

    // Moves every scene of the group to the trash (the best one too) and resolves the group
    const trashAll = async (id: number): Promise<TrashAllResult> => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}/trash-all`, {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const trashAllBulk = async (groupIds: number[]): Promise<BulkTrashAllResult> => {
        const response = await fetch('/api/v1/admin/duplicates/trash-all-bulk', {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ group_ids: groupIds }),
        });
        return handleResponse(response);
    };

    const cancelDeletion = async (groupId: number, sceneId: number) => {
        const response = await fetch(`/api/v1/admin/duplicates/${groupId}/cancel-deletion`, {
            ...fetchOptions(),
            method: 'POST',
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
        autoSelectBest,
        resolveBulk,
        trashAll,
        trashAllBulk,
        cancelDeletion,
        getConfig,
        updateConfig,
    };
};
