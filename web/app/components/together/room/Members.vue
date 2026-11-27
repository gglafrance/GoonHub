<script setup lang="ts">
import type { LobbyMember } from '~/types/together';

// Member list: online first, then by name. Kick and ban ask for a confirmation. The list can be
// folded to its header (remembered on this device) and scrolls inside a short box, so that it
// never pushes the chat out of the room's side column.
const room = useInjectedTogetherRoom();
const { socket, isMine, isHost, canModerate, onlineCount } = room;

const COLLAPSED_KEY = 'goonhub:together:members-collapsed';
const collapsed = ref(false);
onMounted(() => {
    collapsed.value = localStorage.getItem(COLLAPSED_KEY) === 'true';
});
const toggle = () => {
    collapsed.value = !collapsed.value;
    localStorage.setItem(COLLAPSED_KEY, String(collapsed.value));
};

const sorted = computed(() =>
    [...socket.members.value].sort((a, b) => {
        if (a.host !== b.host) return a.host ? -1 : 1;
        if (a.online !== b.online) return a.online ? -1 : 1;
        return a.username.localeCompare(b.username);
    }),
);

const confirm = ref<{ kind: 'kick' | 'ban'; member: LobbyMember } | null>(null);

const onAction = (kind: 'grant' | 'revoke' | 'host' | 'kick' | 'ban', member: LobbyMember) => {
    const target = memberTarget(member);
    if (!target) return;
    switch (kind) {
        case 'grant':
            socket.send({ t: 'grant_control', ...target });
            break;
        case 'revoke':
            socket.send({ t: 'revoke_control', ...target });
            break;
        case 'host':
            socket.send({ t: 'transfer_host', ...target });
            break;
        default:
            confirm.value = { kind, member };
    }
};

const doConfirm = () => {
    const c = confirm.value;
    const target = c ? memberTarget(c.member) : null;
    if (!c || !target) return;
    socket.send({ t: 'kick', ...target, ban: c.kind === 'ban' });
    confirm.value = null;
};
</script>

<template>
    <div class="border-border bg-panel/40 rounded-xl border">
        <button
            type="button"
            class="group flex w-full items-center gap-2 px-3 py-2 text-left"
            :class="{ 'border-border border-b': !collapsed }"
            :aria-expanded="!collapsed"
            :title="collapsed ? 'Show the members' : 'Hide the members'"
            @click="toggle"
        >
            <Icon name="heroicons:users" size="13" class="text-lava" />
            <span class="text-xs font-semibold text-white">Members</span>
            <span class="text-dim font-mono text-[10px]">{{ socket.members.value.length }}</span>
            <span class="text-dim ml-auto font-mono text-[10px]">{{ onlineCount }} online</span>
            <Icon
                name="heroicons:chevron-down"
                size="12"
                class="text-dim transition-transform group-hover:text-white"
                :class="{ '-rotate-90': collapsed }"
            />
        </button>
        <div v-if="!collapsed" class="max-h-40 overflow-y-auto p-1">
            <TogetherRoomMemberRow
                v-for="m in sorted"
                :key="memberKey(m)"
                :member="m"
                :is-me="isMine(m.user_id, m.anon_number)"
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
