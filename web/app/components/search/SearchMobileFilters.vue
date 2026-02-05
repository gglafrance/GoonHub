<script setup lang="ts">
const searchStore = useSearchStore();

const props = defineProps<{
    visible: boolean;
}>();

const emit = defineEmits<{
    close: [];
}>();

const activeFilterCount = computed(() => {
    let count = 0;
    if (searchStore.selectedTags.length > 0) count += searchStore.selectedTags.length;
    if (searchStore.selectedActors.length > 0) count += searchStore.selectedActors.length;
    if (searchStore.selectedMarkerLabels.length > 0)
        count += searchStore.selectedMarkerLabels.length;
    if (searchStore.studio) count++;
    if (searchStore.minDuration > 0 || searchStore.maxDuration > 0) count++;
    if (searchStore.minDate || searchStore.maxDate) count++;
    if (searchStore.resolution) count++;
    if (searchStore.liked) count++;
    if (searchStore.minRating > 0 || searchStore.maxRating > 0) count++;
    if (searchStore.minJizzCount > 0 || searchStore.maxJizzCount > 0) count++;
    if (searchStore.matchType !== 'broad') count++;
    return count;
});

const resolutionOptions = [
    { value: '', label: 'Any' },
    { value: '4k', label: '4K' },
    { value: '1440p', label: '1440p' },
    { value: '1080p', label: '1080p' },
    { value: '720p', label: '720p' },
    { value: '480p', label: '480p' },
    { value: '360p', label: '360p' },
];

const sortOptions = [
    { value: '', label: 'Default' },
    { value: 'relevance', label: 'Relevance' },
    { value: 'random', label: 'Random' },
    { value: 'created_at_desc', label: 'Newest' },
    { value: 'created_at_asc', label: 'Oldest' },
    { value: 'title_asc', label: 'Title A-Z' },
    { value: 'title_desc', label: 'Title Z-A' },
    { value: 'duration_asc', label: 'Shortest' },
    { value: 'duration_desc', label: 'Longest' },
    { value: 'view_count_desc', label: 'Most Viewed' },
    { value: 'view_count_asc', label: 'Least Viewed' },
];

const handleReset = () => {
    searchStore.resetFilters();
    searchStore.search();
};

const handleApply = () => {
    emit('close');
};

// Prevent body scroll when drawer is open
watch(
    () => props.visible,
    (isVisible) => {
        if (isVisible) {
            document.body.style.overflow = 'hidden';
        } else {
            document.body.style.overflow = '';
        }
    },
);

onUnmounted(() => {
    document.body.style.overflow = '';
});
</script>

<template>
    <Teleport to="body">
        <!-- Backdrop -->
        <Transition name="fade">
            <div
                v-if="visible"
                class="fixed inset-0 z-50 bg-black/60 backdrop-blur-sm"
                @click="emit('close')"
            />
        </Transition>

        <!-- Drawer -->
        <Transition name="slide-up">
            <div
                v-if="visible"
                class="bg-surface fixed inset-x-0 bottom-0 z-50 flex max-h-[85vh] flex-col
                    rounded-t-2xl"
            >
                <!-- Handle bar -->
                <div class="flex justify-center py-3">
                    <div class="h-1 w-10 rounded-full bg-white/20" />
                </div>

                <!-- Header -->
                <div class="border-border flex items-center justify-between border-b px-4 pb-3">
                    <div class="flex items-center gap-2">
                        <h2 class="text-sm font-semibold text-white">Filters</h2>
                        <span
                            v-if="activeFilterCount > 0"
                            class="bg-lava flex h-5 min-w-5 items-center justify-center rounded-full
                                px-1.5 text-[10px] font-bold text-white"
                        >
                            {{ activeFilterCount }}
                        </span>
                    </div>
                    <button
                        class="text-dim flex h-8 w-8 items-center justify-center rounded-full
                            transition-colors hover:bg-white/10 hover:text-white"
                        @click="emit('close')"
                    >
                        <Icon name="heroicons:x-mark" size="20" />
                    </button>
                </div>

                <!-- Scrollable filter content -->
                <div class="flex-1 overflow-y-auto px-4 py-4">
                    <div class="space-y-1">
                        <!-- Sort (mobile only) -->
                        <SearchFiltersFilterSelect
                            v-model="searchStore.sort"
                            title="Sort By"
                            icon="heroicons:arrows-up-down"
                            :options="sortOptions"
                        />

                        <!-- Reshuffle button (random sort) -->
                        <button
                            v-if="searchStore.sort === 'random'"
                            class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10
                                flex w-full items-center justify-center gap-2 rounded-lg border px-3
                                py-2.5 text-xs font-medium text-white transition-all"
                            @click="searchStore.reshuffle()"
                        >
                            <Icon name="heroicons:arrow-path" size="14" />
                            Reshuffle
                        </button>

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
                    </div>
                </div>

                <!-- Footer actions -->
                <div
                    class="border-border flex gap-3 border-t px-4 py-4"
                    style="padding-bottom: max(1rem, env(safe-area-inset-bottom))"
                >
                    <button
                        v-if="activeFilterCount > 0"
                        class="border-border text-dim flex-1 rounded-xl border py-3 text-sm
                            font-medium transition-colors hover:border-white/20 hover:text-white"
                        @click="handleReset"
                    >
                        Reset
                    </button>
                    <button
                        class="bg-lava hover:bg-lava-glow flex-1 rounded-xl py-3 text-sm
                            font-semibold text-white transition-colors"
                        :class="{ 'flex-2': activeFilterCount > 0 }"
                        @click="handleApply"
                    >
                        Show Results
                    </button>
                </div>
            </div>
        </Transition>
    </Teleport>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
    transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
    opacity: 0;
}

.slide-up-enter-active,
.slide-up-leave-active {
    transition: transform 0.3s cubic-bezier(0.32, 0.72, 0, 1);
}

.slide-up-enter-from,
.slide-up-leave-to {
    transform: translateY(100%);
}
</style>
