import type {
    PlaylistListItem,
    PlaylistDetail,
    PlaylistListResponse,
    CreatePlaylistInput,
    UpdatePlaylistInput,
    PlaylistTag,
    PlaylistResume,
} from '~/types/playlist';

/**
 * Playlist API operations: CRUD, scenes, tags, likes, progress.
 */
export const useApiPlaylists = () => {
    const { fetchOptions, getAuthHeaders, handleResponse, handleResponseWithNoContent } =
        useApiCore();

    const fetchPlaylists = async (
        params: Record<string, string | number | undefined> = {},
    ): Promise<PlaylistListResponse> => {
        const query = new URLSearchParams();
        for (const [key, value] of Object.entries(params)) {
            if (value !== undefined && value !== '') {
                query.set(key, String(value));
            }
        }
        const qs = query.toString();
        const url = `/api/v1/playlists${qs ? `?${qs}` : ''}`;
        const response = await fetch(url, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const fetchPlaylist = async (uuid: string): Promise<PlaylistDetail> => {
        const response = await fetch(`/api/v1/playlists/${uuid}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const createPlaylist = async (data: CreatePlaylistInput): Promise<PlaylistListItem> => {
        const response = await fetch('/api/v1/playlists', {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const updatePlaylist = async (
        uuid: string,
        data: UpdatePlaylistInput,
    ): Promise<PlaylistListItem> => {
        const response = await fetch(`/api/v1/playlists/${uuid}`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify(data),
        });
        return handleResponse(response);
    };

    const deletePlaylist = async (uuid: string): Promise<void> => {
        const response = await fetch(`/api/v1/playlists/${uuid}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        await handleResponseWithNoContent(response);
    };

    const addScenes = async (uuid: string, sceneIDs: number[]): Promise<void> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/scenes`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_ids: sceneIDs }),
        });
        await handleResponseWithNoContent(response);
    };

    const removeScene = async (uuid: string, sceneID: number): Promise<void> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/scenes/${sceneID}`, {
            method: 'DELETE',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        await handleResponseWithNoContent(response);
    };

    const removeScenes = async (uuid: string, sceneIDs: number[]): Promise<void> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/scenes/remove`, {
            method: 'POST',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_ids: sceneIDs }),
        });
        await handleResponseWithNoContent(response);
    };

    const reorderScenes = async (uuid: string, sceneIDs: number[]): Promise<void> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/scenes/reorder`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_ids: sceneIDs }),
        });
        await handleResponseWithNoContent(response);
    };

    const fetchPlaylistTags = async (uuid: string): Promise<PlaylistTag[]> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/tags`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const setPlaylistTags = async (uuid: string, tagIDs: number[]): Promise<void> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/tags`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ tag_ids: tagIDs }),
        });
        await handleResponseWithNoContent(response);
    };

    const toggleLike = async (uuid: string): Promise<{ liked: boolean }> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/like`, {
            method: 'POST',
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getLikeStatus = async (uuid: string): Promise<{ liked: boolean }> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/like`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const getProgress = async (uuid: string): Promise<PlaylistResume> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/progress`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
        });
        return handleResponse(response);
    };

    const updateProgress = async (
        uuid: string,
        sceneID: number,
        positionS: number,
    ): Promise<void> => {
        const response = await fetch(`/api/v1/playlists/${uuid}/progress`, {
            method: 'PUT',
            headers: getAuthHeaders(),
            body: JSON.stringify({ scene_id: sceneID, position_s: positionS }),
        });
        await handleResponseWithNoContent(response);
    };

    return {
        fetchPlaylists,
        fetchPlaylist,
        createPlaylist,
        updatePlaylist,
        deletePlaylist,
        addScenes,
        removeScene,
        removeScenes,
        reorderScenes,
        fetchPlaylistTags,
        setPlaylistTags,
        toggleLike,
        getLikeStatus,
        getProgress,
        updateProgress,
    };
};
