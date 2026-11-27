<script setup lang="ts">
import type { ThreadItem } from '~/utils/messages';
import type { ReactionLimits } from '~/utils/reactions';

// Scrolling history of a thread: day separators, grouped messages, older pages loaded when
// scrolled near the top (the scroll position is kept), stuck to the bottom for new messages
// while the user is there, and the typing indicator.
const props = defineProps<{
    items: ThreadItem[];
    hasMore: boolean;
    loadingOlder: boolean;
    conversationId: number | null;
    otherName: string;
    // may write: edit, and add reactions
    canEdit: boolean;
    reactionLimits: ReactionLimits;
}>();
const emit = defineEmits<{
    loadOlder: [];
    edit: [id: number, body: string];
    delete: [id: number];
    joinLobby: [messageId: number];
    react: [id: number, key: string, add: boolean];
}>();

const messagesStore = useMessagesStore();
const list = ref<HTMLElement | null>(null);
const stick = ref(true);
const now = ref(Date.now());
let clock: ReturnType<typeof setInterval> | null = null;
// scroll height before older messages are prepended
let anchorHeight = 0;

const typing = computed(
    () => props.conversationId != null && messagesStore.isTyping(props.conversationId, now.value),
);
const firstId = computed(() => {
    const first = props.items.find((i) => i.kind === 'message');
    return first && first.kind === 'message' ? first.message.id : 0;
});

const onScroll = () => {
    const el = list.value;
    if (!el) return;
    stick.value = el.scrollHeight - el.scrollTop - el.clientHeight < 60;
    if (el.scrollTop < 80 && props.hasMore && !props.loadingOlder) {
        anchorHeight = el.scrollHeight;
        emit('loadOlder');
    }
};

const toBottom = () => {
    if (list.value) list.value.scrollTop = list.value.scrollHeight;
};

// Older messages prepended: keep what the user was looking at in place
watch(firstId, async (id, old) => {
    if (!old || id >= old || !anchorHeight) return;
    await nextTick();
    const el = list.value;
    if (el) el.scrollTop += el.scrollHeight - anchorHeight;
    anchorHeight = 0;
});

// New messages (or the first page): follow them while at the bottom
watch(
    () => [props.items.length, typing.value],
    async () => {
        if (!stick.value) return;
        await nextTick();
        toBottom();
    },
    { immediate: true },
);

onMounted(() => {
    clock = setInterval(() => (now.value = Date.now()), 1000);
    nextTick(toBottom);
});
onBeforeUnmount(() => {
    if (clock) clearInterval(clock);
});
</script>

<template>
    <div ref="list" class="min-h-0 flex-1 overflow-y-auto px-3 py-3" @scroll="onScroll">
        <div v-if="loadingOlder" class="flex justify-center py-2"><LoadingSpinner /></div>
        <p
            v-else-if="!hasMore && items.length > 0"
            class="text-dim py-2 text-center text-[10px] tracking-wider uppercase"
        >
            Start of the conversation
        </p>
        <p v-if="items.length === 0" class="text-dim py-10 text-center text-xs">
            No message yet. Say hi to {{ otherName }}!
        </p>
        <template v-for="item in items" :key="item.key">
            <div v-if="item.kind === 'day'" class="my-3 flex items-center gap-3">
                <div class="bg-border h-px flex-1" />
                <span class="text-dim text-[10px] font-medium tracking-wider uppercase">
                    <template v-if="relativeDay(item.day, new Date(now)) === 'today'"
                        >Today</template
                    >
                    <template v-else-if="relativeDay(item.day, new Date(now)) === 'yesterday'">
                        Yesterday
                    </template>
                    <UiFormattedDate v-else :datetime="item.date" />
                </span>
                <div class="bg-border h-px flex-1" />
            </div>
            <MessagesThreadMessage
                v-else
                :item="item"
                :other-name="otherName"
                :can-edit="canEdit"
                :reaction-limits="reactionLimits"
                @edit="(id, body) => emit('edit', id, body)"
                @react="(id, key, add) => emit('react', id, key, add)"
                @delete="emit('delete', $event)"
                @join-lobby="emit('joinLobby', $event)"
            />
        </template>
        <div v-if="typing" class="text-dim mt-2 flex items-center gap-1.5 px-1 text-[11px] italic">
            <span class="flex gap-0.5">
                <span class="bg-lava/70 h-1 w-1 animate-bounce rounded-full" />
                <span
                    class="bg-lava/70 h-1 w-1 animate-bounce rounded-full [animation-delay:150ms]"
                />
                <span
                    class="bg-lava/70 h-1 w-1 animate-bounce rounded-full [animation-delay:300ms]"
                />
            </span>
            {{ otherName }} is typing
        </div>
    </div>
</template>
