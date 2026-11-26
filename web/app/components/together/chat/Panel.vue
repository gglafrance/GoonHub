<script setup lang="ts">
import type { ChatMessage } from '~/types/together';
import type { ReactionLimits } from '~/utils/reactions';

// Room chat: recent messages (sent by the server on connect), new ones, deletion for the
// host, moderators and the author, reactions on the messages (chat_react / chat_unreact).
const room = useInjectedTogetherRoom();
const { socket, isMine, isHost, canModerate } = room;

const list = ref<HTMLElement | null>(null);
// Auto-scroll only while the user is at the bottom
const stick = ref(true);

const onScroll = () => {
    const el = list.value;
    if (!el) return;
    stick.value = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
};

watch(
    // a new array on every change (new message, reactions): follow while at the bottom
    () => socket.chat.value,
    async () => {
        if (!stick.value) return;
        await nextTick();
        if (list.value) list.value.scrollTop = list.value.scrollHeight;
    },
    { immediate: true },
);

const canDelete = (m: ChatMessage) =>
    isHost.value || canModerate.value || isMine(m.user_id, m.anon_number);

const send = (text: string) => socket.send({ t: 'chat', text });
const remove = (id: number) => socket.send({ t: 'chat_delete', id });

const connected = computed(() => socket.status.value === 'open');
const reactionLimits = computed<ReactionLimits>(() => ({
    maxKinds: socket.config.value.chat_max_reaction_kinds,
    maxPerUser: socket.config.value.chat_max_reactions_per_user,
}));
const pills = (m: ChatMessage) => lobbyReactionPills(m.reactions, isMine);
const react = (id: number, emote: string, add: boolean) =>
    socket.send({ t: add ? 'chat_react' : 'chat_unreact', id, emote });
</script>

<template>
    <div class="border-border bg-panel/40 flex min-h-0 flex-col rounded-xl border">
        <div class="border-border flex items-center gap-2 border-b px-3 py-2">
            <Icon name="heroicons:chat-bubble-left-right" size="13" class="text-lava" />
            <span class="text-xs font-semibold text-white">Chat</span>
        </div>
        <div
            ref="list"
            class="min-h-0 flex-1 space-y-0.5 overflow-y-auto px-1 py-2"
            @scroll="onScroll"
        >
            <p v-if="socket.chat.value.length === 0" class="text-dim py-6 text-center text-[11px]">
                No message yet. Say hi!
            </p>
            <TogetherChatMessage
                v-for="m in socket.chat.value"
                :key="m.id"
                :message="m"
                :mine="!m.system && isMine(m.user_id, m.anon_number)"
                :can-delete="!m.system && canDelete(m)"
                :pills="pills(m)"
                :can-react="connected"
                :reaction-limits="reactionLimits"
                @delete="remove"
                @react="react"
            />
        </div>
        <div class="border-border border-t p-2">
            <TogetherChatInput
                :max-length="socket.config.value.chat_max_length"
                :disabled="socket.status.value !== 'open'"
                @send="send"
            />
        </div>
    </div>
</template>
