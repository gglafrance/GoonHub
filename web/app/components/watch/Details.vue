<script setup lang="ts">
import type { Video } from '~/types/video';

const video = inject<Ref<Video | null>>('watchVideo');
const thumbnailVersion = inject<Ref<number>>('thumbnailVersion');
const detailsRefreshKey = inject<Ref<number>>('detailsRefreshKey');
const authStore = useAuthStore();
const { updateVideoDetails, fetchVideoInteractions, fetchVideo, getPornDBStatus } = useApi();

const error = ref<string | null>(null);
const saving = ref(false);
const saved = ref(false);
let savedTimeout: ReturnType<typeof setTimeout> | null = null;

// Fetch metadata modal state
const showFetchMetadataModal = ref(false);
const pornDBConfigured = ref(false);
const checkingPornDB = ref(false);

// Interactions state
const initialRating = ref(0);
const initialLiked = ref(false);
const initialJizzedCount = ref(0);

// Tag manager ref for reloading
const tagManagerRef = ref<{ reload: () => void } | null>(null);

const isAdmin = computed(() => authStore.user?.role === 'admin');

async function checkPornDBStatus() {
    if (!isAdmin.value) return;
    checkingPornDB.value = true;
    try {
        const status = await getPornDBStatus();
        pornDBConfigured.value = status.configured;
    } catch {
        pornDBConfigured.value = false;
    } finally {
        checkingPornDB.value = false;
    }
}

async function loadInteractions() {
    if (!video?.value) return;
    try {
        const res = await fetchVideoInteractions(video.value.id);
        initialRating.value = res.rating || 0;
        initialLiked.value = res.liked || false;
        initialJizzedCount.value = res.jizzed_count || 0;
    } catch {
        // Silently fail for interactions
    }
}

async function handleMetadataApplied() {
    // Refresh video data after metadata is applied
    if (video?.value) {
        try {
            const updated = await fetchVideo(video.value.id);
            if (video.value) {
                Object.assign(video.value, updated);
            }
            // Bust thumbnail cache in case it was updated
            if (thumbnailVersion) {
                thumbnailVersion.value = Date.now();
            }
            // Reload tags since metadata apply may have changed them
            tagManagerRef.value?.reload();
            // Signal child components (e.g. Actors) to refresh
            if (detailsRefreshKey) {
                detailsRefreshKey.value++;
            }
        } catch {
            // Silently fail refresh
        }
    }
}

async function saveTitle(title: string) {
    if (!video?.value) return;
    await saveDetails(title, video.value.description || '');
}

async function saveDescription(description: string) {
    if (!video?.value) return;
    await saveDetails(video.value.title || '', description);
}

async function saveReleaseDate(releaseDate: string | null) {
    if (!video?.value) return;

    saving.value = true;
    error.value = null;

    try {
        const updated = await updateVideoDetails(
            video.value.id,
            video.value.title,
            video.value.description || '',
            releaseDate,
        );
        if (video.value) {
            video.value.release_date = updated.release_date;
        }
        showSavedIndicator();
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to save release date';
    } finally {
        saving.value = false;
    }
}

async function saveDetails(title: string, description: string) {
    if (!video?.value) return;

    saving.value = true;
    error.value = null;

    try {
        const updated = await updateVideoDetails(video.value.id, title, description);
        if (video.value) {
            video.value.title = updated.title;
            video.value.description = updated.description;
        }
        showSavedIndicator();
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to save details';
    } finally {
        saving.value = false;
    }
}

function showSavedIndicator() {
    saved.value = true;
    if (savedTimeout) clearTimeout(savedTimeout);
    savedTimeout = setTimeout(() => {
        saved.value = false;
    }, 2000);
}

onMounted(async () => {
    await Promise.all([loadInteractions(), checkPornDBStatus()]);
});
</script>

