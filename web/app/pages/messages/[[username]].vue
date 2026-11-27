<script setup lang="ts">
// Messages: the conversation list and the thread with a user (/messages/<username>). On small
// screens, the list or the thread (with a back button).
definePageMeta({
    middleware: ['auth'],
});

const route = useRoute();
const messagesStore = useMessagesStore();
const username = computed(() => {
    const p = route.params.username;
    return typeof p === 'string' && p ? p : null;
});

useHead({ title: () => (username.value ? `Messages - ${username.value}` : 'Messages') });

onMounted(() => {
    messagesStore.suppressToasts = true;
});
onBeforeUnmount(() => {
    messagesStore.suppressToasts = false;
});
</script>

<template>
    <div class="mx-auto max-w-415 px-4 py-4 sm:px-5">
        <div
            class="border-border bg-panel/40 flex
                h-[calc(100dvh-var(--app-header-h)-var(--app-bottom-nav-h)-2rem)] min-h-[420px]
                overflow-hidden rounded-xl border backdrop-blur-sm md:h-[calc(100vh-9rem)]"
        >
            <aside
                class="border-border w-full shrink-0 flex-col border-r md:flex md:w-80"
                :class="username ? 'hidden' : 'flex'"
            >
                <MessagesConversationList :active-username="username" />
            </aside>
            <section class="min-w-0 flex-1 flex-col md:flex" :class="username ? 'flex' : 'hidden'">
                <MessagesThread v-if="username" :key="username" :username="username" />
                <div
                    v-else
                    class="text-dim flex flex-1 flex-col items-center justify-center gap-2 text-xs"
                >
                    <Icon name="heroicons:chat-bubble-left-right" size="28" class="text-lava/60" />
                    Pick a conversation, or start one from a friend.
                </div>
            </section>
        </div>
    </div>
</template>
