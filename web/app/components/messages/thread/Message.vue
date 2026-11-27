<script setup lang="ts">
import type { ThreadItem } from '~/utils/messages';
import type { ReactionLimits } from '~/utils/reactions';
import type ChatReactions from '~/components/chat/Reactions.vue';

type MessageItem = Extract<ThreadItem, { kind: 'message' }>;

// One message of a thread: bubble (mine on the right), share card, time, edited mark, "seen",
// reactions (pills under the bubble, "Add reaction" on hover or long press), and for my
// messages edit (inline) and delete (confirmed).
const props = defineProps<{
    item: MessageItem;
    otherName: string;
    // may write in the conversation: edit, and add reactions (removing mine is always allowed)
    canEdit: boolean;
    reactionLimits: ReactionLimits;
}>();
const emit = defineEmits<{
    edit: [id: number, body: string];
    delete: [id: number];
    joinLobby: [messageId: number];
    react: [id: number, key: string, add: boolean];
}>();

const messagesStore = useMessagesStore();
const authStore = useAuthStore();
const editing = ref(false);
const reactionsEl = ref<InstanceType<typeof ChatReactions> | null>(null);
const m = computed(() => props.item.message);
const time = computed(() =>
    new Date(m.value.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
);
const author = computed(() =>
    props.item.mine ? 'You' : m.value.sender_id === null ? 'Deleted user' : props.otherName,
);
const pills = computed(() =>
    m.value.deleted ? [] : messageReactionPills(m.value.reactions, authStore.user?.id ?? 0),
);
const canAddNew = computed(
    () => props.canEdit && !m.value.deleted && canAddReaction(pills.value, props.reactionLimits),
);

const saveEdit = (body: string) => {
    editing.value = false;
    if (body.trim() !== m.value.body) emit('edit', m.value.id, body);
};

const openPicker = (anchor: HTMLElement) => reactionsEl.value?.openPicker(anchor);
const longPress = useLongPress((el) => {
    if (!m.value.deleted && !editing.value) openPicker(el);
});
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
                v-on="longPress"
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
            <MessagesThreadActions
                v-if="!m.deleted && !editing && (item.mine || canAddNew)"
                :mine="item.mine"
                :can-react="canAddNew"
                :can-edit="canEdit && !!m.body"
                @react="openPicker"
                @edit="editing = true"
                @delete="emit('delete', m.id)"
            />
        </div>
        <ChatReactions
            v-if="!m.deleted"
            ref="reactionsEl"
            class="max-w-[85%]"
            :pills="pills"
            :can-react="canEdit"
            :limits="reactionLimits"
            :align="item.mine ? 'end' : 'start'"
            @toggle="(key, add) => emit('react', m.id, key, add)"
        />
        <span v-if="item.seen" class="text-dim mt-0.5 flex items-center gap-0.5 px-1 text-[9px]">
            <Icon name="heroicons:check" size="9" class="text-emerald" />
            Seen
        </span>
    </div>
</template>
