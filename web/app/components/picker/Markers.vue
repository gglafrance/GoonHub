<script setup lang="ts">
import type { MarkerWithScene } from '~/types/marker';

// Markers tab of the search picker: the "All" listing of the markers page (markers the user
// sees: their own and the shared ones) with its filters (labels, marker tags, scene tags,
// actors, studio, with an end only) and sorts, plus a text search on the label or the scene
// title. Only markers of scenes still in the library are listed (`live_only`), which is also
// the rule of the profile showcase. Local state: nothing goes to the URL.
defineProps<{
    isSelected: (marker: MarkerWithScene) => boolean;
    markedLabel: (marker: MarkerWithScene) => string | undefined;
    isDisabled: (marker: MarkerWithScene) => boolean;
}>();

const emit = defineEmits<{ toggle: [marker: MarkerWithScene] }>();

const { fetchAllMarkers } = useApiMarkers();
const { fetchProcessingConfig } = useApiJobs();
const { limit, showSelector, maxLimit, updatePageSize } = usePageSize();
const { sceneGridClass, sceneGridStyle } = useGridDensity();
const { observe } = useAnimatedMarkerPreview();

const sortOptions = [
    { value: 'recent', label: 'Recent' },
    { value: 'oldest', label: 'Oldest' },
    { value: 'label_asc', label: 'A-Z' },
    { value: 'label_desc', label: 'Z-A' },
    { value: 'random', label: 'Random' },
];
const sortBy = ref('recent');
const query = ref('');
const page = ref(1);
const markers = ref<MarkerWithScene[]>([]);
const total = ref(0);
const loading = ref(false);
const error = ref('');
const animated = ref(false);
const showFilters = ref(false);
const input = ref<HTMLInputElement | null>(null);
defineExpose({ focus: () => input.value?.focus() });

const {
    state: filterState,
    params: filterParams,
    activeCount: filterCount,
    reset: resetFilters,
    options: filterOptions,
    loadOptions: loadFilterOptions,
    seed,
    reshuffle,
} = useMarkerFilters(sortBy, true, { url: false, alwaysEnabled: true });

let request = 0;
const load = async () => {
    const current = ++request;
    loading.value = true;
    error.value = '';
    try {
        const extra: Record<string, string> = { ...filterParams.value, live_only: 'true' };
        const q = query.value.trim();
        if (q) extra.q = q;
        const res = await fetchAllMarkers(page.value, limit.value, sortBy.value, extra);
        if (current !== request) return;
        markers.value = res.data;
        total.value = res.pagination.total_items;
    } catch (e: unknown) {
        if (current === request)
            error.value = e instanceof Error ? e.message : 'Failed to load markers';
    } finally {
        if (current === request) loading.value = false;
    }
};

let timer: ReturnType<typeof setTimeout> | null = null;
const loadFirstPage = () => {
    if (timer) clearTimeout(timer);
    if (page.value !== 1) page.value = 1;
    else load();
};
watch(query, () => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(loadFirstPage, 300);
});
watch([sortBy, () => JSON.stringify(filterParams.value), limit], loadFirstPage);
watch(page, load);

const toggleFilters = () => {
    showFilters.value = !showFilters.value;
    if (showFilters.value) loadFilterOptions();
};

onMounted(async () => {
    load();
    try {
        const config = await fetchProcessingConfig();
        animated.value = config.marker_thumbnail_type === 'animated';
    } catch {
        // Static thumbnails
    }
});
onBeforeUnmount(() => {
    if (timer) clearTimeout(timer);
});
</script>

<template>
    <div class="flex h-full min-h-0 flex-col">
        <div class="flex gap-2 px-4 pt-3 sm:px-5">
            <div class="relative min-w-0 flex-1">
                <Icon
                    name="heroicons:magnifying-glass"
                    size="16"
                    class="text-dim absolute top-1/2 left-3 -translate-y-1/2"
                />
                <input
                    ref="input"
                    v-model="query"
                    type="text"
                    placeholder="Search markers by label or video..."
                    enterkeyhint="search"
                    class="border-border bg-surface placeholder:text-dim h-10 w-full rounded-lg
                        border py-2 pr-3 pl-9 text-sm text-white transition-colors
                        focus:border-white/20 focus:outline-none"
                    @keydown.enter="loadFirstPage"
                />
            </div>
            <MarkersFiltersToggle
                :open="showFilters"
                :active-count="filterCount"
                @toggle="toggleFilters"
            />
            <UiSortSelect v-model="sortBy" :options="sortOptions" />
            <MarkersRandomSeed v-if="sortBy === 'random'" :seed="seed" @reshuffle="reshuffle" />
        </div>

        <div class="min-h-0 flex-1 overflow-y-auto px-4 pt-3 pb-4 sm:px-5">
            <MarkersFiltersPanel
                v-if="showFilters"
                class="mb-3"
                :state="filterState"
                :options="filterOptions"
                :active-count="filterCount"
                with-labels
                @reset="resetFilters"
            />

            <div class="text-dim mb-3 text-xs">
                <span v-if="loading && markers.length === 0">Searching...</span>
                <span v-else>{{ total }} marker{{ total !== 1 ? 's' : '' }}</span>
            </div>

            <ErrorAlert v-if="error" :message="error" class="mb-3" />

            <div v-if="loading && markers.length === 0" class="flex justify-center py-16">
                <LoadingSpinner />
            </div>
            <div
                v-else-if="markers.length === 0"
                class="flex flex-col items-center justify-center py-16"
            >
                <Icon name="heroicons:bookmark" size="36" class="text-dim mb-3 opacity-30" />
                <p class="text-dim text-sm">No markers found</p>
            </div>
            <div v-else :class="loading ? 'opacity-60' : ''">
                <Pagination
                    v-model="page"
                    position="top"
                    :total="total"
                    :limit="limit"
                    :show-page-size-selector="showSelector"
                    :max-limit="maxLimit"
                    @update:limit="updatePageSize"
                />
                <div
                    class="scene-grid grid justify-center"
                    :class="sceneGridClass"
                    :style="sceneGridStyle"
                >
                    <MarkersMarkerCard
                        v-for="marker in markers"
                        :key="marker.id"
                        :marker="marker"
                        :animated="animated"
                        :observe="observe"
                        selectable
                        :selected="isSelected(marker)"
                        :marked="markedLabel(marker)"
                        :class="isDisabled(marker) ? 'opacity-50' : ''"
                        @select="emit('toggle', marker)"
                    />
                </div>
                <Pagination
                    v-model="page"
                    :total="total"
                    :limit="limit"
                    :show-page-size-selector="showSelector"
                    :max-limit="maxLimit"
                    @update:limit="updatePageSize"
                />
            </div>
        </div>
    </div>
</template>
