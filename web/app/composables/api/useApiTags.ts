/**
 * Tag-related API operations: CRUD and video-tag associations.
 */
export const useApiTags = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const fetchTags = async () => {
        const response = await fetch('/api/v1/tags', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createTag = async (name: string, color: string) => {
        const response = await fetch('/api/v1/tags', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ name, color }),
        });
        return handleResponse(response);
    };

    const deleteTag = async (id: number) => {
        const response = await fetch(`/api/v1/tags/${id}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchVideoTags = async (videoId: number) => {
        const response = await fetch(`/api/v1/videos/${videoId}/tags`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setVideoTags = async (videoId: number, tagIds: number[]) => {
        const response = await fetch(`/api/v1/videos/${videoId}/tags`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ tag_ids: tagIds }),
        });
        return handleResponse(response);
    };

    return {
        fetchTags,
        createTag,
        deleteTag,
        fetchVideoTags,
        setVideoTags,
    };
};
