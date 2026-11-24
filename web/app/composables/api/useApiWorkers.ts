import type {
    CreatedWorker,
    RemoteWorker,
    RoutingMode,
    WorkerInput,
    WorkerRouting,
    WorkersListResponse,
} from '~/types/workers';

/**
 * Remote workers admin API: list, create (token shown once), edit, rotate the token, delete,
 * per-phase routing.
 */
export const useApiWorkers = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const fetchWorkers = async (): Promise<WorkersListResponse> => {
        const response = await fetch('/api/v1/admin/workers', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createWorker = async (input: WorkerInput): Promise<CreatedWorker> => {
        const response = await fetch('/api/v1/admin/workers', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(input),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateWorker = async (id: number, input: WorkerInput): Promise<RemoteWorker> => {
        const response = await fetch(`/api/v1/admin/workers/${id}`, {
            method: 'PATCH',
            headers: getAuthHeaders(),
            body: JSON.stringify(input),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const rotateWorkerToken = async (id: number): Promise<CreatedWorker> => {
        const response = await fetch(`/api/v1/admin/workers/${id}/token`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const deleteWorker = async (id: number): Promise<{ deleted: boolean }> => {
        const response = await fetch(`/api/v1/admin/workers/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateWorkerRouting = async (
        routing: Partial<Record<string, RoutingMode | ''>>,
    ): Promise<WorkerRouting> => {
        const response = await fetch('/api/v1/admin/worker-routing', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ routing }),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchWorkers,
        createWorker,
        updateWorker,
        rotateWorkerToken,
        deleteWorker,
        updateWorkerRouting,
    };
};
