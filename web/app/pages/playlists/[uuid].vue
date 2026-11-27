<script setup lang="ts">
import { seededShuffle } from '~/composables/usePlaylistPlayer';

definePageMeta({ middleware: 'auth' });

const route = useRoute();
const router = useRouter();
const store = usePlaylistStore();
const authStore = useAuthStore();
const { formatDuration } = useFormatter();

const uuid = computed(() => route.params.uuid as string);
const isOwner = computed(
    () => !!authStore.user && store.currentPlaylist?.owner.id === authStore.user.id,
);

// Back to the playlists list with its filters and scroll position when coming from it
const { backTo } = useNavOrigin();

const showEditModal = ref(false);
const showDeleteConfirm = ref(false);

const pageTitle = computed(() => store.currentPlaylist?.name || 'Playlist');

// A playlist holds scenes or markers (docs/MARKER_PLAYLISTS.md); playback works on its items
const isMarkers = computed(() => store.currentPlaylist?.kind === 'markers');
const items = computed(() =>
    store.currentPlaylist ? playlistQueueItems(store.currentPlaylist) : [],
);

useHead({ title: pageTitle });

const seoDescription = computed(() => {
    const p = store.currentPlaylist;
    if (!p) return 'View playlist';
    const count = items.value.length;
    const word = p.kind === 'markers' ? 'markers' : 'scenes';
    return `${count} ${word} in ${p.name}`;
});

useSeoMeta({
    title: pageTitle,
    ogTitle: computed(() => `${pageTitle.value} - GoonHub`),
    description: seoDescription,
    ogDescription: seoDescription,
});

onMounted(() => {
    store.loadPlaylist(uuid.value);
});

watch(uuid, (newUuid) => {
    store.loadPlaylist(newUuid);
});

// Resume button: the last item played (scene or marker) and the time in its scene
const resumeData = computed(() => {
    if (!store.currentPlaylist) return null;
    const target = playlistResumeTarget(store.currentPlaylist);
    if (!target) return null;
    return {
        sceneId: target.item.sceneId,
        positionS: target.startAt,
        index: target.index,
        item: target.item,
        title: target.item.title,
    };
});

const handleResume = () => {
    if (!resumeData.value) return;
    router.push({
        path: `/watch/${resumeData.value.sceneId}`,
        query: playlistItemQuery(
            uuid.value,
            resumeData.value.item,
            resumeData.value.index,
            0,
            resumeData.value.positionS,
        ),
    });
};

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

const handleRemoveScenes = async (sceneIds: number[]) => {
    if (!store.currentPlaylist) return;
    await store.removeScenes(store.currentPlaylist.uuid, sceneIds);
};

const handleReorder = async (sceneIds: number[]) => {
    if (!store.currentPlaylist) return;
    await store.reorderScenes(store.currentPlaylist.uuid, sceneIds);
};

const handleRemoveMarkers = async (markerIds: number[]) => {
    if (!store.currentPlaylist) return;
    await store.removeMarkers(store.currentPlaylist.uuid, markerIds);
};

const handleReorderMarkers = async (markerIds: number[]) => {
    if (!store.currentPlaylist) return;
    await store.reorderMarkers(store.currentPlaylist.uuid, markerIds);
};

const handlePlay = (index: number) => {
    const item = items.value[index];
    if (!item) return;
    router.push({
        path: `/watch/${item.sceneId}`,
        query: playlistItemQuery(uuid.value, item, index),
    });
};

const handlePlayAll = () => {
    if (!items.value.length) return;
    handlePlay(0);
};

const handleShuffle = () => {
    if (!items.value.length) return;
    const seed = Math.floor(Math.random() * 0xffffffff);
    const order = seededShuffle(items.value.length, seed);
    const firstIndex = order[0];
    if (firstIndex === undefined) return;
    const first = items.value[firstIndex];
    if (!first) return;
    router.push({
        path: `/watch/${first.sceneId}`,
        query: playlistItemQuery(uuid.value, first, 0, seed),
    });
};
</script>

<template>
    <div class="mx-auto max-w-5xl px-4 py-6 sm:px-6">
        <!-- Back button -->
        <button
            class="text-dim mb-4 inline-flex items-center gap-1 text-xs transition-colors
                hover:text-white"
            @click="backTo('/playlists')"
        >
            <Icon name="heroicons:arrow-left" size="14" />
            Back to Playlists
        </button>

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
                    :disabled="items.length === 0"
                    class="bg-lava hover:bg-lava-glow flex items-center gap-1.5 rounded-lg px-4 py-2
                        text-xs font-semibold text-white transition-all disabled:cursor-not-allowed
                        disabled:opacity-40"
                    @click="handlePlayAll"
                >
                    <Icon name="heroicons:play-solid" size="14" />
                    Play All
                </button>
                <button
                    v-if="resumeData"
                    class="border-border bg-surface text-dim hover:border-border-hover
                        hover:bg-elevated flex items-center gap-1.5 rounded-lg border px-3 py-2
                        text-xs font-medium transition-all hover:text-white"
                    @click="handleResume"
                >
                    <Icon name="heroicons:play-pause" size="14" />
                    Resume
                    <span class="max-w-32 truncate opacity-60">
                        {{ resumeData.title }}
                    </span>
                    <span class="font-mono opacity-50">
                        {{ formatDuration(Math.floor(resumeData.positionS)) }}
                    </span>
                </button>
                <button
                    :disabled="items.length === 0"
                    class="border-border bg-surface text-dim hover:border-border-hover
                        hover:bg-elevated flex items-center gap-1.5 rounded-lg border px-3 py-2
                        text-xs font-medium transition-all hover:text-white
                        disabled:cursor-not-allowed disabled:opacity-40"
                    @click="handleShuffle"
                >
                    <Icon name="heroicons:arrows-right-left" size="14" />
                    Shuffle
                </button>
                <!-- Send to friends: only a public playlist can be seen by them -->
                <ShareButton
                    v-if="store.currentPlaylist.visibility === 'public'"
                    :subject="{
                        kind: 'playlist',
                        uuid: store.currentPlaylist.uuid,
                        name: store.currentPlaylist.name,
                    }"
                    label="Send to friends"
                />

                <!-- List / table -->
                <SceneTableViewControls
                    v-if="!isMarkers && store.currentPlaylist.scenes.length > 0"
                    class="ml-auto"
                    page="playlist"
                    grid-label="List"
                    grid-icon="heroicons:list-bullet"
                    :hidden-columns="['rating', 'actors']"
                />
            </div>

            <!-- Marker list (marker playlists) -->
            <PlaylistMarkerList
                v-if="isMarkers"
                :markers="store.currentPlaylist.markers"
                :is-owner="isOwner"
                @remove="(id: number) => handleRemoveMarkers([id])"
                @remove-many="handleRemoveMarkers"
                @reorder="handleReorderMarkers"
                @play="handlePlay"
            />

            <!-- Scene list -->
            <PlaylistSceneList
                v-else
                :scenes="store.currentPlaylist.scenes"
                :is-owner="isOwner"
                :playlist-uuid="uuid"
                @remove="handleRemoveScene"
                @remove-many="handleRemoveScenes"
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
