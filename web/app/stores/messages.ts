import type {
    ChatConversationEvent,
    ChatMessageEvent,
    ChatReadEvent,
    ChatTypingEvent,
    Conversation,
    MessageSettings,
    UnreadCounts,
} from '~/types/messages';
import type { MessageSseEvent } from '~/utils/messages';

export type MessageEventType = MessageSseEvent | 'reconnect';
export type MessageEventListener = (type: MessageEventType, data: unknown) => void;

export interface MessageToast {
    id: number;
    conversationId: number;
    username: string;
    title: string;
    text: string;
}

/**
 * Messages of the signed-in user (docs/MESSAGES.md): the recent conversations (header popover,
 * /messages list), the unread counts (header badge) and the typing indicators, kept live by
 * the chat:* SSE events. The open thread subscribes to the events of its conversation.
 * Started by the header (HeaderMessages).
 */
export const useMessagesStore = defineStore('messages', () => {
    const conversations = ref<Conversation[]>([]);
    const total = ref(0);
    const unread = ref<UnreadCounts>({ messages: 0, conversations: 0 });
    const loaded = ref(false);
    const settings = ref<MessageSettings | null>(null);
    // the conversation shown in an open thread (its messages are read as they arrive)
    const activeConversationId = ref<number | null>(null);
    // conversation id -> time the typing indicator ends
    const typing = ref<Record<number, number>>({});
    const toasts = ref<MessageToast[]>([]);
    // set by the /messages page: no toast while it is open
    const suppressToasts = ref(false);

    const listeners = new Set<MessageEventListener>();
    let unsubscribers: (() => void)[] = [];
    let unreadTimer: ReturnType<typeof setTimeout> | null = null;
    let toastSeq = 0;

    const myId = () => useAuthStore().user?.id ?? 0;

    async function refresh() {
        if (!useAuthStore().isAuthenticated) return;
        try {
            const api = useApiMessages();
            const [page, counts] = await Promise.all([
                api.listConversations(1, 30),
                api.getUnread(),
            ]);
            conversations.value = sortConversations(page.data ?? []);
            total.value = page.pagination?.total_items ?? conversations.value.length;
            unread.value = counts;
            loaded.value = true;
        } catch {
            // keeps the last state; the next event or reconnect refreshes it
        }
    }

    async function loadSettings(force = false) {
        if (settings.value && !force) return settings.value;
        try {
            settings.value = await useApiMessages().getSettings();
        } catch {
            // defaults apply (toasts on)
        }
        return settings.value;
    }

    function refreshUnreadSoon() {
        if (unreadTimer) clearTimeout(unreadTimer);
        unreadTimer = setTimeout(async () => {
            unreadTimer = null;
            try {
                unread.value = await useApiMessages().getUnread();
            } catch {
                // next event
            }
        }, 400);
    }

    function upsert(conv: Conversation) {
        const i = conversations.value.findIndex((c) => c.id === conv.id);
        const list = [...conversations.value];
        if (i >= 0) list[i] = conv;
        else list.push(conv);
        conversations.value = sortConversations(list);
    }

    async function reloadConversation(id: number) {
        try {
            upsert(await useApiMessages().getConversation(id));
        } catch {
            // left as is
        }
    }

    function byId(id: number) {
        return conversations.value.find((c) => c.id === id);
    }

    function isViewing(id: number) {
        return (
            activeConversationId.value === id &&
            import.meta.client &&
            document.visibilityState === 'visible'
        );
    }

    function pushToast(conv: Conversation | undefined, data: ChatMessageEvent) {
        if (suppressToasts.value || !conv?.other || conv.muted) return;
        if (settings.value && !settings.value.toasts) return;
        const toast: MessageToast = {
            id: ++toastSeq,
            conversationId: conv.id,
            username: conv.other.username,
            title: conversationTitle(conv),
            text: messagePreviewText(
                {
                    id: data.message.id,
                    sender_id: data.message.sender_id,
                    body: data.message.body,
                    attachment_kind: data.message.attachment?.kind ?? null,
                    deleted: data.message.deleted,
                    created_at: data.message.created_at,
                },
                myId(),
            ),
        };
        toasts.value = [...toasts.value.slice(-2), toast];
        setTimeout(() => dismissToast(toast.id), 6000);
    }

    function dismissToast(id: number) {
        toasts.value = toasts.value.filter((t) => t.id !== id);
    }

    async function onMessage(data: ChatMessageEvent) {
        const id = data.conversation_id;
        if (typing.value[id]) typing.value = { ...typing.value, [id]: 0 };
        let conv = byId(id);
        if (!conv) {
            await reloadConversation(id);
            conv = byId(id);
        } else {
            conv = applyNewMessage(conv, data.message, myId(), isViewing(id));
            upsert(conv);
        }
        if (data.message.sender_id !== myId() && !isViewing(id)) pushToast(conv, data);
        refreshUnreadSoon();
    }

    function onUpdated(data: ChatMessageEvent) {
        const conv = byId(data.conversation_id);
        if (!conv?.last_message || conv.last_message.id !== data.message.id) return;
        upsert({
            ...conv,
            last_message: {
                ...conv.last_message,
                body: data.message.body.slice(0, 280),
                deleted: data.message.deleted,
                attachment_kind: data.message.deleted ? null : conv.last_message.attachment_kind,
            },
        });
        refreshUnreadSoon();
    }

    function onRead(data: ChatReadEvent) {
        const conv = byId(data.conversation_id);
        if (conv) {
            if (data.user_id === myId()) {
                const all = !conv.last_message || data.last_read_message_id >= conv.last_message.id;
                if (all) {
                    upsert({
                        ...conv,
                        unread_count: 0,
                        last_read_message_id: data.last_read_message_id,
                    });
                } else {
                    reloadConversation(conv.id);
                }
            } else {
                upsert({ ...conv, other_last_read_message_id: data.last_read_message_id });
            }
        }
        if (data.user_id === myId()) refreshUnreadSoon();
    }

    function onTyping(data: ChatTypingEvent) {
        if (data.user_id === myId()) return;
        typing.value = { ...typing.value, [data.conversation_id]: Date.now() + TYPING_TTL_MS };
    }

    function isTyping(conversationId: number, now = Date.now()): boolean {
        return (typing.value[conversationId] ?? 0) > now;
    }

    function dispatch(type: MessageEventType, data: unknown) {
        for (const fn of listeners) {
            try {
                fn(type, data);
            } catch {
                // a failing listener must not break the others
            }
        }
    }

    /** Listens to the message events (and 'reconnect'); returns the unsubscribe function. */
    function subscribe(fn: MessageEventListener): () => void {
        listeners.add(fn);
        return () => listeners.delete(fn);
    }

    function handle(type: MessageEventType, data: unknown) {
        switch (type) {
            case 'chat:message':
                onMessage(data as ChatMessageEvent);
                break;
            case 'chat:message_updated':
                onUpdated(data as ChatMessageEvent);
                break;
            case 'chat:read':
                onRead(data as ChatReadEvent);
                break;
            case 'chat:typing':
                onTyping(data as ChatTypingEvent);
                break;
            case 'chat:conversation_updated':
                reloadConversation((data as ChatConversationEvent).conversation_id);
                refreshUnreadSoon();
                break;
        }
        dispatch(type, data);
    }

    /** Starts listening to the message events (idempotent). */
    function start() {
        if (unsubscribers.length > 0 || !import.meta.client) return;
        unsubscribers = MESSAGE_SSE_EVENTS.map((type) =>
            onSSEEvent(type, (e) => {
                if (e.data) handle(type, e.data);
            }),
        );
        refresh();
        loadSettings();
    }

    function stop() {
        for (const off of unsubscribers) off();
        unsubscribers = [];
        if (unreadTimer) clearTimeout(unreadTimer);
        unreadTimer = null;
        conversations.value = [];
        unread.value = { messages: 0, conversations: 0 };
        loaded.value = false;
        typing.value = {};
        toasts.value = [];
    }

    /** After an SSE reconnection: reload, and tell the open thread to reload too. */
    function reconnected() {
        refresh();
        dispatch('reconnect', null);
    }

    /** This tab read a conversation up to a message (optimistic, the server confirms). */
    function markedRead(conversationId: number, messageId: number) {
        const conv = byId(conversationId);
        if (conv && (conv.last_read_message_id ?? 0) < messageId) {
            const all = !conv.last_message || messageId >= conv.last_message.id;
            upsert({
                ...conv,
                last_read_message_id: messageId,
                unread_count: all ? 0 : conv.unread_count,
            });
        }
        refreshUnreadSoon();
    }

    return {
        conversations,
        total,
        unread,
        loaded,
        settings,
        activeConversationId,
        typing,
        toasts,
        suppressToasts,
        refresh,
        loadSettings,
        upsert,
        reloadConversation,
        byId,
        isTyping,
        subscribe,
        start,
        stop,
        reconnected,
        markedRead,
        dismissToast,
    };
});
