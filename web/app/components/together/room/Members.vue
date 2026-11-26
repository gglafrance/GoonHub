<script setup lang="ts">
import type { LobbyMember } from '~/types/together';

// Member list: online first, then by name. Kick and ban ask for a confirmation.
const room = useInjectedTogetherRoom();
const { socket, myId, isHost, canModerate, onlineCount } = room;

const sorted = computed(() =>
    [...socket.members.value].sort((a, b) => {
        if (a.host !== b.host) return a.host ? -1 : 1;
        if (a.online !== b.online) return a.online ? -1 : 1;
        return a.username.localeCompare(b.username);
    }),
);

const confirm = ref<{ kind: 'kick' | 'ban'; member: LobbyMember } | null>(null);

const onAction = (kind: 'grant' | 'revoke' | 'host' | 'kick' | 'ban', member: LobbyMember) => {
    switch (kind) {
        case 'grant':
            socket.send({ t: 'grant_control', user_id: member.user_id });
            break;
        case 'revoke':
            socket.send({ t: 'revoke_control', user_id: member.user_id });
            break;
        case 'host':
            socket.send({ t: 'transfer_host', user_id: member.user_id });
            break;
        default:
            confirm.value = { kind, member };
    }
};

const doConfirm = () => {
    const c = confirm.value;
    if (!c) return;
    socket.send({ t: 'kick', user_id: c.member.user_id, ban: c.kind === 'ban' });
    confirm.value = null;
};
</script>

<template>
    <div class="border-border bg-panel/40 rounded-xl border">
        <div class="border-border flex items-center gap-2 border-b px-3 py-2">
            <Icon name="heroicons:users" size="13" class="text-lava" />
            <span class="text-xs font-semibold text-white">Members</span>
            <span class="text-dim ml-auto font-mono text-[10px]"> {{ onlineCount }} online </span>
        </div>
        <div class="max-h-56 overflow-y-auto p-1">
            <TogetherRoomMemberRow
                v-for="m in sorted"
                :key="m.user_id"
                :member="m"
                :is-me="m.user_id === myId"
                :as-host="isHost"
                :as-moderator="canModerate"
                :control-open="!!socket.lobby.value?.control_open"
                @action="onAction"
            />
        </div>

        <UiConfirmModal
            :visible="!!confirm"
            :title="confirm?.kind === 'ban' ? 'Ban from this lobby?' : 'Kick from this lobby?'"
            :confirm-label="confirm?.kind === 'ban' ? 'Ban' : 'Kick'"
            @close="confirm = null"
            @confirm="doConfirm"
        >
            <p v-if="confirm?.kind === 'ban'">
                {{ confirm.member.username }} is removed and cannot join this lobby again.
            </p>
            <p v-else-if="confirm">
                {{ confirm.member.username }} is removed from the lobby (they can join again with
                the invite).
            </p>
        </UiConfirmModal>
    </div>
</template>
