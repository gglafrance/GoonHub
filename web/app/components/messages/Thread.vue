<script setup lang="ts">
import type {
    ChatReactionEvent,
    DirectThread,
    Message,
    MessageHistory,
    SendMessageBody,
} from '~/types/messages';
import type { MessageEventType } from '~/stores/messages';

// The thread with a user: header, history (older pages when scrolled up, live messages),
// composer. Reads the messages as they arrive while visible; reloads after a reconnection.
// Reactions: useMessageReactions (toggle, chat:reaction events).
const props = defineProps<{ username: string }>();

const api = useApiMessages();
const authStore = useAuthStore();
const messagesStore = useMessagesStore();
const myId = computed(() => authStore.user?.id ?? 0);

const thread = ref<DirectThread | null>(null);
const messages = ref<Message[]>([]);
const hasMore = ref(false);
const loading = ref(true);
const loadingOlder = ref(false);
const error = ref('');
const sendError = ref('');
const sending = ref(false);

const conversation = computed(() => {
    const id = thread.value?.conversation?.id;
    return (id ? messagesStore.byId(id) : null) ?? thread.value?.conversation ?? null;
});
const canSend = computed(() => conversation.value?.can_send ?? thread.value?.can_send ?? false);
const readOnlyReason = computed(
    () => conversation.value?.read_only_reason ?? thread.value?.read_only_reason,
);
const items = computed(() =>
    buildThreadItems(
        messages.value,
        myId.value,
        conversation.value?.other_last_read_message_id ?? null,
    ),
);
const name = computed(() =>
    thread.value ? thread.value.user.display_name || thread.value.user.username : props.username,
);
const reactions = useMessageReactions(
    messages,
    () => conversation.value?.id ?? null,
    (message) => (sendError.value = message),
);

function applyHistory(h: MessageHistory, older: boolean) {
    messages.value = mergeMessages(messages.value, h.messages);
    if (older || messages.value.length === h.messages.length) hasMore.value = h.has_more;
}

async function load() {
    loading.value = true;
    error.value = '';
    try {
        thread.value = await api.getThreadWith(props.username);
        messages.value = [];
        hasMore.value = false;
        const conv = thread.value.conversation;
        if (conv) {
            messagesStore.upsert(conv);
            applyHistory(await api.getHistory(conv.id), false);
            markRead();
        }
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load the conversation';
    } finally {
        loading.value = false;
    }
}

async function loadOlder() {
    const conv = conversation.value;
    const first = messages.value[0];
    if (!conv || !first || !hasMore.value || loadingOlder.value) return;
    loadingOlder.value = true;
    try {
        applyHistory(await api.getHistory(conv.id, first.id), true);
    } catch {
        // retried on the next scroll
    } finally {
        loadingOlder.value = false;
    }
}

let readInflight = 0;
function markRead() {
    const conv = conversation.value;
    if (!conv || document.visibilityState !== 'visible') return;
    let last = 0;
    for (const m of messages.value) if (m.sender_id !== myId.value && m.id > last) last = m.id;
    if (!last || (conv.last_read_message_id ?? 0) >= last || readInflight >= last) return;
    readInflight = last;
    messagesStore.markedRead(conv.id, last);
    api.markRead(conv.id, last).catch(() => {
        readInflight = 0;
    });
}

async function send(body: SendMessageBody) {
    if (!thread.value || sending.value) return;
    sending.value = true;
    sendError.value = '';
    try {
        const conv = conversation.value;
        if (conv) {
            const msg = await api.sendMessage(conv.id, body);
            messages.value = mergeMessages(messages.value, [msg]);
        } else {
            const res = await api.sendDirect(thread.value.user.id, body);
            thread.value = { ...thread.value, conversation: res.conversation };
            messagesStore.upsert(res.conversation);
            messages.value = mergeMessages(messages.value, [res.message]);
        }
    } catch (e: unknown) {
        sendError.value = e instanceof Error ? e.message : 'Failed to send';
        const status = (e as { status?: number }).status;
        if (status === 403) load();
    } finally {
        sending.value = false;
    }
}

