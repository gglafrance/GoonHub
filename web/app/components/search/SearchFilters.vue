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

const studioOptions = computed(() =>
    searchStore.filterOptions.studios.map((s) => ({ value: s, label: s })),
);
</script>

<template>
    <aside>
        <div class="mb-3 flex items-center justify-between">
            <h3 class="text-xs font-semibold tracking-wide text-white uppercase">Filters</h3>
            <button
                @click="showFilters = !showFilters"
                class="text-dim transition-colors hover:text-white"
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

            <!-- Actors -->
            <SearchFiltersFilterActors />

            <!-- Studio -->
            <SearchFiltersFilterSelect
                v-if="searchStore.filterOptions.studios.length > 0"
                title="Studio"
                v-model="searchStore.studio"
                :options="studioOptions"
                placeholder="All Studios"
            />

            <!-- Duration -->
            <SearchFiltersFilterDuration />

            <!-- Date Range -->
            <SearchFiltersFilterDateRange />

            <!-- Resolution -->
            <SearchFiltersFilterSelect
                title="Resolution"
                v-model="searchStore.resolution"
                :options="resolutionOptions"
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
                @click="
                    searchStore.resetFilters();
                    searchStore.search();
                "
                class="text-lava hover:text-lava/80 w-full rounded-md py-2 text-xs font-medium
                    transition-colors"
            >
                Reset All Filters
            </button>
        </div>
    </aside>
</template>
