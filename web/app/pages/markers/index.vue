<script setup lang="ts">
import type { MarkerLabelGroup, MarkerWithScene } from '~/types/marker';

useHead({ title: 'Markers' });

useSeoMeta({
    title: 'Markers',
    ogTitle: 'Markers - GoonHub',
    description: 'Browse scene markers and bookmarks',
    ogDescription: 'Browse scene markers and bookmarks',
});

const { fetchLabelGroups, fetchAllMarkers } = useApiMarkers();
const { fetchProcessingConfig } = useApiJobs();
const { formatDuration } = useFormatter();
const { observe } = useAnimatedMarkerPreview();
const settingsStore = useSettingsStore();

// Marker thumbnail type (static or animated)
const markerThumbnailType = ref('static');

type ViewMode = 'grouped' | 'all';
const viewMode = ref<ViewMode>('grouped');

// Grouped mode state
const groups = ref<MarkerLabelGroup[]>([]);
const groupTotal = ref(0);

// All mode state
const allMarkers = ref<MarkerWithScene[]>([]);
const allTotal = ref(0);

const currentPage = useUrlPagination();
const { limit, showSelector, maxLimit, updatePageSize } = usePageSize();
const searchQuery = ref('');
const sortBy = useUrlSort(settingsStore.sortPreferences?.markers || 'label_asc');
const isLoading = ref(false);
const error = ref<string | null>(null);

const groupedSortOptions = [
    { value: 'label_asc', label: 'A-Z' },
    { value: 'label_desc', label: 'Z-A' },
    { value: 'count_desc', label: 'Most markers' },
    { value: 'count_asc', label: 'Fewest markers' },
    { value: 'recent', label: 'Recently added' },
];

const allSortOptions = [
    { value: 'label_asc', label: 'A-Z' },
    { value: 'label_desc', label: 'Z-A' },
    { value: 'recent', label: 'Recent' },
    { value: 'oldest', label: 'Oldest' },
];

const sortOptions = computed(() =>
    viewMode.value === 'grouped' ? groupedSortOptions : allSortOptions,
);

const total = computed(() => (viewMode.value === 'grouped' ? groupTotal.value : allTotal.value));
const totalLabel = computed(() =>
    viewMode.value === 'grouped' ? `${total.value} labels` : `${total.value} markers`,
);

let searchTimeout: ReturnType<typeof setTimeout> | null = null;

// Filter groups by search query (client-side, grouped mode only)
const filteredGroups = computed(() => {
    if (!searchQuery.value.trim()) {
        return groups.value;
    }
    const query = searchQuery.value.toLowerCase().trim();
    return groups.value.filter((g) => g.label.toLowerCase().includes(query));
});

const loadGroups = async (page = 1) => {
    isLoading.value = true;
    error.value = null;
    try {
        const response = await fetchLabelGroups(page, limit.value, sortBy.value);
        groups.value = response.data;
        groupTotal.value = response.pagination.total_items;
        currentPage.value = page;
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load marker labels';
    } finally {
        isLoading.value = false;
    }
};

const loadAllMarkers = async (page = 1) => {
    isLoading.value = true;
    error.value = null;
    try {
        const response = await fetchAllMarkers(page, limit.value, sortBy.value);
        allMarkers.value = response.data;
        allTotal.value = response.pagination.total_items;
        currentPage.value = page;
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load markers';
    } finally {
        isLoading.value = false;
    }
};

const loadData = (page = 1) => {
    if (viewMode.value === 'grouped') {
        loadGroups(page);
    } else {
        loadAllMarkers(page);
    }
};

onMounted(async () => {
    loadData(currentPage.value);
    try {
        const config = await fetchProcessingConfig();
        markerThumbnailType.value = config.marker_thumbnail_type || 'static';
    } catch {
        // Default to static
    }
});

watch(
    () => currentPage.value,
    (newPage) => {
        loadData(newPage);
    },
);

