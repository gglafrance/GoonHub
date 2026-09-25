import type {
    ClipsFilter,
    SceneShortsResponse,
    ShortsConfig,
    ShortsFeedResponse,
    ShortsSearchFilter,
    ShortsSettings,
    ShortsSort,
    ShortTask,
} from '~/types/shorts';

/**
 * Shorts API: the vertical feed, creating shorts from a scene's A/B range,
 * and the admin shorts settings.
 */
export const useApiShorts = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const getShorts = async (params: {
        sort: ShortsSort;
        page: number;
        limit: number;
        seed?: number;
        clips?: ClipsFilter;
    }): Promise<ShortsFeedResponse> => {
        const query = new URLSearchParams({
            sort: params.sort,
            page: String(params.page),
            limit: String(params.limit),
        });
        if (params.seed) query.set('seed', String(params.seed));
        if (params.clips && params.clips !== 'all') query.set('clips', params.clips);

        const response = await fetch(`/api/v1/shorts?${query}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getShortsConfig = async (): Promise<ShortsConfig> => {
        const response = await fetch('/api/v1/shorts/config', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getSceneShorts = async (
        sceneId: number,
        page = 1,
        limit = 50,
    ): Promise<SceneShortsResponse> => {
        const query = new URLSearchParams({ page: String(page), limit: String(limit) });
        const response = await fetch(`/api/v1/scenes/${sceneId}/shorts?${query}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createShort = async (
        sceneId: number,
        body: { start: number; end: number; title?: string },
    ): Promise<{ task_id: string; task: ShortTask }> => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/shorts`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(body),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getShortsSettings = async (): Promise<ShortsSettings> => {
        const response = await fetch('/api/v1/admin/shorts-settings', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateShortsSettings = async (body: {
        max_duration: number;
        save_dir: string | null;
        search_default: ShortsSearchFilter;
    }): Promise<ShortsSettings> => {
        const response = await fetch('/api/v1/admin/shorts-settings', {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(body),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    return {
        getShorts,
        getShortsConfig,
        getSceneShorts,
        createShort,
        getShortsSettings,
        updateShortsSettings,
    };
};
