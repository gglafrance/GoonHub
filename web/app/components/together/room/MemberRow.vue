<script setup lang="ts">
import type { LobbyMember } from '~/types/together';

// One member: badges (host, control), online and buffering state, actions for the host
// (control, host transfer, kick, ban) and moderators (kick, ban).
const props = defineProps<{
    member: LobbyMember;
    isMe: boolean;
    asHost: boolean;
    asModerator: boolean;
    controlOpen: boolean;
}>();

const emit = defineEmits<{
    action: [kind: 'grant' | 'revoke' | 'host' | 'kick' | 'ban', member: LobbyMember];
}>();

const open = ref(false);
const root = ref<HTMLElement | null>(null);

const actions = computed(() => {
    const m = props.member;
    const list: {
        kind: 'grant' | 'revoke' | 'host' | 'kick' | 'ban';
        label: string;
        icon: string;
        danger?: boolean;
    }[] = [];
    if (props.isMe || m.host) return list;
    if (props.asHost && !props.controlOpen) {
        list.push(
            m.control
                ? { kind: 'revoke', label: 'Revoke control', icon: 'heroicons:hand-raised' }
                : { kind: 'grant', label: 'Give control', icon: 'heroicons:play-pause' },
        );
    }
    if (props.asHost && m.online) {
        list.push({ kind: 'host', label: 'Make host', icon: 'heroicons:star' });
    }
    if (props.asHost || props.asModerator) {
        list.push({
            kind: 'kick',
            label: 'Kick',
            icon: 'heroicons:arrow-right-start-on-rectangle',
            danger: true,
        });
        list.push({
            kind: 'ban',
            label: 'Ban from lobby',
            icon: 'heroicons:no-symbol',
            danger: true,
        });
    }
    return list;
});

const run = (kind: 'grant' | 'revoke' | 'host' | 'kick' | 'ban') => {
    open.value = false;
    emit('action', kind, props.member);
};

const onDocumentClick = (e: MouseEvent) => {
    if (open.value && root.value && !root.value.contains(e.target as Node)) open.value = false;
};
onMounted(() => document.addEventListener('click', onDocumentClick));
onBeforeUnmount(() => document.removeEventListener('click', onDocumentClick));
</script>

<template>
    <div
        ref="root"
        class="group relative flex items-center gap-2 rounded-md px-2 py-1.5 transition-colors
            hover:bg-white/[0.03]"
    >
        <div class="relative shrink-0">
            <div
                class="border-border bg-surface flex h-6 w-6 items-center justify-center
                    rounded-full border text-[10px] font-semibold text-white"
                :class="{ 'opacity-40': !member.online }"
            >
                {{ member.username.charAt(0).toUpperCase() }}
            </div>
            <span
                class="border-void absolute -right-0.5 -bottom-0.5 h-2 w-2 rounded-full border"
                :class="
                    !member.online
                        ? 'bg-white/20'
                        : member.buffering
                          ? 'bg-amber-400'
                          : 'bg-emerald'
                "
                :title="!member.online ? 'Offline' : member.buffering ? 'Buffering' : 'Online'"
            />
        </div>
        <div class="min-w-0 flex-1">
            <div class="flex items-center gap-1">
                <span class="truncate text-xs" :class="member.online ? 'text-white' : 'text-dim'">
                    {{ member.username }}
                </span>
                <span v-if="isMe" class="text-dim text-[10px]">(you)</span>
            </div>
            <div v-if="member.buffering && member.online" class="text-[10px] text-amber-400">
                Buffering...
            </div>
        </div>
        <span
            v-if="member.host"
            class="bg-lava/15 text-lava rounded px-1.5 py-px text-[9px] font-semibold tracking-wider
                uppercase"
        >
            Host
        </span>
        <span
            v-else-if="member.control"
            class="bg-emerald/15 text-emerald rounded px-1.5 py-px text-[9px] font-semibold
                tracking-wider uppercase"
            title="Can control playback and the queue"
        >
            Control
        </span>
        <button
            v-if="actions.length > 0"
            class="text-dim flex h-5 w-5 items-center justify-center rounded transition-colors
                hover:text-white"
            title="Member actions"
            @click.stop="open = !open"
        >
            <Icon name="heroicons:ellipsis-vertical" size="13" />
        </button>
        <div
            v-if="open"
            class="border-border bg-panel/95 absolute top-full right-1 z-30 mt-1 w-44
                overflow-hidden rounded-lg border shadow-2xl backdrop-blur-md"
        >
            <button
                v-for="a in actions"
                :key="a.kind"
                class="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs
                    transition-colors hover:bg-white/5"
                :class="a.danger ? 'text-red-400' : 'text-white'"
                @click="run(a.kind)"
            >
                <Icon :name="a.icon" size="13" />
                {{ a.label }}
            </button>
        </div>
    </div>
</template>