<template>
    <div class="space-y-5">
        <!-- Error -->
        <div
            v-if="error"
            class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
        >
            <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
            <span class="text-xs text-red-300">{{ error }}</span>
        </div>

        <!-- Top section: Title/Description + Engagement -->
        <div class="flex gap-5">
            <!-- Left: Title & Description card -->
            <div class="border-border/50 bg-surface/30 min-w-0 flex-1 rounded-xl border p-4">
                <div class="space-y-4">
                    <!-- Title -->
                    <WatchDetailsTitleEditor
                        :title="video?.title || ''"
                        :saved="saved"
                        @save="saveTitle"
                    />

                    <!-- Description -->
                    <WatchDetailsDescriptionEditor
                        :description="video?.description || ''"
                        @save="saveDescription"
                    />
                </div>
            </div>

            <!-- Right: Engagement card -->
            <div
                class="border-border/50 bg-surface/30 flex w-36 shrink-0 flex-col items-center
                    justify-center gap-4 rounded-xl border p-4"
            >
                <div class="text-dim text-[10px] font-medium tracking-wider uppercase">
                    Your Rating
                </div>
                <WatchDetailsRatingPanel
                    v-if="video"
                    :video-id="video.id"
                    :initial-rating="initialRating"
                />
                <div class="bg-border/50 h-px w-full" />
                <WatchDetailsInteractionsBar
                    v-if="video"
                    :video-id="video.id"
                    :initial-liked="initialLiked"
                    :initial-jizzed-count="initialJizzedCount"
                />
            </div>
        </div>

        <!-- Metadata row -->
        <div class="flex flex-wrap gap-3">
            <!-- Release Date -->
            <div class="border-border/50 bg-surface/30 w-40 shrink-0 rounded-lg border p-3">
                <WatchDetailsReleaseDateEditor
                    :release-date="video?.release_date || null"
                    @save="saveReleaseDate"
                />
            </div>

            <!-- Studio -->
            <div class="border-border/50 bg-surface/30 min-w-48 flex-1 rounded-lg border p-3">
                <WatchStudio />
            </div>

            <!-- PornDB -->
            <div class="border-border/50 bg-surface/30 w-44 shrink-0 rounded-lg border p-3">
                <div class="space-y-2">
                    <h3 class="text-dim text-[11px] font-medium tracking-wider uppercase">
                        PornDB
                    </h3>
                    <!-- Linked scene -->
                    <a
                        v-if="video?.porndb_scene_id"
                        :href="`https://theporndb.net/scenes/${video.porndb_scene_id}`"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="text-lava hover:text-lava-glow inline-flex items-center gap-1.5
                            text-xs transition-colors"
                    >
                        View scene
                        <Icon name="heroicons:arrow-top-right-on-square" size="10" />
                    </a>
                    <p v-else class="text-dim text-xs">Not linked</p>
                    <!-- Fetch/Refresh button -->
                    <button
                        v-if="isAdmin && pornDBConfigured"
                        @click="showFetchMetadataModal = true"
                        class="hover:border-lava/40 hover:text-lava text-dim flex w-full
                            items-center justify-center gap-2 rounded-md border border-dashed
                            border-white/10 px-2 py-1.5 text-xs transition-colors"
                    >
                        <Icon name="heroicons:cloud-arrow-down" size="12" />
                        {{ video?.porndb_scene_id ? 'Refresh' : 'Fetch' }}
                    </button>
                </div>
            </div>
        </div>

        <!-- Tags section -->
        <div class="border-border/50 bg-surface/30 rounded-xl border p-4">
            <WatchDetailsTagManager v-if="video" ref="tagManagerRef" :video-id="video.id" />
        </div>

        <!-- Actors section -->
        <div class="border-border/50 bg-surface/30 rounded-xl border p-4">
            <WatchActors />
        </div>
    </div>

    <!-- Fetch Scene Metadata Modal -->
    <WatchFetchSceneMetadataModal
        v-if="video"
        :visible="showFetchMetadataModal"
        :video="video"
        @close="showFetchMetadataModal = false"
        @applied="handleMetadataApplied"
    />
</template>
