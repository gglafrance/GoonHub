<script setup lang="ts">
import type { ThreadItem } from '~/utils/messages';

type MessageItem = Extract<ThreadItem, { kind: 'message' }>;

// One message of a thread: bubble (mine on the right), share card, time, edited mark, "seen",
// and for my messages edit (inline) and delete (confirmed).
const props = defineProps<{ item: MessageItem; otherName: string; canEdit: boolean }>();
const emit = defineEmits<{
    edit: [id: number, body: string];
    delete: [id: number];
    joinLobby: [messageId: number];
}>();

const messagesStore = useMessagesStore();
const editing = ref(false);
const confirmDelete = ref(false);
const m = computed(() => props.item.message);
const time = computed(() =>
    new Date(m.value.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
);
const author = computed(() =>
    props.item.mine ? 'You' : m.value.sender_id === null ? 'Deleted user' : props.otherName,
);

const doDelete = () => {
    confirmDelete.value = false;
    emit('delete', m.value.id);
};

const saveEdit = (body: string) => {
    editing.value = false;
    if (body.trim() !== m.value.body) emit('edit', m.value.id, body);
};
</script>

<template>
    <div
        class="group flex flex-col"
        :class="[item.mine ? 'items-end' : 'items-start', item.groupStart ? 'mt-2.5' : 'mt-0.5']"
    >
        <div
            v-if="item.groupStart"
            class="text-dim mb-0.5 flex items-center gap-1.5 px-1 text-[10px]"
        >
            <span class="font-semibold" :class="item.mine ? 'text-lava/80' : 'text-white/70'">
                {{ author }}
            </span>
            <span class="font-mono">{{ time }}</span>
        </div>
        <div class="flex max-w-[85%] items-center gap-1" :class="{ 'flex-row-reverse': item.mine }">
            <div
                class="min-w-0 rounded-xl border px-3 py-1.5 text-xs leading-relaxed"
                :class="[
                    m.deleted
                        ? 'border-border text-dim bg-transparent italic'
                        : item.mine
                          ? 'border-lava/25 bg-lava/10 text-white/90'
                          : 'border-border bg-surface/70 text-white/85',
                ]"
                :title="item.groupStart ? undefined : time"
            >
                <template v-if="m.deleted">Message deleted</template>
                <template v-else>
                    <div v-if="editing" class="w-64 max-w-full">
                        <TogetherChatInput
                            :max-length="messagesStore.settings?.max_length ?? 2000"
                            :initial="m.body"
                            placeholder="Edit the message (Escape to cancel)"
                            autofocus
                            @send="saveEdit"
                            @cancel="editing = false"
                        />
                    </div>
                    <ChatRichText v-else-if="m.body" :text="m.body" />
                    <MessagesShareCard
                        v-if="m.attachment"
                        :attachment="m.attachment"
                        :class="{ 'mt-1.5': !!m.body }"
                        @join="emit('joinLobby', m.id)"
                    />
                    <span v-if="m.edited_at && !editing" class="text-dim ml-1 text-[9px]">
                        (edited)
                    </span>
                </template>
            </div>
            <div
                v-if="item.mine && !m.deleted && !editing"
                class="flex shrink-0 items-center opacity-0 transition-opacity
                    group-hover:opacity-100 focus-within:opacity-100"
            >
                <button
                    v-if="canEdit && m.body"
                    class="text-dim flex h-6 w-6 items-center justify-center rounded
                        hover:text-white"
                    title="Edit"
                    @click="editing = true"
                >
                    <Icon name="heroicons:pencil" size="11" />
                </button>
                <button
                    v-if="!confirmDelete"
                    class="text-dim hover:text-lava flex h-6 w-6 items-center justify-center
                        rounded"
                    title="Delete"
                    @click="confirmDelete = true"
                >
                    <Icon name="heroicons:trash" size="11" />
                </button>
                <template v-else>
                    <button
                        class="bg-lava/15 text-lava rounded px-1.5 py-0.5 text-[10px] font-medium"
                        @click="doDelete"
                    >
                        Delete
                    </button>
                    <button
                        class="text-dim px-1 text-[10px] hover:text-white"
                        @click="confirmDelete = false"
                    >
                        Keep
                    </button>
                </template>
            </div>
        </div>
        <span v-if="item.seen" class="text-dim mt-0.5 flex items-center gap-0.5 px-1 text-[9px]">
            <Icon name="heroicons:check" size="9" class="text-emerald" />
            Seen
        </span>
    </div>
</template>