watch(sortBy, () => {
    loadData(1);
});

watch(viewMode, () => {
    // Reset sort to a valid default for the new mode
    const validSorts = sortOptions.value.map((o) => o.value);
    if (!validSorts.includes(sortBy.value)) {
        sortBy.value = 'label_asc';
    }
    searchQuery.value = '';
    loadData(1);
});

// Debounce search to avoid flickering
watch(searchQuery, () => {
    if (searchTimeout) {
        clearTimeout(searchTimeout);
    }
    searchTimeout = setTimeout(() => {
        // Client-side filtering, no need to reload
    }, 100);
});

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Header -->
            <div class="mb-6">
                <div class="flex items-center justify-between">
                    <h1 class="text-lg font-semibold text-white">Markers</h1>
                    <span
                        class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                            font-mono text-[11px]"
                    >
                        {{ totalLabel }}
                    </span>
                </div>

                <!-- View mode toggle, search bar, and sort -->
                <div class="mt-4 flex gap-3">
                    <!-- View mode toggle -->
                    <div
                        class="border-border bg-panel flex shrink-0 items-center rounded-lg border
                            p-0.5"
                    >
                        <button
                            class="rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-all"
                            :class="
                                viewMode === 'grouped'
                                    ? 'bg-lava/15 text-lava'
                                    : 'text-dim hover:text-white'
                            "
                            @click="viewMode = 'grouped'"
                        >
                            Grouped
                        </button>
                        <button
                            class="rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-all"
                            :class="
                                viewMode === 'all'
                                    ? 'bg-lava/15 text-lava'
                                    : 'text-dim hover:text-white'
                            "
                            @click="viewMode = 'all'"
                        >
                            All
                        </button>
                    </div>

                    <div v-if="viewMode === 'grouped'" class="relative flex-1">
                        <Icon
                            name="heroicons:magnifying-glass"
                            size="16"
                            class="text-dim absolute top-1/2 left-3 -translate-y-1/2"
                        />
                        <input
                            v-model="searchQuery"
                            type="text"
                            placeholder="Filter labels..."
                            class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20
                                w-full rounded-lg border py-2 pr-3 pl-9 text-sm text-white
                                placeholder-white/40 transition-all focus:ring-2 focus:outline-none"
                        />
                    </div>
                    <div v-else class="flex-1" />

                    <UiSortSelect v-model="sortBy" :options="sortOptions" />
                </div>
            </div>

            <!-- Error -->
            <ErrorAlert v-if="error" :message="error" class="mb-4" />

            <!-- Loading State -->
            <div
                v-if="isLoading && groups.length === 0 && allMarkers.length === 0"
                class="flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading markers..." />
            </div>

            <!-- ===== GROUPED VIEW ===== -->
            <template v-else-if="viewMode === 'grouped'">
                <!-- Empty State -->
                <div
                    v-if="groups.length === 0"
                    class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                        border border-dashed text-center"
                >
                    <div
                        class="bg-panel border-border flex h-10 w-10 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:bookmark" size="20" class="text-dim" />
                    </div>
                    <p class="text-muted mt-3 text-sm">No marker labels found</p>
                    <p class="text-dim mt-1 text-xs">Create markers on scenes to see them here</p>
                </div>

                <!-- No search results -->
                <div
                    v-else-if="filteredGroups.length === 0"
                    class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                        border border-dashed text-center"
                >
                    <div
                        class="bg-panel border-border flex h-10 w-10 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:magnifying-glass" size="20" class="text-dim" />
                    </div>
                    <p class="text-muted mt-3 text-sm">No labels match your search</p>
                    <p class="text-dim mt-1 text-xs">Try a different search term</p>
                </div>

                <!-- Label Grid -->
                <div v-else>
                    <MarkerLabelGrid
                        :groups="filteredGroups"
                        :marker-thumbnail-type="markerThumbnailType"
                    />
                    <Pagination
                        v-model="currentPage"
                        :total="groupTotal"
                        :limit="limit"
                        :show-page-size-selector="showSelector"
                        :max-limit="maxLimit"
                        @update:limit="(v: number) => { updatePageSize(v); if (currentPage === 1) loadData(1); else currentPage = 1; }"
                    />
                </div>
            </template>

            <!-- ===== ALL VIEW ===== -->
            <template v-else>
                <!-- Empty State -->
                <div
                    v-if="allMarkers.length === 0 && !isLoading"
                    class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                        border border-dashed text-center"
                >
                    <div
                        class="bg-panel border-border flex h-10 w-10 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:bookmark" size="20" class="text-dim" />
                    </div>
                    <p class="text-muted mt-3 text-sm">No markers found</p>
                    <p class="text-dim mt-1 text-xs">Create markers on scenes to see them here</p>
                </div>

                <!-- All markers grid -->
                <div v-else>
                    <div
                        class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5"
                    >
                        <NuxtLink
                            v-for="marker in allMarkers"
                            :key="marker.id"
                            :to="`/watch/${marker.scene_id}?t=${marker.timestamp}`"
                            class="group relative block max-w-[320px] overflow-hidden rounded-lg
                                transition-transform duration-200"
                        >
                            <!-- Thumbnail -->
                            <div class="relative aspect-video w-full overflow-hidden bg-black/40">
                                <video
                                    v-if="
                                        markerThumbnailType === 'animated' &&
                                        marker.animated_thumbnail_path
                                    "
                                    :ref="(el) => observe(el as HTMLVideoElement)"
                                    :src="`/marker-thumbnails/${marker.id}/animated`"
                                    muted
                                    loop
                                    playsinline
                                    preload="none"
                                    class="h-full w-full object-contain transition-transform
                                        duration-300 group-hover:scale-105"
                                />
                                <img
                                    v-else
                                    :src="`/marker-thumbnails/${marker.id}`"
                                    :alt="marker.scene_title"
                                    class="h-full w-full object-cover transition-transform
                                        duration-300 group-hover:scale-105"
                                    loading="lazy"
                                />

                                <!-- Gradient overlay -->
                                <div
                                    class="pointer-events-none absolute inset-0 bg-linear-to-t
                                        from-black/80 via-black/20 to-transparent"
                                />

                                <!-- Timestamp badge -->
                                <div
                                    class="absolute right-1.5 bottom-1.5 rounded bg-black/80 px-1.5
                                        py-0.5 text-[10px] font-semibold text-white/90 tabular-nums
                                        backdrop-blur-sm"
                                >
                                    {{ formatDuration(marker.timestamp) }}
                                </div>

                                <!-- Label badge -->
                                <div
                                    v-if="marker.label"
                                    class="absolute top-1.5 left-1.5 max-w-[80%] truncate rounded
                                        bg-black/70 px-1.5 py-0.5 text-[10px] font-medium
                                        text-white/90 backdrop-blur-sm"
                                >
                                    {{ marker.label }}
                                </div>
                            </div>

                            <!-- Info -->
                            <div class="border-border bg-surface border-t px-2 py-1.5">
                                <p
                                    class="truncate text-xs font-medium text-white
                                        group-hover:text-white"
                                >
                                    {{ marker.scene_title }}
                                </p>
                                <NuxtTime
                                    :datetime="marker.created_at"
                                    class="text-dim mt-0.5 text-[10px]"
                                    relative
                                />
                            </div>
                        </NuxtLink>
                    </div>

                    <Pagination
                        v-model="currentPage"
                        :total="allTotal"
                        :limit="limit"
                        :show-page-size-selector="showSelector"
                        :max-limit="maxLimit"
                        @update:limit="(v: number) => { updatePageSize(v); if (currentPage === 1) loadData(1); else currentPage = 1; }"
                    />
                </div>
            </template>
        </div>
    </div>
</template>
