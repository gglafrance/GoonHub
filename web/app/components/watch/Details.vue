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
    <div class="flex min-h-64 gap-6">
        <!-- Left column: existing content -->
        <div class="min-w-0 flex-1 space-y-4">
            <!-- Error -->
            <div
                v-if="error"
                class="border-lava/30 bg-lava/5 flex items-center gap-2 rounded-lg border px-3 py-2"
            >
                <Icon name="heroicons:exclamation-triangle" size="14" class="text-lava" />
                <span class="text-xs text-red-300">{{ error }}</span>
            </div>

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

            <!-- Release Date -->
            <WatchDetailsReleaseDateEditor
                :release-date="video?.release_date || null"
                @save="saveReleaseDate"
            />

            <!-- PornDB Link -->
            <WatchDetailsPornDBStatus
                v-if="video?.porndb_scene_id"
                :scene-id="video.porndb_scene_id"
            />

            <!-- Tags section -->
            <WatchDetailsTagManager v-if="video" ref="tagManagerRef" :video-id="video.id" />

            <!-- Actors section -->
            <WatchActors />

            <!-- Fetch Metadata button (admin only, PornDB configured) -->
            <button
                v-if="isAdmin && pornDBConfigured"
                @click="showFetchMetadataModal = true"
                class="border-border hover:border-lava/40 hover:text-lava text-dim flex items-center
                    gap-2 rounded-lg border px-3 py-2 text-xs transition-colors"
            >
                <Icon name="heroicons:cloud-arrow-down" size="14" />
                Fetch Scene Metadata
            </button>
        </div>

        <!-- Right column: Rating & Actions -->
        <div class="flex shrink-0 flex-col items-center gap-2.5">
            <WatchDetailsRatingPanel
                v-if="video"
                :video-id="video.id"
                :initial-rating="initialRating"
            />

            <WatchDetailsInteractionsBar
                v-if="video"
                :video-id="video.id"
                :initial-liked="initialLiked"
                :initial-jizzed-count="initialJizzedCount"
            />
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
