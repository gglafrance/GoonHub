import type {
    Conversation,
    ConversationPage,
    DirectThread,
    Message,
    MessageHistory,
    MessagePolicy,
    MessageSettings,
    SendMessageBody,
    ShareResult,
    ShareTarget,
    UnreadCounts,
} from '~/types/messages';

/**
 * Messages API (docs/MESSAGES.md): conversations, history, sending, edits, read state, typing,
 * shares and the message settings.
 */
export const useApiMessages = () => {
    const { fetchOptions, getAuthHeaders, handleResponse } = useApiCore();

    const request = async <T>(path: string, init: RequestInit = {}): Promise<T> => {
        const response = await fetch(`/api/v1/messages${path}`, {
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

    const listConversations = (page = 1, limit = 30) =>
        request<ConversationPage>(`/conversations?page=${page}&limit=${limit}`);

    const getConversation = (id: number) => request<Conversation>(`/conversations/${id}`);

    const getUnread = () => request<UnreadCounts>('/unread');

    const getThreadWith = (username: string) =>
        request<DirectThread>(`/with/${encodeURIComponent(username)}`);

    const getHistory = (conversationId: number, before = 0, limit = 0) => {
        const q = new URLSearchParams();
        if (before > 0) q.set('before', String(before));
        if (limit > 0) q.set('limit', String(limit));
        const qs = q.toString();
        return request<MessageHistory>(
            `/conversations/${conversationId}/messages${qs ? `?${qs}` : ''}`,
        );
    };

    const sendMessage = (conversationId: number, body: SendMessageBody) =>
        request<Message>(`/conversations/${conversationId}/messages`, json('POST', body));

    const sendDirect = (userId: number, body: SendMessageBody) =>
        request<{ conversation: Conversation; message: Message }>(
            `/direct/${userId}/messages`,
            json('POST', body),
        );

    const editMessage = (conversationId: number, messageId: number, body: string) =>
        request<Message>(
            `/conversations/${conversationId}/messages/${messageId}`,
            json('PATCH', { body }),
        );

    const deleteMessage = (conversationId: number, messageId: number) =>
        request<null>(`/conversations/${conversationId}/messages/${messageId}`, {
            method: 'DELETE',
        });

    const markRead = (conversationId: number, messageId: number) =>
        request<null>(
            `/conversations/${conversationId}/read`,
            json('POST', { message_id: messageId }),
        );

    const sendTyping = (conversationId: number) =>
        request<null>(`/conversations/${conversationId}/typing`, { method: 'POST' });

    const setMuted = (conversationId: number, muted: boolean) =>
        request<Conversation>(`/conversations/${conversationId}/mute`, json('PUT', { muted }));

    const joinLobby = (conversationId: number, messageId: number) =>
        request<{ lobby_id: string }>(
            `/conversations/${conversationId}/messages/${messageId}/join-lobby`,
            { method: 'POST' },
        );

    const share = (userIds: number[], items: ShareTarget[], body = '') =>
        request<{ results: ShareResult[] }>(
            '/share',
            json('POST', { user_ids: userIds, items, body }),
        );

    const getSettings = () => request<MessageSettings>('/settings');

    const updateSettings = (update: { policy?: MessagePolicy; toasts?: boolean }) =>
        request<MessageSettings>('/settings', json('PUT', update));

    return {
        listConversations,
        getConversation,
        getUnread,
        getThreadWith,
        getHistory,
        sendMessage,
        sendDirect,
        editMessage,
        deleteMessage,
        markRead,
        sendTyping,
        setMuted,
        joinLobby,
        share,
        getSettings,
        updateSettings,
    };
};
