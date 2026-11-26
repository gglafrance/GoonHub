<script setup lang="ts">
import type { LobbySummary } from '~/types/together';

// Goon2gether room: a member connects at once; a non-member gets the join screen (join, or
// anonymously for a public lobby), or joins directly with ?join=named|anonymous (lobby
// browser). Layout: player, reactions and queue on the left; members and chat on the right.
const route = useRoute();
const settingsStore = useSettingsStore();
const { getLobby, joinLobby } = useApiTogether();

const lobbyId = computed(() => String(route.params.id ?? ''));
const room = useTogetherRoom(lobbyId);
provide(TOGETHER_ROOM_KEY, room);
const { socket } = room;

const loadError = ref('');
const ready = ref(false);
const showSettings = ref(false);
// not a member yet: the join screen
const pending = ref<LobbySummary | null>(null);
const joining = ref(false);
const joinError = ref('');

const lobbyName = computed(() => socket.lobby.value?.name ?? pending.value?.name ?? 'Together');
useHead({ title: lobbyName });

const closedForInactivity = computed(() => socket.byeReason.value === 'inactive');

const errorText = (e: unknown) => {
    const status = (e as { status?: number }).status;
    return status === 404
        ? 'This lobby does not exist anymore, or it is private: ask for an invite link.'
        : status === 403
          ? 'You are banned from this lobby.'
          : e instanceof Error
            ? e.message
            : 'Failed to open the lobby';
};

const join = async (anonymous: boolean) => {
    joining.value = true;
    joinError.value = '';
    try {
        await joinLobby(lobbyId.value, anonymous);
        pending.value = null;
        ready.value = true;
        socket.start();
    } catch (e: unknown) {
        joinError.value = errorText(e);
    } finally {
        joining.value = false;
    }
};

const enter = async () => {
    ready.value = false;
    loadError.value = '';
    pending.value = null;
    joinError.value = '';
    try {
        const summary = await getLobby(lobbyId.value);
        if (!summary.is_member) {
            const how = route.query.join;
            if (how === 'named' || how === 'anonymous') {
                pending.value = summary;
                await join(how === 'anonymous');
                if (joinError.value) loadError.value = joinError.value;
                return;
            }
            pending.value = summary;
            return;
        }
        ready.value = true;
        socket.start();
    } catch (e: unknown) {
        loadError.value = errorText(e);
    }
};

onMounted(enter);
watch(lobbyId, () => {
    socket.disconnect();
    enter();
});

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen">
        <TogetherRoomHeader @settings="showSettings = true" />

        <div class="mx-auto max-w-415 p-4 sm:px-5 lg:py-6">
            <div
                v-if="loadError || socket.finalReason.value"
                class="flex h-[60vh] flex-col items-center justify-center text-center"
            >
                <div
                    class="border-lava/20 bg-lava/5 flex h-12 w-12 items-center justify-center
                        rounded-xl border"
                >
                    <Icon
                        :name="closedForInactivity ? 'heroicons:moon' : 'heroicons:user-group'"
                        size="24"
                        class="text-lava"
                    />
                </div>
                <h2 class="mt-4 text-lg font-semibold text-white">
                    {{ closedForInactivity ? 'Lobby closed for inactivity' : 'Lobby unavailable' }}
                </h2>
                <p class="text-dim mt-1 max-w-sm text-xs">
                    {{ loadError || socket.finalReason.value }}
                </p>
                <NuxtLink
                    to="/together"
                    class="border-border bg-surface text-muted hover:border-border-hover mt-6
                        rounded-lg border px-4 py-2 text-xs font-medium transition-all
                        hover:text-white"
                >
                    Back to the lobbies
                </NuxtLink>
            </div>

            <TogetherRoomJoinPrompt
                v-else-if="pending"
                :lobby="pending"
                :busy="joining"
                :error="joinError"
                @join="join"
            />

            <div
                v-else-if="!ready || !socket.connectedOnce.value"
                class="flex h-[60vh] items-center justify-center"
            >
                <LoadingSpinner label="Joining the lobby..." />
            </div>

            <div
                v-else
                :class="['grid gap-5', settingsStore.theaterMode ? '' : 'xl:grid-cols-[1fr_340px]']"
            >
                <div class="min-w-0 space-y-3">
                    <TogetherRoomClosingBanner />
                    <TogetherRoomPlayer />
                    <div class="flex flex-wrap items-start gap-2">
                        <TogetherRoomReactionBar class="min-w-0 flex-1" />
                        <TogetherRoomAutoNext />
                    </div>
                    <TogetherQueuePanel />
                </div>

                <div class="flex min-w-0 flex-col gap-3 xl:h-[calc(100vh-8rem)]">
                    <TogetherRoomMembers />
                    <TogetherChatPanel class="h-[28rem] xl:h-auto xl:flex-1" />
                </div>
            </div>
        </div>

        <!-- Refused commands and errors -->
        <Transition
            enter-active-class="transition duration-200 ease-out"
            enter-from-class="translate-y-2 opacity-0"
            leave-active-class="transition duration-150 ease-in"
            leave-to-class="translate-y-2 opacity-0"
        >
            <div
                v-if="socket.lastError.value"
                class="border-lava/30 bg-void/90 fixed bottom-6 left-1/2 z-50 -translate-x-1/2
                    rounded-lg border px-4 py-2 text-xs text-white shadow-2xl backdrop-blur-md"
            >
                <Icon name="heroicons:exclamation-triangle" size="13" class="text-lava mr-1.5" />
                {{ socket.lastError.value.message }}
            </div>
        </Transition>

        <TogetherRoomSettingsModal
            :visible="showSettings"
            :lobby="socket.lobby.value"
            @close="showSettings = false"
        />
    </div>
</template>
