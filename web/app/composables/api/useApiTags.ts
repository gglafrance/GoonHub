/**
 * Tag-related API operations: CRUD and scene-tag associations.
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

    const fetchSceneTags = async (sceneId: number) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/tags`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setSceneTags = async (sceneId: number, tagIds: number[]) => {
        const response = await fetch(`/api/v1/scenes/${sceneId}/tags`, {
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
        fetchSceneTags,
        setSceneTags,
    };
};
