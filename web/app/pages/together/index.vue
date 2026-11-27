<script setup lang="ts">
import type { LobbySummary } from '~/types/together';

// Goon2gether lobby browser: my lobbies, public lobbies, create, join with a code.
// Moderators can list every lobby (also private) and close them.
useHead({ title: 'Together' });

const perms = useMyPermissions();
const { listLobbies, closeLobby } = useApiTogether();
const togetherLobbies = useTogetherLobbiesStore();

const lobbies = ref<LobbySummary[]>([]);
const loading = ref(true);
const error = ref('');
const showAll = ref(false);
const showCreate = ref(false);
const closing = ref<LobbySummary | null>(null);

const canHost = computed(() => perms.has('goon2gether:host'));
const canModerate = computed(() => perms.has('goon2gether:moderate'));
const myUserId = computed(() => useAuthStore().user?.id ?? null);

const mine = computed(() => lobbies.value.filter((l) => l.is_member));
const others = computed(() => lobbies.value.filter((l) => !l.is_member));
const othersTitle = computed(() => (showAll.value ? 'All lobbies' : 'Public lobbies'));

const load = async () => {
    error.value = '';
    try {
        const res = await listLobbies(showAll.value && canModerate.value);
        lobbies.value = res.lobbies ?? [];
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to load the lobbies';
    } finally {
        loading.value = false;
    }
};

let refresh: ReturnType<typeof setInterval> | null = null;
onMounted(() => {
    load();
    // Online counts and now playing move: refreshed while the page is open
    refresh = setInterval(load, 15000);
});
onBeforeUnmount(() => {
    if (refresh) clearInterval(refresh);
});
watch(showAll, load);
// A public lobby opened or closed elsewhere (live count of the nav badge)
watch(() => togetherLobbies.publicCount, load);

const doClose = async () => {
    const lobby = closing.value;
    closing.value = null;
    if (!lobby) return;
    try {
        await closeLobby(lobby.id);
        await load();
    } catch (e: unknown) {
        error.value = e instanceof Error ? e.message : 'Failed to close the lobby';
    }
};

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="mx-auto max-w-415 p-4 sm:px-5 lg:py-6">
        <div class="mb-6 flex flex-wrap items-end gap-4">
            <div>
                <h1 class="flex items-center gap-2 text-lg font-semibold text-white">
                    <Icon name="heroicons:user-group" size="18" class="text-lava" />
                    Together
                </h1>
                <p class="text-dim mt-0.5 text-xs">
                    Watch scenes in sync with others, with a shared queue and chat.
                </p>
            </div>
            <div class="ml-auto flex flex-wrap items-center gap-3">
                <TogetherLobbyJoinCode />
                <button
                    v-if="canHost"
                    class="bg-lava hover:bg-lava/90 flex items-center gap-1.5 rounded-lg px-3 py-1.5
                        text-xs font-medium text-white transition-colors"
                    @click="showCreate = true"
                >
                    <Icon name="heroicons:plus" size="13" />
                    Create lobby
                </button>
            </div>
        </div>

        <ErrorAlert v-if="error" :message="error" class="mb-4" />

        <div v-if="loading" class="flex h-48 items-center justify-center">
            <LoadingSpinner label="Loading lobbies..." />
        </div>

        <template v-else>
            <section v-if="mine.length > 0" class="mb-8">
                <h2 class="text-dim mb-3 text-[11px] font-semibold tracking-wider uppercase">
                    My lobbies
                </h2>
                <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    <TogetherLobbyCard
                        v-for="l in mine"
                        :key="l.id"
                        :lobby="l"
                        :can-close="canModerate || l.host?.id === myUserId"
                        @close="closing = $event"
                    />
                </div>
            </section>

            <section>
                <div class="mb-3 flex items-center gap-3">
                    <h2 class="text-dim text-[11px] font-semibold tracking-wider uppercase">
                        {{ othersTitle }}
                    </h2>
                    <label
                        v-if="canModerate"
                        class="text-dim ml-auto flex items-center gap-2 text-[11px]"
                    >
                        All lobbies
                        <UiToggle v-model="showAll" />
                    </label>
                </div>
                <div v-if="others.length > 0" class="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                    <TogetherLobbyCard
                        v-for="l in others"
                        :key="l.id"
                        :lobby="l"
                        :can-close="canModerate"
                        @close="closing = $event"
                    />
                </div>
                <div
                    v-else
                    class="border-border text-dim flex flex-col items-center gap-2 rounded-xl border
                        border-dashed py-10 text-center text-xs"
                >
                    <Icon name="heroicons:tv" size="22" />
                    No public lobby right now.
                    <span v-if="canHost">Create one and invite people with its link.</span>
                </div>
            </section>
        </template>

        <TogetherLobbyCreateModal :visible="showCreate" @close="showCreate = false" />

        <UiConfirmModal
            :visible="!!closing"
            title="Close this lobby?"
            confirm-label="Close lobby"
            @close="closing = null"
            @confirm="doClose"
        >
            <p>"{{ closing?.name }}" is closed for everyone: its queue and chat are deleted.</p>
        </UiConfirmModal>
    </div>
</template>
