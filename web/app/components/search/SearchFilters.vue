<script setup lang="ts">
const searchStore = useSearchStore();

const showFilters = ref(true);

const resolutionOptions = [
    { value: '', label: 'Any' },
    { value: '4k', label: '4K' },
    { value: '1440p', label: '1440p' },
    { value: '1080p', label: '1080p' },
    { value: '720p', label: '720p' },
    { value: '480p', label: '480p' },
    { value: '360p', label: '360p' },
];
</script>

<template>
    <aside>
        <div class="mb-3 flex items-center justify-between">
            <h3 class="text-xs font-semibold tracking-wide text-white uppercase">Filters</h3>
            <button
                class="text-dim transition-colors hover:text-white"
                @click="showFilters = !showFilters"
            >
                <Icon
                    :name="showFilters ? 'heroicons:chevron-up' : 'heroicons:chevron-down'"
                    size="14"
                />
            </button>
        </div>

        <div v-show="showFilters" class="space-y-1">
            <!-- Match Type -->
            <SearchFiltersFilterMatchType />

            <!-- Tags -->
            <SearchFiltersFilterTags />

            <!-- Marker Labels -->
            <SearchFiltersFilterMarkerLabels />

            <!-- Actors -->
            <SearchFiltersFilterActors />

            <!-- Studio -->
            <SearchFiltersFilterStudio />

            <!-- Duration -->
            <SearchFiltersFilterDuration />

            <!-- Date Range -->
            <SearchFiltersFilterDateRange />

            <!-- Resolution -->
            <SearchFiltersFilterSelect
                v-model="searchStore.resolution"
                title="Resolution"
                icon="heroicons:tv"
                :options="resolutionOptions"
                default-collapsed
            />

            <!-- Liked -->
            <SearchFiltersFilterLiked />

            <!-- Rating -->
            <SearchFiltersFilterRatingRange />

            <!-- Jizz Count -->
            <SearchFiltersFilterJizzRange />

            <!-- Reset -->
            <button
                v-if="searchStore.hasActiveFilters"
                class="text-lava hover:text-lava/80 w-full rounded-md py-2 text-xs font-medium
                    transition-colors"
                @click="
                    searchStore.resetFilters();
                    searchStore.search();
                "
            >
                Reset All Filters
            </button>
        </div>
    </aside>
</template>
