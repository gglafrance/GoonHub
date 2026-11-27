<script setup lang="ts">
// Popover of the messages: the recent conversations (presence, last message, unread, time).
const props = defineProps<{ visible: boolean; anchorEl: HTMLElement | null }>();
const emit = defineEmits<{ close: [] }>();

const messagesStore = useMessagesStore();
const popupRef = ref<HTMLDivElement | null>(null);
const position = ref({ top: 0, left: 0 });
const recent = computed(() => messagesStore.conversations.slice(0, 12));

function updatePosition() {
    if (!props.anchorEl) return;
    const rect = props.anchorEl.getBoundingClientRect();
    const width = 340;
    const padding = 8;
    let left = rect.left + rect.width / 2 - width / 2;
    left = Math.max(padding, Math.min(left, window.innerWidth - width - padding));
    position.value = { top: rect.bottom + 8, left };
}

function onClickOutside(e: MouseEvent) {
    if (
        popupRef.value &&
        !popupRef.value.contains(e.target as Node) &&
        props.anchorEl &&
        !props.anchorEl.contains(e.target as Node)
    ) {
        emit('close');
    }
}

function teardown() {
    window.removeEventListener('resize', updatePosition);
    document.removeEventListener('mousedown', onClickOutside);
}

watch(
    () => props.visible,
    (visible) => {
        if (visible) {
            updatePosition();
            window.addEventListener('resize', updatePosition);
            document.addEventListener('mousedown', onClickOutside);
        } else {
            teardown();
        }
    },
);

onBeforeUnmount(teardown);
</script>

<template>
    <Teleport to="body">
        <div
            v-if="visible"
            ref="popupRef"
            class="border-border bg-panel/95 fixed z-50 flex max-h-[70vh] w-[340px]
                max-w-[calc(100vw-16px)] flex-col rounded-lg border shadow-2xl backdrop-blur-md"
            :style="{ top: `${position.top}px`, left: `${position.left}px` }"
        >
            <div class="border-border flex items-center justify-between border-b px-4 py-3">
                <div class="flex items-center gap-2">
                    <Icon
                        name="heroicons:chat-bubble-oval-left-ellipsis"
                        size="16"
                        class="text-lava"
                    />
                    <span class="text-xs font-semibold text-white">Messages</span>
                </div>
                <NuxtLink
                    to="/messages"
                    class="text-dim hover:text-lava text-[10px] transition-colors"
                    @click="emit('close')"
                >
                    All messages
                </NuxtLink>
            </div>
            <div class="min-h-0 flex-1 overflow-y-auto p-1.5">
                <div v-if="!messagesStore.loaded" class="flex justify-center py-6">
                    <LoadingSpinner />
                </div>
                <p
                    v-else-if="recent.length === 0"
                    class="text-dim px-4 py-6 text-center text-[11px]"
                >
                    No conversation yet. Write to a friend from their profile or the friends page,
                    or share a video with them.
                </p>
                <MessagesConversationItem
                    v-for="c in recent"
                    :key="c.id"
                    :conversation="c"
                    compact
                    @navigate="emit('close')"
                />
            </div>
        </div>
    </Teleport>
</template>
