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

// Clear selection when search results change or select mode toggled off
watch(
    () => searchStore.scenes,
    () => clearSelection(),
);
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

            <div
                v-if="!searchStore.isLoading && searchStore.scenes.length > 0"
                class="flex items-center gap-2"
            >
                <!-- Select All / Deselect controls -->
                <template v-if="selectMode">
                    <button
                        v-if="hasSelection"
                        class="text-dim text-xs transition-colors hover:text-white"
                        @click="clearSelection()"
                    >
                        Deselect all
                    </button>
                    <button
                        v-if="!allPageScenesSelected"
                        class="text-dim text-xs transition-colors hover:text-white"
                        @click="selectAllOnPage()"
                    >
                        Select page
                    </button>
                    <button
                        v-if="!allScenesSelected"
                        :disabled="isSelectingAll"
                        class="text-lava hover:text-lava/80 text-xs font-medium transition-colors
                            disabled:opacity-50"
                        @click="selectAllScenes()"
                    >
                        <template v-if="isSelectingAll">Selecting...</template>
                        <template v-else> Select all {{ searchStore.total }} scenes </template>
                    </button>
                </template>

                <button
                    class="flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-xs
                        font-medium transition-all"
                    :class="
                        selectMode
                            ? 'border-lava/40 bg-lava/10 text-lava'
                            : 'border-border text-dim hover:border-border-hover hover:text-white'
                    "
                    @click="selectMode = !selectMode"
                >
                    <Icon name="heroicons:check-circle" size="14" />
                    Select
                </button>
            </div>
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
