import type { ShareLinksResponse } from '~/types/share';

/**
 * Share link API operations: create, list, delete, resolve.
 */
export const useApiShares = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    const createShareLink = async (sceneId: number, shareType: string, expiresIn: string) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/shares`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ share_type: shareType, expires_in: expiresIn }),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const listShareLinks = async (sceneId: number): Promise<ShareLinksResponse> => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/shares`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        return handleResponse(response);
    };

    const deleteShareLink = async (linkId: number) => {
        const response = await fetch(`/api/v1/shares/${linkId}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });

        await handleResponseWithNoContent(response);
    };

    const resolveShareLink = async (token: string) => {
        const response = await fetch(`/api/v1/shares/${token}`, {
            ...fetchOptions(),
        });

        // Don't use handleResponse because it auto-logs out on 401
        if (!response.ok) {
            const errorBody = await response.json();
            const err = new Error(errorBody.error || 'Request failed') as Error & {
                status: number;
                code: string;
            };
            err.status = response.status;
            err.code = errorBody.code;
            throw err;
        }

        return response.json();
    };

    return {
        createShareLink,
        listShareLinks,
        deleteShareLink,
        resolveShareLink,
    };
};
