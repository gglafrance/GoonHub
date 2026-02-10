<script setup lang="ts">
const searchStore = useSearchStore();
const api = useApi();
const { showSelector, maxLimit, updatePageSize } = usePageSize();

const selectMode = ref(false);
const scenesRef = computed(() => searchStore.scenes);
const {
    hasSelection,
    selectionCount,
    isSelectingAll,
    allPageScenesSelected,
    toggleSceneSelection,
    selectAllOnPage,
    selectAll,
    dragSelect,
    clearSelection,
    isSceneSelected,
    getSelectedSceneIDs,
} = useSceneSelection(scenesRef);

const allScenesSelected = computed(
    () => searchStore.total > 0 && selectionCount.value === searchStore.total,
);

const selectAllScenes = async () => {
    isSelectingAll.value = true;
    try {
        const ids = await api.fetchAllSearchSceneIDs(searchStore.getSearchParams());
        selectAll(ids);
    } finally {
        isSelectingAll.value = false;
    }
};

// Clear selection when search filters/sort change (but not on page change)
const filterSignature = computed(() => JSON.stringify(searchStore.getSearchParams()));
watch(filterSignature, () => clearSelection());
watch(selectMode, (on) => {
    if (!on) clearSelection();
});

const handleBulkComplete = () => {
    clearSelection();
    selectMode.value = false;
    searchStore.search();
};
</script>

<template>
    <div>
        <!-- Result count + Select toggle -->
        <div class="mb-3 flex items-center justify-between">
            <div class="text-dim text-xs">
                <span v-if="searchStore.isLoading">Searching...</span>
                <span v-else-if="searchStore.total > 0">
                    {{ searchStore.total }} scene{{ searchStore.total !== 1 ? 's' : '' }} found
                </span>
                <span v-else-if="searchStore.hasActiveFilters">No scenes match your filters</span>
                <span v-else>No scenes found</span>
            </div>

            <SceneSelectionControls
                v-if="!searchStore.isLoading && searchStore.scenes.length > 0"
                :select-mode="selectMode"
                :has-selection="hasSelection"
                :is-selecting-all="isSelectingAll"
                :all-page-scenes-selected="allPageScenesSelected"
                :all-scenes-selected="allScenesSelected"
                :total-scenes="searchStore.total"
                @update:select-mode="selectMode = $event"
                @deselect-all="clearSelection()"
                @select-page="selectAllOnPage()"
                @select-all="selectAllScenes()"
            />
        </div>

        <!-- Error -->
        <div
            v-if="searchStore.error"
            class="border-lava/30 bg-lava/5 text-lava mb-4 rounded-lg border px-4 py-3 text-xs"
        >
            {{ searchStore.error }}
        </div>

        <!-- Loading -->
        <div v-if="searchStore.isLoading" class="flex items-center justify-center py-20">
            <div
                class="border-lava h-6 w-6 animate-spin rounded-full border-2 border-t-transparent"
            ></div>
        </div>

        <!-- Results Grid -->
        <template v-else-if="searchStore.scenes.length > 0">
            <SelectableSceneGrid
                v-if="selectMode"
                :scenes="searchStore.scenes"
                :ratings="searchStore.ratings"
                :likes="searchStore.likes"
                :jizz-counts="searchStore.jizzCounts"
                :is-scene-selected="isSceneSelected"
                @toggle-selection="toggleSceneSelection"
                @drag-select="(ids, additive) => dragSelect(ids, additive)"
            />
            <SceneGrid
                v-else
                :scenes="searchStore.scenes"
                :ratings="searchStore.ratings"
                :likes="searchStore.likes"
                :jizz-counts="searchStore.jizzCounts"
            />

            <Pagination
                v-model="searchStore.page"
                :total="searchStore.total"
                :limit="searchStore.limit"
                :show-page-size-selector="showSelector"
                :max-limit="maxLimit"
                @update:limit="
                    (v: number) => {
                        updatePageSize(v);
                        if (searchStore.page === 1) searchStore.search();
                        else searchStore.page = 1;
                    }
                "
            />
        </template>

        <!-- Empty state -->
        <div
            v-else-if="!searchStore.isLoading && searchStore.hasActiveFilters"
            class="flex flex-col items-center justify-center py-20"
        >
            <Icon name="heroicons:magnifying-glass" size="40" class="text-dim mb-3 opacity-30" />
            <p class="text-dim text-sm">No results found</p>
            <button
                class="text-lava mt-2 text-xs hover:underline"
                @click="
                    searchStore.resetFilters();
                    searchStore.search();
                "
            >
                Clear all filters
            </button>
        </div>

        <!-- Bulk Toolbar -->
        <BulkToolbar
            v-if="hasSelection"
            :scene-ids="getSelectedSceneIDs()"
            :selection-count="selectionCount"
            @clear-selection="clearSelection()"
            @complete="handleBulkComplete"
        />
    </div>
</template>
