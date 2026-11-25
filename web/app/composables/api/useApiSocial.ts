import type {
    FriendRequestSent,
    PrivacySettings,
    PrivacyUpdate,
    RelationshipChange,
    SocialListKind,
    SocialProfile,
    SocialSummary,
    SocialUserPage,
} from '~/types/social';

/**
 * Social graph API (docs/SOCIAL.md): directory, follows, friend requests, friends, privacy.
 */
export const useApiSocial = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
        const response = await fetch(`/api/v1/social${path}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
            ...init,
        });
        return (await handleResponse(response)) as T;
    };

    const pageQuery = (page: number, limit: number, extra: Record<string, string> = {}) =>
        new URLSearchParams({ page: String(page), limit: String(limit), ...extra }).toString();

    const getSummary = () => request<SocialSummary>('/me');

    const searchUsers = (q: string, page = 1, limit = 30) =>
        request<SocialUserPage>(`/users?${pageQuery(page, limit, { q })}`);

    const getSocialProfile = (username: string) =>
        request<SocialProfile>(`/users/${encodeURIComponent(username)}`);

    const listRelations = (username: string, kind: SocialListKind, page = 1, limit = 30) =>
        request<SocialUserPage>(
            `/users/${encodeURIComponent(username)}/${kind}?${pageQuery(page, limit)}`,
        );

    const listFriendRequests = (direction: 'incoming' | 'outgoing', page = 1, limit = 30) =>
        request<SocialUserPage>(`/friend-requests?${pageQuery(page, limit, { direction })}`);

    const follow = (userId: number) =>
        request<RelationshipChange>(`/follows/${userId}`, { method: 'POST' });

    const unfollow = (userId: number) =>
        request<RelationshipChange>(`/follows/${userId}`, { method: 'DELETE' });

    const sendFriendRequest = (userId: number) =>
        request<FriendRequestSent>('/friend-requests', {
            method: 'POST',
            body: JSON.stringify({ user_id: userId }),
        });

    const acceptFriendRequest = (userId: number) =>
        request<RelationshipChange>(`/friend-requests/${userId}/accept`, { method: 'POST' });

    const declineFriendRequest = (userId: number) =>
        request<RelationshipChange>(`/friend-requests/${userId}/decline`, { method: 'POST' });

    const cancelFriendRequest = (userId: number) =>
        request<RelationshipChange>(`/friend-requests/${userId}`, { method: 'DELETE' });

    const removeFriend = (userId: number) =>
        request<RelationshipChange>(`/friends/${userId}`, { method: 'DELETE' });

    const getPrivacySettings = () => request<PrivacySettings>('/privacy');

    const updatePrivacySettings = (update: PrivacyUpdate) =>
        request<PrivacySettings>('/privacy', { method: 'PUT', body: JSON.stringify(update) });

    return {
        getSummary,
        searchUsers,
        getSocialProfile,
        listRelations,
        listFriendRequests,
        follow,
        unfollow,
        sendFriendRequest,
        acceptFriendRequest,
        declineFriendRequest,
        cancelFriendRequest,
        removeFriend,
        getPrivacySettings,
        updatePrivacySettings,
    };
};
