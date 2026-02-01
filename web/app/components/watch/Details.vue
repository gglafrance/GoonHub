<script setup lang="ts">
import type { Scene } from '~/types/scene';
import type { WatchPageData } from '~/composables/useWatchPageData';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

const scene = inject<Ref<Scene | null>>('watchScene');
const thumbnailVersion = inject<Ref<number>>('thumbnailVersion');
const detailsRefreshKey = inject<Ref<number>>('detailsRefreshKey');
const authStore = useAuthStore();
const { updateSceneDetails, fetchScene } = useApi();

// Inject centralized watch page data
const watchPageData = inject<WatchPageData>(WATCH_PAGE_DATA_KEY);

const error = ref<string | null>(null);
const saving = ref(false);
const saved = ref(false);
let savedTimeout: ReturnType<typeof setTimeout> | null = null;

// Fetch metadata modal state
const showFetchMetadataModal = ref(false);

// Tag manager ref for reloading
const tagManagerRef = ref<{ reload: () => void } | null>(null);

const isAdmin = computed(() => authStore.user?.role === 'admin');

// Loading state for interactions
const interactionsLoading = computed(() => watchPageData?.loading.details ?? true);

// Interactions from centralized data (with fallback for backwards compatibility)
const initialRating = computed(() => watchPageData?.interactions.value?.rating ?? 0);
const initialLiked = computed(() => watchPageData?.interactions.value?.liked ?? false);
const initialJizzedCount = computed(() => watchPageData?.interactions.value?.jizzed_count ?? 0);

// PornDB status from centralized data
const pornDBConfigured = computed(() => watchPageData?.pornDBConfigured.value ?? false);

async function handleMetadataApplied() {
    // Refresh scene data after metadata is applied
    if (scene?.value) {
        try {
            const updated = await fetchScene(scene.value.id);
            if (scene.value) {
                Object.assign(scene.value, updated);
            }
            // Bust thumbnail cache in case it was updated
            if (thumbnailVersion) {
                thumbnailVersion.value = Date.now();
            }
            // Reload tags since metadata apply may have changed them
            tagManagerRef.value?.reload();
            // Refresh centralized data for studio, tags, actors
            await Promise.all([
                watchPageData?.refreshStudio(),
                watchPageData?.refreshTags(),
                watchPageData?.refreshActors(),
            ]);
            // Signal child components (e.g. Actors) to refresh via legacy key
            if (detailsRefreshKey) {
                detailsRefreshKey.value++;
            }
        } catch {
            // Silently fail refresh
        }
    }
}

async function saveTitle(title: string) {
    if (!scene?.value) return;
    await saveDetails(title, scene.value.description || '');
}

async function saveDescription(description: string) {
    if (!scene?.value) return;
    await saveDetails(scene.value.title || '', description);
}

async function saveReleaseDate(releaseDate: string | null) {
    if (!scene?.value) return;

    saving.value = true;
    error.value = null;

    try {
        const updated = await updateSceneDetails(
            scene.value.id,
            scene.value.title,
            scene.value.description || '',
            releaseDate,
        );
        if (scene.value) {
            scene.value.release_date = updated.release_date;
        }
        showSavedIndicator();
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to save release date';
    } finally {
        saving.value = false;
    }
}

async function saveDetails(title: string, description: string) {
    if (!scene?.value) return;

    saving.value = true;
    error.value = null;

    try {
        const updated = await updateSceneDetails(scene.value.id, title, description);
        if (scene.value) {
            scene.value.title = updated.title;
            scene.value.description = updated.description;
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
                        :title="scene?.title || ''"
                        :saved="saved"
                        @save="saveTitle"
                    />

                    <!-- Description -->
                    <WatchDetailsDescriptionEditor
                        :description="scene?.description || ''"
                        @save="saveDescription"
                    />
                </div>
            </div>

            <!-- Right: Engagement card -->
            <div
                class="border-border/50 bg-surface/30 flex min-h-46 w-36 shrink-0 flex-col
                    items-center justify-center gap-4 rounded-xl border p-4"
            >
                <div class="text-dim text-[10px] font-medium tracking-wider uppercase">
                    Your Rating
                </div>

                <!-- Rating skeleton or content -->
                <div v-if="interactionsLoading" class="flex flex-col items-center gap-2.5">
                    <div class="flex items-center gap-0.75">
                        <div
                            v-for="i in 5"
                            :key="i"
                            class="bg-border/30 h-4.5 w-4.5 animate-pulse rounded"
                        />
                    </div>
                    <div class="bg-border/30 h-3 w-6 animate-pulse rounded" />
                </div>
                <WatchDetailsRatingPanel
                    v-else-if="scene"
                    :scene-id="scene.id"
                    :initial-rating="initialRating"
                />

                <div class="bg-border/50 h-px w-full" />

                <!-- Interactions skeleton or content -->
                <div
                    v-if="interactionsLoading"
                    class="flex w-full items-center justify-center gap-3"
                >
                    <div class="flex flex-col items-center gap-0.5">
                        <div class="bg-border/30 h-4.5 w-4.5 animate-pulse rounded" />
                        <div class="bg-border/30 h-2.5 w-6 animate-pulse rounded" />
                    </div>
                    <div class="flex flex-col items-center gap-0.5">
                        <div class="bg-border/30 h-4.5 w-4.5 animate-pulse rounded" />
                        <div class="bg-border/30 h-2.5 w-6 animate-pulse rounded" />
                    </div>
                </div>
                <WatchDetailsInteractionsBar
                    v-else-if="scene"
                    :scene-id="scene.id"
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
                    :release-date="scene?.release_date || null"
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
                        v-if="scene?.porndb_scene_id"
                        :href="`https://theporndb.net/scenes/${scene.porndb_scene_id}`"
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
                        {{ scene?.porndb_scene_id ? 'Refresh' : 'Fetch' }}
                    </button>
                </div>
            </div>
        </div>

        <!-- Tags section -->
        <div class="border-border/50 bg-surface/30 rounded-xl border p-4">
            <WatchDetailsTagManager v-if="scene" ref="tagManagerRef" :scene-id="scene.id" />
        </div>

        <!-- Actors section -->
        <div class="border-border/50 bg-surface/30 rounded-xl border p-4">
            <WatchActors />
        </div>
    </div>

    <!-- Fetch Scene Metadata Modal -->
    <WatchFetchSceneMetadataModal
        v-if="scene"
        :visible="showFetchMetadataModal"
        :scene="scene"
        @close="showFetchMetadataModal = false"
        @applied="handleMetadataApplied"
    />
</template>
