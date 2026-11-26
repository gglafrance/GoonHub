<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';
import type SearchBar from '~/components/search/SearchBar.vue';

// Videos tab of the search picker: the search page interface (search bar, sort, filters,
// active filter chips, result grid with the user's card template, pages or infinite scroll)
// on its own search store instance, so the search page state and the URL stay untouched.
defineProps<{
    isSelected: (scene: SceneListItem) => boolean;
    /** Badge text of an already picked scene, or undefined */
    markedLabel: (scene: SceneListItem) => string | undefined;
    /** The scene cannot be picked (already picked, or the selection is full) */
    isDisabled: (scene: SceneListItem) => boolean;
}>();

const emit = defineEmits<{ toggle: [scene: SceneListItem] }>();

const store = usePickerSearchStore();
provideSearchStore(store);

const settingsStore = useSettingsStore();
const { showSelector, maxLimit, updatePageSize } = usePageSize();
const { sceneGridClass, sceneGridStyle } = useGridDensity();

const showFilters = ref(false);
const bar = ref<InstanceType<typeof SearchBar> | null>(null);
defineExpose({ focus: () => bar.value?.focus() });

const infiniteScroll = computed(() => settingsStore.infiniteScroll);
const hasMore = computed(() => store.loadedPages.last * store.limit < store.total);
const loadMore = () => {
    if (store.isLoading || store.isLoadingMore) return;
    store.page = store.loadedPages.last + 1;
};

// Searches like the search page does, without the URL: typing is debounced, a filter or sort
// change goes back to the first page, a page change loads it (appended with infinite scroll)
let timer: ReturnType<typeof setTimeout> | null = null;
const searchFromFirstPage = () => {
    if (timer) clearTimeout(timer);
    if (store.page !== 1) store.page = 1;
    else store.search();
};
const submit = () => searchFromFirstPage();

watch(
    () => store.query,
    () => {
        if (timer) clearTimeout(timer);
        timer = setTimeout(searchFromFirstPage, 300);
    },
);
// Every parameter but the text query (debounced above)
watch(() => {
    const params = { ...store.getSearchParams() };
    delete params.q;
    return JSON.stringify(params);
}, searchFromFirstPage);
watch(
    () => store.page,
    () => store.search(),
);
watch(infiniteScroll, () => store.search());

const changeLimit = (v: number) => {
    updatePageSize(v);
    searchFromFirstPage();
};

onMounted(() => {
    if (store.filterOptions.tags.length === 0) store.loadFilterOptions();
    if (store.sourceSites.length === 0) store.loadSourceSites();
    store.search();
});
onBeforeUnmount(() => {
    if (timer) clearTimeout(timer);
});
</script>

<template>
    <div class="flex h-full min-h-0 flex-col">
        <div class="flex items-center gap-2 px-4 pt-3 sm:px-5">
            <div class="min-w-0 flex-1">
                <SearchBar ref="bar" sort-on-mobile @submit="submit" />
            </div>
            <button
                class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10 flex h-10
                    shrink-0 items-center gap-1.5 rounded-lg border px-3 text-xs font-medium
                    text-white transition-all lg:hidden"
                :class="showFilters ? 'border-lava/40 bg-lava/10' : ''"
                @click="showFilters = !showFilters"
            >
                <Icon name="heroicons:adjustments-horizontal" size="15" />
                {{ showFilters ? 'Results' : 'Filters' }}
            </button>
        </div>
        <SearchActiveFilters class="px-4 pt-2 sm:px-5" />

        <div class="flex min-h-0 flex-1 gap-5 px-4 pt-3 sm:px-5">
            <div
                class="min-h-0 shrink-0 overflow-y-auto pb-4 lg:block lg:w-56"
                :class="showFilters ? 'block w-full' : 'hidden'"
            >
                <SearchFilters />
            </div>

            <div
                class="min-h-0 min-w-0 flex-1 overflow-y-auto pb-4"
                :class="showFilters ? 'hidden lg:block' : ''"
            >
                <div class="text-dim mb-3 text-xs">
                    <span v-if="store.isLoading">Searching...</span>
                    <span v-else-if="store.total > 0">
                        {{ store.total }} scene{{ store.total !== 1 ? 's' : '' }} found
                    </span>
                    <span v-else>No scenes match your filters</span>
                </div>

                <ErrorAlert v-if="store.error" :message="store.error" class="mb-3" />

                <div v-if="store.isLoading" class="flex justify-center py-16">
                    <LoadingSpinner />
                </div>
                <template v-else-if="store.scenes.length > 0">
                    <Pagination
                        v-if="!infiniteScroll"
                        v-model="store.page"
                        position="top"
                        :total="store.total"
                        :limit="store.limit"
                        :show-page-size-selector="showSelector"
                        :max-limit="maxLimit"
                        @update:limit="changeLimit"
                    />
                    <div
                        class="scene-grid grid justify-center"
                        :class="sceneGridClass"
                        :style="sceneGridStyle"
                    >
                        <div
                            v-for="scene in store.scenes"
                            :key="scene.id"
                            class="relative"
                            :class="isDisabled(scene) ? 'opacity-50' : ''"
                        >
                            <SceneCard
                                :scene="scene"
                                :rating="store.ratings[scene.id]"
                                :liked="store.likes[scene.id]"
                                :jizz-count="store.jizzCounts[scene.id]"
                                :completed="store.watched[scene.id]"
                                fluid
                                selectable
                                :selected="isSelected(scene)"
                                @toggle-selection="emit('toggle', scene)"
                            />
                            <span
                                v-if="markedLabel(scene)"
                                class="bg-emerald/20 text-emerald pointer-events-none absolute top-2
                                    left-9 z-30 flex items-center gap-1 rounded px-1.5 py-0.5
                                    text-[10px] font-semibold backdrop-blur-sm"
                            >
                                <Icon name="heroicons:check-circle" size="11" />
                                {{ markedLabel(scene) }}
                            </span>
                        </div>
                    </div>
                    <SceneListInfiniteLoader
                        v-if="infiniteScroll"
                        :has-more="hasMore"
                        :loading="store.isLoadingMore"
                        :total="store.total"
                        @load-more="loadMore"
                    />
                    <Pagination
                        v-else
                        v-model="store.page"
                        :total="store.total"
                        :limit="store.limit"
                        :show-page-size-selector="showSelector"
                        :max-limit="maxLimit"
                        @update:limit="changeLimit"
                    />
                </template>
                <div v-else class="flex flex-col items-center justify-center py-16">
                    <Icon
                        name="heroicons:magnifying-glass"
                        size="36"
                        class="text-dim mb-3 opacity-30"
                    />
                    <p class="text-dim text-sm">No results found</p>
                    <button
                        v-if="store.hasActiveFilters"
                        class="text-lava mt-2 text-xs hover:underline"
                        @click="store.resetFilters()"
                    >
                        Clear all filters
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>
