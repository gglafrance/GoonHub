import type { SceneListItem } from '~/types/scene';
import type { WidgetMarker } from '~/types/homepageMarkerWidgets';
import type {
    CustomList,
    CustomListInput,
    ImageCrop,
    Profile,
    ProfileEdit,
    ProfileLayoutBlock,
    ProfileListKind,
    ProfileListResponse,
    ProfileRatingsPayload,
    ProfileTabConfig,
} from '~/types/profile';

/** Parameters of a page of a profile list (rating: the ratings list only). */
export interface ProfileListParams {
    page: number;
    limit: number;
    sort?: string;
    rating?: number | null;
    cardFields?: string;
}

/**
 * Profile pages API (docs/PROFILES.md): a profile as the viewer sees it, and the edits of the
 * signed-in user's own profile.
 */
export const useApiProfiles = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
        const response = await fetch(`/api/v1/profiles${path}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
            ...init,
        });
        return (await handleResponse(response)) as T;
    };

    const json = (method: string, body: unknown): RequestInit => ({
        method,
        body: JSON.stringify(body),
    });

    // Multipart: no JSON Content-Type header, the browser sets the boundary
    const upload = async (path: string, file: File, crop?: ImageCrop | null) => {
        const form = new FormData();
        form.append('image', file);
        if (crop) {
            form.append('crop_x', String(crop.x));
            form.append('crop_y', String(crop.y));
            form.append('crop_w', String(crop.w));
            form.append('crop_h', String(crop.h));
        }
        const response = await fetch(`/api/v1/profiles${path}`, {
            method: 'POST',
            body: form,
            ...fetchOptions(),
        });
        return (await handleResponse(response)) as { version: number };
    };

    // tab: a tab id or custom tab slug ('' = the profile tab, '-' = none)
    const getProfile = (username: string, tab = '') =>
        request<Profile>(
            `/users/${encodeURIComponent(username)}${tab ? `?tab=${encodeURIComponent(tab)}` : ''}`,
        );

    const getRatingHistogram = (username: string) =>
        request<ProfileRatingsPayload>(`/users/${encodeURIComponent(username)}/ratings/histogram`);

    // The liked, rated or jizz videos of a user, or a custom list (`lists/<id>`)
    const listProfileScenes = (
        username: string,
        kind: ProfileListKind | `lists/${number}`,
        p: ProfileListParams,
    ) => {
        const qs = new URLSearchParams({ page: String(p.page), limit: String(p.limit) });
        if (p.sort) qs.set('sort', p.sort);
        if (p.rating != null) qs.set('rating', String(p.rating));
        if (p.cardFields) qs.set('card_fields', p.cardFields);
        return request<ProfileListResponse>(`/users/${encodeURIComponent(username)}/${kind}?${qs}`);
    };

    const getMyProfile = () => request<ProfileEdit>('/me');

    const setTabs = (tabs: ProfileTabConfig[]) =>
        request<{ tabs: ProfileTabConfig[] }>('/me/tabs', json('PUT', { tabs }));

    const resetTabs = () =>
        request<{ tabs: ProfileTabConfig[] }>('/me/tabs', json('PUT', { reset: true }));

    const listMyLists = () => request<{ lists: CustomList[] }>('/me/lists');
    const createList = (input: CustomListInput) =>
        request<CustomList>('/me/lists', json('POST', input));
    const updateList = (id: number, input: CustomListInput) =>
        request<CustomList>(`/me/lists/${id}`, json('PUT', input));
    const deleteList = (id: number) => request<null>(`/me/lists/${id}`, { method: 'DELETE' });

    const updateMyProfile = (fields: { display_name?: string; bio?: string }) =>
        request<ProfileEdit>('/me', json('PATCH', fields));

    const setLayout = (blocks: ProfileLayoutBlock[]) =>
        request<{ blocks: ProfileLayoutBlock[] }>('/me/layout', json('PUT', { blocks }));

    const resetLayout = () =>
        request<{ blocks: ProfileLayoutBlock[] }>('/me/layout', json('PUT', { reset: true }));

    const setFavoriteScenes = (sceneIds: number[]) =>
        request<{ scenes: SceneListItem[] }>(
            '/me/showcases/scenes',
            json('PUT', { scene_ids: sceneIds }),
        );

    const setFavoriteMarkers = (markerIds: number[]) =>
        request<{ markers: WidgetMarker[] }>(
            '/me/showcases/markers',
            json('PUT', { marker_ids: markerIds }),
        );

    const listMarkerCandidates = (q: string, page = 1, limit = 24) =>
        request<{ data: WidgetMarker[]; pagination: { total_items: number } }>(
            `/me/marker-candidates?${new URLSearchParams({ q, page: String(page), limit: String(limit) })}`,
        );

    const uploadAvatar = (file: File, crop?: ImageCrop | null) => upload('/me/avatar', file, crop);
    const deleteAvatar = () => request<null>('/me/avatar', { method: 'DELETE' });
    const uploadBanner = (file: File, crop?: ImageCrop | null) => upload('/me/banner', file, crop);
    const setBannerScene = (sceneId: number) =>
        request<null>('/me/banner', json('PUT', { type: 'scene', scene_id: sceneId }));
    const clearBanner = () => request<null>('/me/banner', { method: 'DELETE' });

    return {
        getProfile,
        getRatingHistogram,
        listProfileScenes,
        getMyProfile,
        setTabs,
        resetTabs,
        listMyLists,
        createList,
        updateList,
        deleteList,
        updateMyProfile,
        setLayout,
        resetLayout,
        setFavoriteScenes,
        setFavoriteMarkers,
        listMarkerCandidates,
        uploadAvatar,
        deleteAvatar,
        uploadBanner,
        setBannerScene,
        clearBanner,
    };
};
