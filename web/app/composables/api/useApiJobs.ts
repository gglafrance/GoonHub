/**
 * Job-related API operations: history, pool config, processing config, triggers.
 */
export const useApiJobs = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const fetchJobs = async (page: number, limit: number) => {
        const params = new URLSearchParams({
            page: page.toString(),
            limit: limit.toString(),
        });
        const response = await fetch(`/api/v1/admin/jobs?${params}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchPoolConfig = async () => {
        const response = await fetch('/api/v1/admin/pool-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updatePoolConfig = async (config: {
        metadata_workers: number;
        thumbnail_workers: number;
        sprites_workers: number;
    }) => {
        const response = await fetch('/api/v1/admin/pool-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const fetchProcessingConfig = async () => {
        const response = await fetch('/api/v1/admin/processing-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateProcessingConfig = async (config: {
        max_frame_dimension_sm: number;
        max_frame_dimension_lg: number;
        frame_quality_sm: number;
        frame_quality_lg: number;
        frame_quality_sprites: number;
        sprites_concurrency: number;
    }) => {
        const response = await fetch('/api/v1/admin/processing-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const fetchTriggerConfig = async () => {
        const response = await fetch('/api/v1/admin/trigger-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateTriggerConfig = async (config: {
        phase: string;
        trigger_type: string;
        after_phase?: string | null;
        cron_expression?: string | null;
    }) => {
        const response = await fetch('/api/v1/admin/trigger-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const triggerScenePhase = async (sceneId: number, phase: string) => {
        const response = await fetch(`/api/v1/admin/scenes/${sceneId}/process/${phase}`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const triggerBulkPhase = async (phase: string, mode: string) => {
        const response = await fetch('/api/v1/admin/jobs/bulk', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ phase, mode }),
        });
        return handleResponse(response);
    };

    const fetchRetryConfig = async () => {
        const response = await fetch('/api/v1/admin/retry-config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateRetryConfig = async (config: {
        phase: string;
        max_retries: number;
        initial_delay_seconds: number;
        max_delay_seconds: number;
        backoff_factor: number;
    }) => {
        const response = await fetch('/api/v1/admin/retry-config', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(config),
        });
        return handleResponse(response);
    };

    const cancelJob = async (jobID: string) => {
        const response = await fetch(`/api/v1/admin/jobs/${jobID}/cancel`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        fetchJobs,
        fetchPoolConfig,
        updatePoolConfig,
        fetchProcessingConfig,
        updateProcessingConfig,
        fetchTriggerConfig,
        updateTriggerConfig,
        triggerScenePhase,
        triggerBulkPhase,
        fetchRetryConfig,
        updateRetryConfig,
        cancelJob,
    };
};
