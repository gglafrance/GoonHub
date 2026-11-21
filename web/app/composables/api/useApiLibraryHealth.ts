import type {
    HealthAction,
    HealthActionPreview,
    HealthCategory,
    HealthCheck,
    HealthSceneList,
    HealthTaskKind,
    HealthTaskStatus,
    LibraryHealthSummary,
} from '~/types/libraryHealth';

/**
 * Library health API: counts, drill-down lists and the background file checks (admin only).
 */
export const useApiLibraryHealth = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const getSummary = async (): Promise<LibraryHealthSummary> => {
        const response = await fetch('/api/v1/admin/library-health', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const listScenes = async (
        category: HealthCategory,
        page = 1,
        limit = 20,
    ): Promise<HealthSceneList> => {
        const params = new URLSearchParams({
            category,
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/library-health/scenes?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const previewAction = async (action: HealthAction): Promise<HealthActionPreview> => {
        const params = new URLSearchParams({ action });
        const response = await fetch(`/api/v1/admin/library-health/preview?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const startTask = async (kind: HealthTaskKind): Promise<HealthCheck> => {
        const response = await fetch('/api/v1/admin/library-health/tasks', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ kind }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const cancelTask = async () => {
        const response = await fetch('/api/v1/admin/library-health/tasks/cancel', {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getTaskStatus = async (): Promise<HealthTaskStatus> => {
        const response = await fetch('/api/v1/admin/library-health/tasks/status', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return { getSummary, listScenes, previewAction, startTask, cancelTask, getTaskStatus };
};
