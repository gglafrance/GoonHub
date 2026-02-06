<script setup lang="ts">
import { seededShuffle } from '~/composables/usePlaylistPlayer';

definePageMeta({ middleware: 'auth' });

const route = useRoute();
const router = useRouter();
const store = usePlaylistStore();
const authStore = useAuthStore();

const uuid = computed(() => route.params.uuid as string);
const isOwner = computed(
    () => !!authStore.user && store.currentPlaylist?.owner.id === authStore.user.id,
);

const showEditModal = ref(false);
const showDeleteConfirm = ref(false);

useHead({
    title: computed(() => store.currentPlaylist?.name || 'Playlist'),
});

onMounted(() => {
    store.loadPlaylist(uuid.value);
});

watch(uuid, (newUuid) => {
    store.loadPlaylist(newUuid);
});

const handleToggleLike = async () => {
    if (!store.currentPlaylist) return;
    await store.toggleLike(store.currentPlaylist.uuid);
};

const handleEdit = () => {
    showEditModal.value = true;
};

const handleUpdated = () => {
    showEditModal.value = false;
    store.loadPlaylist(uuid.value);
};

const handleDelete = async () => {
    if (!store.currentPlaylist) return;
    const ok = await store.deletePlaylist(store.currentPlaylist.uuid);
    if (ok) {
        router.push('/playlists');
    }
};

const handleRemoveScene = async (sceneId: number) => {
    if (!store.currentPlaylist) return;
    await store.removeScene(store.currentPlaylist.uuid, sceneId);
};

const handleReorder = async (sceneIds: number[]) => {
    if (!store.currentPlaylist) return;
    await store.reorderScenes(store.currentPlaylist.uuid, sceneIds);
};

const handlePlay = (index: number) => {
    if (!store.currentPlaylist?.scenes[index]) return;
    const sceneId = store.currentPlaylist.scenes[index].scene.id;
    router.push({
        path: `/watch/${sceneId}`,
        query: { playlist: uuid.value, pos: String(index) },
    });
};

const handlePlayAll = () => {
    if (!store.currentPlaylist?.scenes.length) return;
    handlePlay(0);
};

const handleShuffle = () => {
    if (!store.currentPlaylist?.scenes.length) return;
    const seed = Math.floor(Math.random() * 0xffffffff);
    const order = seededShuffle(store.currentPlaylist.scenes.length, seed);
    const firstSceneId = store.currentPlaylist.scenes[order[0]].scene.id;
    router.push({
        path: `/watch/${firstSceneId}`,
        query: { playlist: uuid.value, pos: '0', shuffle: String(seed) },
    });
};
</script>

<template>
    <div class="mx-auto max-w-5xl px-4 py-6 sm:px-6">
        <!-- Back button -->
        <NuxtLink
            to="/playlists"
            class="text-dim mb-4 inline-flex items-center gap-1 text-xs transition-colors
                hover:text-white"
        >
            <Icon name="heroicons:arrow-left" size="14" />
            Back to Playlists
        </NuxtLink>

        <!-- Loading -->
        <div v-if="store.isLoadingDetail" class="flex justify-center py-16">
            <LoadingSpinner />
        </div>

        <!-- Error -->
        <ErrorAlert v-else-if="store.detailError" :message="store.detailError" />

        <!-- Content -->
        <template v-else-if="store.currentPlaylist">
            <PlaylistHeader
                :playlist="store.currentPlaylist"
                :is-owner="isOwner"
                @edit="handleEdit"
                @delete="showDeleteConfirm = true"
                @toggle-like="handleToggleLike"
            />

            <!-- Action bar -->
            <div class="mb-6 flex flex-wrap gap-2">
                <button
                    :disabled="store.currentPlaylist.scenes.length === 0"
                    class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-4 py-2
                        text-xs font-semibold text-white transition-all disabled:cursor-not-allowed
                        disabled:opacity-40"
                    @click="handlePlayAll"
                >
                    <Icon name="heroicons:play-solid" size="14" />
                    Play All
                </button>
                <button
                    :disabled="store.currentPlaylist.scenes.length === 0"
                    class="border-border bg-surface text-dim hover:border-border-hover
                        hover:bg-elevated flex items-center gap-1.5 rounded-lg border px-3 py-2
                        text-xs font-medium transition-all hover:text-white
                        disabled:cursor-not-allowed disabled:opacity-40"
                    @click="handleShuffle"
                >
                    <Icon name="heroicons:arrows-right-left" size="14" />
                    Shuffle
                </button>
            </div>

            <!-- Scene list -->
            <PlaylistSceneList
                :scenes="store.currentPlaylist.scenes"
                :is-owner="isOwner"
                @remove="handleRemoveScene"
                @reorder="handleReorder"
                @play="handlePlay"
            />

            <!-- Edit Modal -->
            <PlaylistEditModal
                :visible="showEditModal"
                :playlist="store.currentPlaylist"
                @close="showEditModal = false"
                @updated="handleUpdated"
            />

            <!-- Delete Confirmation -->
            <Teleport to="body">
                <div
                    v-if="showDeleteConfirm"
                    class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4
                        backdrop-blur-sm"
                    @click.self="showDeleteConfirm = false"
                >
                    <div class="glass-panel border-border w-full max-w-sm border p-6">
                        <h3 class="mb-2 text-sm font-semibold text-white">Delete Playlist</h3>
                        <p class="text-dim mb-4 text-xs">
                            Are you sure you want to delete "{{ store.currentPlaylist.name }}"? This
                            cannot be undone.
                        </p>
                        <div class="flex justify-end gap-2">
                            <button
                                class="text-dim rounded-lg px-3 py-1.5 text-xs transition-colors
                                    hover:text-white"
                                @click="showDeleteConfirm = false"
                            >
                                Cancel
                            </button>
                            <button
                                class="bg-lava hover:bg-lava-glow rounded-lg px-4 py-1.5 text-xs
                                    font-semibold text-white transition-all"
                                @click="handleDelete"
                            >
                                Delete
                            </button>
                        </div>
                    </div>
                </div>
            </Teleport>
        </template>
    </div>
</template>
