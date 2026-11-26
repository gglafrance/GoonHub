<script setup lang="ts">
// Toasts of incoming messages, when their thread is not open (not on /messages, not muted,
// and the "message toasts" setting is on: Settings > Privacy).
const messagesStore = useMessagesStore();
</script>

<template>
    <Teleport to="body">
        <div class="pointer-events-none fixed right-4 bottom-4 z-[60] flex w-72 flex-col gap-2">
            <TransitionGroup
                enter-from-class="translate-y-2 opacity-0"
                leave-to-class="opacity-0"
                enter-active-class="transition duration-200"
                leave-active-class="transition duration-200"
            >
                <NuxtLink
                    v-for="t in messagesStore.toasts"
                    :key="t.id"
                    :to="messagesPath(t.username)"
                    class="border-border bg-panel/95 hover:border-lava/30 pointer-events-auto flex
                        items-start gap-2.5 rounded-lg border px-3 py-2.5 shadow-2xl
                        backdrop-blur-md transition-colors"
                    @click="messagesStore.dismissToast(t.id)"
                >
                    <Icon
                        name="heroicons:chat-bubble-oval-left-ellipsis"
                        size="15"
                        class="text-lava mt-0.5 shrink-0"
                    />
                    <div class="min-w-0 flex-1">
                        <div class="truncate text-xs font-semibold text-white">{{ t.title }}</div>
                        <div class="text-dim line-clamp-2 text-[11px]">{{ t.text }}</div>
                    </div>
                    <button
                        class="text-dim shrink-0 hover:text-white"
                        title="Dismiss"
                        @click.prevent.stop="messagesStore.dismissToast(t.id)"
                    >
                        <Icon name="heroicons:x-mark" size="12" />
                    </button>
                </NuxtLink>
            </TransitionGroup>
        </div>
    </Teleport>
</template>
