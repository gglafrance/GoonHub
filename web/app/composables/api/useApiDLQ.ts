/**
 * Dead Letter Queue (DLQ) API operations: retry, abandon failed jobs.
 */
export const useApiDLQ = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const fetchDLQ = async (page = 1, limit = 50, status?: string) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        if (status) {
            params.set('status', status);
        }
        const response = await fetch(`/api/v1/admin/dlq?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const retryFromDLQ = async (jobId: string) => {
        const response = await fetch(`/api/v1/admin/dlq/${jobId}/retry`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const abandonDLQ = async (jobId: string) => {
        const response = await fetch(`/api/v1/admin/dlq/${jobId}/abandon`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchDLQ,
        retryFromDLQ,
        abandonDLQ,
    };
};
