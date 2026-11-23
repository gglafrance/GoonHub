import type {
    AutoSelectResult,
    BulkResolveResult,
    BulkTrashAllResult,
    ExcludeMembersResult,
    ResolveGroupResult,
    SceneCopyBulkResult,
    SceneCopyIds,
    SceneCopyListResponse,
    SelectableDuplicateIds,
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

    // IDs of the unresolved groups of a kind ("" = every kind), for "select all"
    const listSelectableIds = async (kind?: string): Promise<SelectableDuplicateIds> => {
        const params = new URLSearchParams();
        if (kind) params.set('kind', kind);
        const response = await fetch(`/api/v1/admin/duplicates/selectable-ids?${params}`, {
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

    // Removes scenes that are not duplicates from a group (never trashes nor deletes anything)
    const excludeMembers = async (
        id: number,
        sceneIds: number[],
    ): Promise<ExcludeMembersResult> => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}/exclude`, {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_ids: sceneIds }),
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

    // Copies recorded by the scan (Duplicates page > Copies); params from buildCopyQuery
    const listCopies = async (params: URLSearchParams): Promise<SceneCopyListResponse> => {
        const response = await fetch(`/api/v1/admin/duplicates/copies?${params}`, {
            ...fetchOptions(),
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    const listCopyIds = async (params: URLSearchParams): Promise<SceneCopyIds> => {
        const response = await fetch(`/api/v1/admin/duplicates/copies/ids?${params}`, {
            ...fetchOptions(),
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    // action: make-main, separate or forget (no file is touched)
    const copyAction = async (copyId: number, action: 'make-main' | 'separate' | 'forget') => {
        const response = await fetch(`/api/v1/admin/duplicates/copies/${copyId}/${action}`, {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
        });
        return handleResponse(response);
    };

    // Permanently deletes the copy's file from disk (after the server checks)
    const deleteCopyFile = async (copyId: number): Promise<{ freed_bytes: number }> => {
        const response = await fetch(`/api/v1/admin/duplicates/copies/${copyId}/delete-file`, {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ confirm: true }),
        });
        return handleResponse(response);
    };

    const forgetCopiesBulk = async (copyIds: number[]): Promise<SceneCopyBulkResult> => {
        const response = await fetch('/api/v1/admin/duplicates/copies/forget-bulk', {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ copy_ids: copyIds }),
        });
        return handleResponse(response);
    };

    const deleteCopyFilesBulk = async (copyIds: number[]): Promise<SceneCopyBulkResult> => {
        const response = await fetch('/api/v1/admin/duplicates/copies/delete-files-bulk', {
            ...fetchOptions(),
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ copy_ids: copyIds, confirm: true }),
        });
        return handleResponse(response);
    };

    return {
        listCopies,
        listCopyIds,
        copyAction,
        deleteCopyFile,
        forgetCopiesBulk,
        deleteCopyFilesBulk,
        listGroups,
        listSelectableIds,
        getGroup,
        getStats,
        resolveGroup,
        dismissGroup,
        setBest,
        autoSelectBest,
        resolveBulk,
        trashAll,
        trashAllBulk,
        excludeMembers,
        cancelDeletion,
        getConfig,
        updateConfig,
    };
};
