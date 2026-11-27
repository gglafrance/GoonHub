<script setup lang="ts">
import type { LobbySummary } from '~/types/together';

// Goon2gether room: a member connects at once; a non-member gets the join screen (join, or
// anonymously for a public lobby), or joins directly with ?join=named|anonymous (lobby
// browser). Layout: see the template; the chat must stay visible with the player.
const route = useRoute();
const settingsStore = useSettingsStore();

// Player and queue on the left, members and chat in a side column from lg (not in theater mode,
// where the player takes the whole width). The side column sticks under the headers and fits
// the viewport: app header 5.25rem + 2px (sm+), room header 3rem + 1px, page padding 1.5rem
// (lg:py-6) above and below. Otherwise one column: player, chat, members, then the queue, so
// that the chat comes right under the player and not after the whole queue.
const sideBySide = computed(() => !settingsStore.theaterMode);
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
                :class="[
                    'grid gap-x-5 gap-y-3',
                    sideBySide &&
                        'lg:grid-cols-[1fr_300px] lg:grid-rows-[auto_1fr] xl:grid-cols-[1fr_340px]',
                ]"
            >
                <div :class="['min-w-0 space-y-3', sideBySide && 'lg:col-start-1 lg:row-start-1']">
                    <TogetherRoomClosingBanner />
                    <TogetherRoomPlayer />
                    <div class="flex flex-wrap items-start gap-2">
                        <TogetherRoomReactionBar class="min-w-0 flex-1" />
                        <TogetherRoomAutoNext />
                    </div>
                </div>

                <!-- Side column: members (foldable, short) then the chat taking the rest; in one
                     column, the chat first -->
                <div
                    :class="[
                        'flex min-w-0 flex-col gap-3',
                        sideBySide &&
                            `lg:sticky lg:top-[calc(9.75rem+3px)] lg:col-start-2 lg:row-span-2
                            lg:row-start-1 lg:h-[calc(100vh-11.25rem-3px)] lg:self-start`,
                    ]"
                >
                    <TogetherChatPanel
                        :class="[
                            'h-[min(28rem,60svh)]',
                            sideBySide && 'lg:h-auto lg:min-h-0 lg:flex-1',
                        ]"
                    />
                    <TogetherRoomMembers :class="['shrink-0', sideBySide && 'lg:order-first']" />
                </div>

                <TogetherQueuePanel
                    :class="['min-w-0 self-start', sideBySide && 'lg:col-start-1 lg:row-start-2']"
                />
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
                class="border-lava/30 bg-void/90 fixed bottom-[calc(1.5rem+var(--app-bottom-clear))]
                    left-1/2 z-50 -translate-x-1/2 rounded-lg border px-4 py-2 text-xs text-white
                    shadow-2xl backdrop-blur-md"
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
