<script setup lang="ts">
// Conversation list of /messages: search, new message, the conversations (live from the store),
// more pages on demand.
const props = defineProps<{ activeUsername: string | null }>();

const messagesStore = useMessagesStore();
const api = useApiMessages();
const query = ref('');
const page = ref(1);
const loadingMore = ref(false);
const showNew = ref(false);

const filtered = computed(() => {
    const q = query.value.trim().toLowerCase();
    if (!q) return messagesStore.conversations;
    return messagesStore.conversations.filter((c) => {
        const o = c.other;
        return (
            !!o &&
            (o.username.toLowerCase().includes(q) ||
                (o.display_name ?? '').toLowerCase().includes(q))
        );
    });
});
const hasMore = computed(() => messagesStore.conversations.length < messagesStore.total);

async function loadMore() {
    if (loadingMore.value) return;
    loadingMore.value = true;
    try {
        const res = await api.listConversations(page.value + 1, 30);
        page.value++;
        for (const c of res.data ?? []) messagesStore.upsert(c);
    } catch {
        // the button stays
    } finally {
        loadingMore.value = false;
    }
}

const pick = (username: string) => {
    showNew.value = false;
    navigateTo(messagesPath(username));
};

onMounted(() => {
    if (!messagesStore.loaded) messagesStore.refresh();
});
</script>

<template>
    <div class="flex min-h-0 flex-1 flex-col">
        <div class="border-border flex items-center gap-2 border-b px-3 py-2.5">
            <Icon name="heroicons:chat-bubble-left-right" size="15" class="text-lava" />
            <h1 class="flex-1 text-sm font-semibold text-white">Messages</h1>
            <button
                class="border-border text-dim hover:border-lava/30 hover:text-lava flex h-7
                    items-center gap-1 rounded-md border px-2 text-[11px] font-medium
                    transition-all"
                title="New message"
                @click="showNew = !showNew"
            >
                <Icon name="heroicons:pencil-square" size="13" />
                New
            </button>
        </div>
        <div v-if="showNew" class="border-border border-b p-2">
            <ShareRecipientPicker single @pick="(u) => pick(u.username)" />
        </div>
        <div class="px-3 py-2">
            <input
                v-model="query"
                type="search"
                placeholder="Search conversations"
                class="border-border bg-surface focus:border-lava/40 w-full rounded-md border px-2.5
                    py-1.5 text-xs text-white outline-none placeholder:text-white/30"
            />
        </div>
        <div class="min-h-0 flex-1 overflow-y-auto px-1.5 pb-2">
            <div v-if="!messagesStore.loaded" class="flex justify-center py-8">
                <LoadingSpinner />
            </div>
            <p v-else-if="filtered.length === 0" class="text-dim px-3 py-8 text-center text-[11px]">
                {{
                    query
                        ? 'No conversation matches.'
                        : 'No conversation yet. Write to a friend with "New".'
                }}
            </p>
            <MessagesConversationItem
                v-for="c in filtered"
                :key="c.id"
                :conversation="c"
                :active="!!c.other && c.other.username === props.activeUsername"
            />
            <button
                v-if="hasMore && !query"
                class="text-dim hover:text-lava mt-1 w-full py-2 text-[11px] transition-colors
                    disabled:opacity-40"
                :disabled="loadingMore"
                @click="loadMore"
            >
                {{ loadingMore ? 'Loading...' : 'Load more' }}
            </button>
        </div>
    </div>
</template>
