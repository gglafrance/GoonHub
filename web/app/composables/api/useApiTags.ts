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

    /**
     * Renames a tag and/or changes its color. A name used by another tag (also ignoring case
     * and extra spaces) answers 409 with that tag in `existing` (see getEntityConflict).
     */
    const updateTag = async (id: number, fields: { name?: string; color?: string }) => {
        const response = await fetch(`/api/v1/tags/${id}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(fields),
        });
        return handleResponse(response);
    };

    const updateTagColor = (id: number, color: string) => updateTag(id, { color });

    const fetchDuplicateTags = async () => {
        const response = await fetch('/api/v1/tags/duplicates', {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    /** Merges sourceIds into targetId; with dryRun only counts what would change. */
    const mergeTags = async (targetId: number, sourceIds: number[], dryRun = false) => {
        const response = await fetch('/api/v1/tags/merge', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ target_id: targetId, source_ids: sourceIds, dry_run: dryRun }),
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
        updateTag,
        updateTagColor,
        fetchDuplicateTags,
        mergeTags,
        fetchSceneTags,
        setSceneTags,
    };
};
