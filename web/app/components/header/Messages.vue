<script setup lang="ts">
// Header entry of the messages: chat bubble with the unread count (muted conversations left
// out) and a popover of the recent conversations. The live messages of the tab and the toasts
// of incoming messages are started by HeaderLiveSources.
const messagesStore = useMessagesStore();
const route = useRoute();
const showPopup = ref(false);
const buttonRef = ref<HTMLButtonElement | null>(null);

const badge = computed(() => unreadBadge(messagesStore.unread.messages));
const title = computed(() =>
    messagesStore.unread.messages > 0
        ? `Messages (${messagesStore.unread.messages} unread)`
        : 'Messages',
);

watch(
    () => route.fullPath,
    () => {
        showPopup.value = false;
    },
);
</script>

<template>
    <div class="relative">
        <button
            ref="buttonRef"
            class="border-border text-dim hover:border-lava/30 hover:text-lava relative flex h-7 w-7
                items-center justify-center rounded-md border transition-all"
            :class="{ '!border-lava/30 !text-lava': showPopup }"
            :title="title"
            :aria-label="title"
            :aria-expanded="showPopup"
            @click="showPopup = !showPopup"
        >
            <Icon name="heroicons:chat-bubble-oval-left-ellipsis" size="16" />
            <span
                v-if="badge"
                class="bg-lava absolute -top-1.5 -right-1.5 flex h-4 min-w-4 items-center
                    justify-center rounded-full px-1 font-mono text-[9px] leading-none font-semibold
                    text-white shadow-[0_0_8px_rgba(255,77,77,0.6)]"
            >
                {{ badge }}
            </span>
        </button>
        <HeaderMessagesPopup
            :visible="showPopup"
            :anchor-el="buttonRef"
            @close="showPopup = false"
        />
    </div>
</template>
