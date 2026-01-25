<script setup lang="ts">
const searchStore = useSearchStore();
</script>

<template>
    <div>
        <!-- Result count -->
        <div class="text-dim mb-3 text-xs">
            <span v-if="searchStore.isLoading">Searching...</span>
            <span v-else-if="searchStore.total > 0">
                {{ searchStore.total }} video{{ searchStore.total !== 1 ? 's' : '' }} found
            </span>
            <span v-else-if="searchStore.hasActiveFilters">No videos match your filters</span>
            <span v-else>No videos found</span>
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
            <div class="border-lava h-6 w-6 animate-spin rounded-full border-2 border-t-transparent"></div>
        </div>

        <!-- Results Grid -->
        <template v-else-if="searchStore.videos.length > 0">
            <VideoGrid :videos="searchStore.videos" />

            <Pagination
                v-model="searchStore.page"
                :total="searchStore.total"
                :limit="searchStore.limit"
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
                @click="searchStore.resetFilters(); searchStore.search()"
                class="text-lava mt-2 text-xs hover:underline"
            >
                Clear all filters
            </button>
        </div>
    </div>
</template>
