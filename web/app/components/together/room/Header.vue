<script setup lang="ts">
// Room bar: back, lobby name and visibility, connection state, invite link, settings (host),
// leave, close (host and moderators).
const room = useInjectedTogetherRoom();
const { socket, isHost, canModerate, isAnonymous } = room;
const { leaveLobby, closeLobby } = useApiTogether();

const emit = defineEmits<{
    settings: [];
}>();

const { copied, manualText, copy, dismiss } = useClipboard();
const confirmClose = ref(false);
const confirmLeave = ref(false);
const busy = ref(false);

const statusInfo = computed(() => {
    switch (socket.status.value) {
        case 'open':
            return { label: 'Live', class: 'bg-emerald' };
        case 'connecting':
            return { label: 'Connecting', class: 'bg-amber-400 animate-pulse' };
        case 'reconnecting':
            return { label: 'Reconnecting', class: 'bg-amber-400 animate-pulse' };
        default:
            return { label: 'Disconnected', class: 'bg-white/30' };
    }
});

// Plain http (LAN address) has no Clipboard API: copy falls back to execCommand, and when
// that fails too the link is shown selected to be copied by hand
const copyInvite = () => {
    const code = socket.lobby.value?.invite_code;
    if (code) copy(inviteLink(location.origin, code));
};

const run = async (fn: () => Promise<unknown>) => {
    const id = socket.lobby.value?.id;
    if (!id || busy.value) return;
    busy.value = true;
    try {
        await fn();
        socket.disconnect();
        await navigateTo('/together');
    } catch (e: unknown) {
        socket.showError({
            code: 'internal',
            message: e instanceof Error ? e.message : 'Request failed',
        });
    } finally {
        busy.value = false;
        confirmClose.value = confirmLeave.value = false;
    }
};

const leave = () => run(() => leaveLobby(socket.lobby.value!.id));
const close = () => run(() => closeLobby(socket.lobby.value!.id));
</script>

<template>
    <div
        class="border-border bg-void/10 sticky top-12 z-40 border-b px-4 py-2.5 backdrop-blur-md
            sm:px-5"
    >
        <div class="mx-auto flex max-w-415 items-center gap-3">
            <NuxtLink
                to="/together"
                class="group text-dim flex items-center gap-2 transition-colors hover:text-white"
            >
                <div
                    class="border-border bg-panel group-hover:border-lava/30 group-hover:text-lava
                        flex h-6 w-6 items-center justify-center rounded-md border transition-all"
                >
                    <Icon name="heroicons:arrow-left" size="12" />
                </div>
                <span class="hidden text-xs font-medium sm:inline">Lobbies</span>
            </NuxtLink>

            <div class="flex min-w-0 items-center gap-2">
                <h1 class="truncate text-sm font-semibold text-white">
                    {{ socket.lobby.value?.name ?? 'Lobby' }}
                </h1>
                <span
                    v-if="socket.lobby.value"
                    class="border-border text-dim flex items-center gap-1 rounded border px-1.5
                        py-px text-[10px]"
                >
                    <Icon
                        :name="
                            socket.lobby.value.visibility === 'public'
                                ? 'heroicons:globe-alt'
                                : 'heroicons:lock-closed'
                        "
                        size="10"
                    />
                    {{ socket.lobby.value.visibility === 'public' ? 'Public' : 'Private' }}
                </span>
                <span class="text-dim flex items-center gap-1.5 text-[10px]">
                    <span class="h-1.5 w-1.5 rounded-full" :class="statusInfo.class" />
                    {{ statusInfo.label }}
                </span>
                <span
                    v-if="isAnonymous"
                    class="border-border text-dim hidden items-center gap-1 rounded border px-1.5
                        py-px text-[10px] sm:flex"
                    title="The other members see you under this name, without your profile"
                >
                    <Icon name="heroicons:eye-slash" size="10" />
                    You are Anonymous #{{ socket.you.value?.anon_number }}
                </span>
            </div>

            <div class="ml-auto flex items-center gap-1.5">
                <div v-if="socket.lobby.value?.invite_code" class="relative">
                    <button
                        class="border-border bg-panel hover:border-lava/30 hover:text-lava flex h-7
                            items-center gap-1.5 rounded-md border px-2.5 text-[11px] font-medium
                            text-white transition-all"
                        title="Copy the invite link"
                        @click="copyInvite"
                    >
                        <Icon :name="copied ? 'heroicons:check' : 'heroicons:link'" size="13" />
                        <span class="hidden sm:inline">{{ copied ? 'Copied' : 'Invite' }}</span>
                    </button>
                    <UiCopyFallback
                        v-if="manualText"
                        :text="manualText"
                        label="Invite link"
                        @close="dismiss"
                    />
                </div>

                <button
                    v-if="isHost"
                    class="border-border bg-panel text-dim hover:border-lava/30 hover:text-lava flex
                        h-7 w-7 items-center justify-center rounded-md border transition-all"
                    title="Lobby settings"
                    @click="emit('settings')"
                >
                    <Icon name="heroicons:cog-6-tooth" size="14" />
                </button>
                <button
                    class="border-border bg-panel text-dim flex h-7 items-center gap-1.5 rounded-md
                        border px-2.5 text-[11px] font-medium transition-all hover:text-white"
                    @click="confirmLeave = true"
                >
                    <Icon name="heroicons:arrow-right-start-on-rectangle" size="13" />
                    <span class="hidden sm:inline">Leave</span>
                </button>
                <button
                    v-if="isHost || canModerate"
                    class="border-border bg-panel flex h-7 w-7 items-center justify-center
                        rounded-md border text-red-400 transition-all hover:border-red-500/50"
                    title="Close the lobby"
                    @click="confirmClose = true"
                >
                    <Icon name="heroicons:power" size="14" />
                </button>
            </div>
        </div>

        <UiConfirmModal
            :visible="confirmLeave"
            title="Leave this lobby?"
            confirm-label="Leave"
            icon="heroicons:arrow-right-start-on-rectangle"
            @close="confirmLeave = false"
            @confirm="leave"
        >
            <p v-if="isHost">The host role goes to another member.</p>
            <p v-else>You can come back with the invite link.</p>
        </UiConfirmModal>
        <UiConfirmModal
            :visible="confirmClose"
            title="Close this lobby?"
            confirm-label="Close lobby"
            @close="confirmClose = false"
            @confirm="close"
        >
            <p>Everyone is disconnected, and the queue and the chat are deleted.</p>
        </UiConfirmModal>
    </div>
</template>
