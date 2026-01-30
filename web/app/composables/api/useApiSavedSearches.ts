import type {
    SavedSearch,
    SavedSearchListResponse,
    CreateSavedSearchInput,
    UpdateSavedSearchInput,
} from '~/types/saved_search';

/**
 * Saved search API operations: CRUD for user's saved search templates.
 */
export const useApiSavedSearches = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    const fetchSavedSearches = async (): Promise<SavedSearchListResponse> => {
        const response = await fetch('/api/v1/saved-searches', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchSavedSearch = async (uuid: string): Promise<SavedSearch> => {
        const response = await fetch(`/api/v1/saved-searches/${uuid}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createSavedSearch = async (data: CreateSavedSearchInput): Promise<SavedSearch> => {
        const response = await fetch('/api/v1/saved-searches', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const updateSavedSearch = async (
        uuid: string,
        data: UpdateSavedSearchInput,
    ): Promise<SavedSearch> => {
        const response = await fetch(`/api/v1/saved-searches/${uuid}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const deleteSavedSearch = async (uuid: string): Promise<void> => {
        const response = await fetch(`/api/v1/saved-searches/${uuid}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        await handleResponseWithNoContent(response);
    };

    return {
        fetchSavedSearches,
        fetchSavedSearch,
        createSavedSearch,
        updateSavedSearch,
        deleteSavedSearch,
    };
};
