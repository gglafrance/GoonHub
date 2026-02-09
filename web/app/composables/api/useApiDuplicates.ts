import type { DuplicateConfig, DuplicateGroup, DuplicateGroupListResponse, RescanStatus } from '~/types/duplicates';

export const useApiDuplicates = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const getConfig = async (): Promise<DuplicateConfig> => {
        const response = await fetch('/api/v1/admin/duplicates/config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateConfig = async (config: DuplicateConfig): Promise<DuplicateConfig> => {
        const response = await fetch('/api/v1/admin/duplicates/config', {
            method: 'PUT',
            headers: { ...getAuthHeaders(), 'Content-Type': 'application/json' },
            body: JSON.stringify(config),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const listGroups = async (page = 1, limit = 20, status?: string): Promise<DuplicateGroupListResponse> => {
        const params = new URLSearchParams({ page: page.toString(), limit: limit.toString() });
        if (status) params.set('status', status);
        const response = await fetch(`/api/v1/admin/duplicates?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getGroup = async (id: number): Promise<DuplicateGroup> => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const resolveGroup = async (id: number): Promise<void> => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}/resolve`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const dismissGroup = async (id: number): Promise<void> => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}/dismiss`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setWinner = async (groupId: number, sceneId: number): Promise<void> => {
        const response = await fetch(`/api/v1/admin/duplicates/${groupId}/winner`, {
            method: 'POST',
            headers: { ...getAuthHeaders(), 'Content-Type': 'application/json' },
            body: JSON.stringify({ scene_id: sceneId }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const deleteGroup = async (id: number): Promise<void> => {
        const response = await fetch(`/api/v1/admin/duplicates/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const startRescan = async (): Promise<void> => {
        const response = await fetch('/api/v1/admin/duplicates/rescan', {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getRescanStatus = async (): Promise<RescanStatus> => {
        const response = await fetch('/api/v1/admin/duplicates/rescan/status', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        getConfig,
        updateConfig,
        listGroups,
        getGroup,
        resolveGroup,
        dismissGroup,
        setWinner,
        deleteGroup,
        startRescan,
        getRescanStatus,
    };
};