async function edit(id: number, body: string) {
    const conv = conversation.value;
    if (!conv) return;
    try {
        messages.value = mergeMessages(messages.value, [await api.editMessage(conv.id, id, body)]);
    } catch (e: unknown) {
        sendError.value = e instanceof Error ? e.message : 'Failed to edit';
    }
}

async function remove(id: number) {
    const conv = conversation.value;
    if (!conv) return;
    try {
        await api.deleteMessage(conv.id, id);
        messages.value = messages.value.map((m) =>
            m.id === id ? { ...m, deleted: true, body: '', attachment: null, reactions: [] } : m,
        );
    } catch (e: unknown) {
        sendError.value = e instanceof Error ? e.message : 'Failed to delete';
    }
}

async function joinLobby(messageId: number) {
    const conv = conversation.value;
    if (!conv) return;
    try {
        const res = await api.joinLobby(conv.id, messageId);
        await navigateTo(`/together/${res.lobby_id}`);
    } catch (e: unknown) {
        sendError.value = e instanceof Error ? e.message : 'Failed to join the lobby';
    }
}

let lastTyping = 0;
function typing() {
    const conv = conversation.value;
    const interval = messagesStore.settings?.typing_interval_ms ?? 3000;
    if (!conv || !canSend.value || Date.now() - lastTyping < interval) return;
    lastTyping = Date.now();
    api.sendTyping(conv.id).catch(() => {});
}

function onEvent(type: MessageEventType, data: unknown) {
    if (type === 'reconnect') {
        load();
        return;
    }
    if (type === 'chat:reaction') {
        reactions.apply(data as ChatReactionEvent);
        return;
    }
    const conv = conversation.value;
    const d = data as { conversation_id?: number; message?: Message };
    if (!conv || d.conversation_id !== conv.id) return;
    if ((type === 'chat:message' || type === 'chat:message_updated') && d.message) {
        messages.value = mergeMessages(messages.value, [d.message]);
        if (type === 'chat:message') markRead();
    }
}

const onVisibility = () => markRead();
let unsubscribe: (() => void) | null = null;

watch(
    () => conversation.value?.id ?? null,
    (id) => {
        messagesStore.activeConversationId = id;
    },
    { immediate: true },
);

onMounted(() => {
    unsubscribe = messagesStore.subscribe(onEvent);
    document.addEventListener('visibilitychange', onVisibility);
    messagesStore.loadSettings();
    load();
});
onBeforeUnmount(() => {
    unsubscribe?.();
    document.removeEventListener('visibilitychange', onVisibility);
    if (messagesStore.activeConversationId === conversation.value?.id) {
        messagesStore.activeConversationId = null;
    }
});
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col">
        <MessagesThreadHeader
            :user="thread?.user ?? null"
            :username="username"
            :conversation="conversation"
        />
        <div v-if="loading" class="flex flex-1 items-center justify-center">
            <LoadingSpinner />
        </div>
        <div v-else-if="error" class="text-lava flex flex-1 items-center justify-center text-xs">
            {{ error }}
        </div>
        <template v-else>
            <MessagesThreadHistory
                :items="items"
                :has-more="hasMore"
                :loading-older="loadingOlder"
                :conversation-id="conversation?.id ?? null"
                :other-name="name"
                :can-edit="canSend"
                :reaction-limits="reactions.limits.value"
                @load-older="loadOlder"
                @react="reactions.react"
                @edit="edit"
                @delete="remove"
                @join-lobby="joinLobby"
            />
            <MessagesThreadComposer
                :can-send="canSend"
                :read-only-text="readOnlyText(readOnlyReason, name)"
                :max-length="messagesStore.settings?.max_length ?? 2000"
                :sending="sending"
                :error="sendError"
                :other-name="name"
                @send="send"
                @typing="typing"
                @dismiss-error="sendError = ''"
            />
        </template>
    </div>
</template>
