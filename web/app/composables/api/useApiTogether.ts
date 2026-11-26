import type { CreateLobbyInput, Emote, LobbySummary, QueueItemInput } from '~/types/together';

/**
 * Goon2gether REST API: lobbies, joining, queueing from elsewhere, server emotes.
 * Contract: docs/GOON2GETHER.md.
 */
export const useApiTogether = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const base = '/api/v1/together';

    const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
        const response = await fetch(`${base}${path}`, {
            headers: getAuthHeaders(),
            ...fetchOptions(),
            ...init,
        });
        return (await handleResponse(response)) as T;
    };

    const json = (method: string, body?: unknown): RequestInit => ({
        method,
        body: body === undefined ? undefined : JSON.stringify(body),
    });

    const listLobbies = (all = false) =>
        request<{ lobbies: LobbySummary[] }>(`/lobbies${all ? '?all=true' : ''}`);

    const getLobby = (id: string) => request<LobbySummary>(`/lobbies/${encodeURIComponent(id)}`);

    const createLobby = (input: CreateLobbyInput) =>
        request<LobbySummary>('/lobbies', json('POST', input));

    /** Joins a lobby; `anonymous` (public lobbies): the others see "Anonymous #N". */
    const joinLobby = (id: string, anonymous = false) =>
        request<LobbySummary>(
            `/lobbies/${encodeURIComponent(id)}/join`,
            json('POST', anonymous ? { anonymous: true } : undefined),
        );

    const joinByCode = (code: string) =>
        request<LobbySummary>('/join', json('POST', { code: code.trim() }));

    const leaveLobby = (id: string) =>
        request<null>(`/lobbies/${encodeURIComponent(id)}/leave`, json('POST'));

    const closeLobby = (id: string) =>
        request<null>(`/lobbies/${encodeURIComponent(id)}`, json('DELETE'));

    const rotateInviteCode = (id: string) =>
        request<{ invite_code: string }>(
            `/lobbies/${encodeURIComponent(id)}/invite-code`,
            json('POST'),
        );

    const addToQueue = (id: string, items: QueueItemInput[]) =>
        request<{ queued: number; suggested: number }>(
            `/lobbies/${encodeURIComponent(id)}/queue`,
            json('POST', { items }),
        );

    const listEmotes = () => request<{ emotes: Emote[] }>('/emotes');

    const uploadEmote = async (name: string, file: File): Promise<Emote> => {
        const form = new FormData();
        form.append('name', name);
        form.append('file', file);
        // No JSON content type: the browser sets the multipart boundary
        const response = await fetch(`${base}/emotes`, {
            method: 'POST',
            body: form,
            ...fetchOptions(),
        });
        return (await handleResponse(response)) as Emote;
    };

    const renameEmote = (id: number, name: string) =>
        request<Emote>(`/emotes/${id}`, json('PUT', { name }));

    const deleteEmote = (id: number) => request<null>(`/emotes/${id}`, json('DELETE'));

    return {
        listLobbies,
        getLobby,
        createLobby,
        joinLobby,
        joinByCode,
        leaveLobby,
        closeLobby,
        rotateInviteCode,
        addToQueue,
        listEmotes,
        uploadEmote,
        renameEmote,
        deleteEmote,
    };
};
