/**
 * Storage and scan API operations: paths, validation, scanning.
 */
export const useApiStorage = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const fetchStoragePaths = async () => {
        const response = await fetch('/api/v1/admin/storage-paths', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createStoragePath = async (name: string, path: string, isDefault: boolean) => {
        const response = await fetch('/api/v1/admin/storage-paths', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, path, is_default: isDefault }),
        });
        return handleResponse(response);
    };

    const updateStoragePath = async (
        id: number,
        name: string,
        path: string,
        isDefault: boolean,
    ) => {
        const response = await fetch(`/api/v1/admin/storage-paths/${id}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, path, is_default: isDefault }),
        });
        return handleResponse(response);
    };

    const deleteStoragePath = async (id: number) => {
        const response = await fetch(`/api/v1/admin/storage-paths/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const validateStoragePath = async (path: string) => {
        const response = await fetch('/api/v1/admin/storage-paths/validate', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ path }),
        });
        return handleResponse(response);
    };

    const startScan = async () => {
        const response = await fetch('/api/v1/admin/scan', {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const cancelScan = async () => {
        const response = await fetch('/api/v1/admin/scan/cancel', {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getScanStatus = async () => {
        const response = await fetch('/api/v1/admin/scan/status', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getScanHistory = async (page = 1, limit = 10) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/scan/history?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchStoragePaths,
        createStoragePath,
        updateStoragePath,
        deleteStoragePath,
        validateStoragePath,
        startScan,
        cancelScan,
        getScanStatus,
        getScanHistory,
    };
};
