import type {
    PresenceActivityReport,
    PresenceSettings,
    PresenceSnapshot,
    PresenceView,
} from '~/types/presence';

/**
 * Presence API (docs/PRESENCE.md): snapshot of the contacts, reports of this tab (heartbeat,
 * activity), invisible mode.
 */
export const useApiPresence = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
        const response = await fetch(`/api/v1/social/presence${path}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
            ...init,
        });
        return (await handleResponse(response)) as T;
    };

    const getPresence = () => request<PresenceSnapshot>('');

    const getUserPresence = (userId: number) => request<PresenceView>(`/users/${userId}`);

    /** The last input (or playback) of this tab was `idleSeconds` ago. */
    const sendHeartbeat = (active: boolean, idleSeconds = 0) =>
        request<null>('/heartbeat', {
            method: 'POST',
            body: JSON.stringify({ active, idle_seconds: idleSeconds }),
        });

    const reportActivity = (report: PresenceActivityReport) =>
        request<null>('/activity', { method: 'PUT', body: JSON.stringify(report) });

    /** keepalive: also sent while the page unloads. */
    const clearActivity = (tabId: string) =>
        request<null>(`/activity?tab_id=${encodeURIComponent(tabId)}`, {
            method: 'DELETE',
            keepalive: true,
        });

    const getPresenceSettings = () => request<PresenceSettings>('/settings');

    const updatePresenceSettings = (settings: PresenceSettings) =>
        request<PresenceSettings>('/settings', { method: 'PUT', body: JSON.stringify(settings) });

    return {
        getPresence,
        getUserPresence,
        sendHeartbeat,
        reportActivity,
        clearActivity,
        getPresenceSettings,
        updatePresenceSettings,
    };
};
