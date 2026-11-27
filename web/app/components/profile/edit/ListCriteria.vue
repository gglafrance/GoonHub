<script setup lang="ts">
import type { SavedSearchFilters } from '~/types/saved_search';

// Criteria of a custom list (modal): the search page interface (search bar with the sorts,
// filters, active filter chips) on its own search store, with a preview of the first results
// (run as yourself: what the list will show). "Use these criteria" returns the filters.
const props = defineProps<{ filters: SavedSearchFilters }>();
const emit = defineEmits<{ close: []; apply: [filters: SavedSearchFilters] }>();

const store = useProfileListSearchStore();
provideSearchStore(store);

const PREVIEW = 12;
let timer: ReturnType<typeof setTimeout> | null = null;
const searchNow = () => {
    if (timer) clearTimeout(timer);
    store.page = 1;
    store.search();
};
watch(
    () => store.query,
    () => {
        if (timer) clearTimeout(timer);
        timer = setTimeout(searchNow, 300);
    },
);
watch(() => {
    const params = { ...store.getSearchParams() };
    delete params.q;
    return JSON.stringify(params);
}, searchNow);

const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') emit('close');
};
onMounted(() => {
    store.loadFilters(props.filters);
    if (store.filterOptions.tags.length === 0) store.loadFilterOptions();
    if (store.sourceSites.length === 0) store.loadSourceSites();
    store.search();
    window.addEventListener('keydown', onKey);
});
onUnmounted(() => {
    if (timer) clearTimeout(timer);
    window.removeEventListener('keydown', onKey);
});
</script>

<template>
    <div
        class="fixed inset-0 z-[60] flex items-stretch justify-center bg-black/70 backdrop-blur-sm
            sm:p-6"
        @click.self="emit('close')"
    >
        <div
            class="glass-panel flex h-full w-full max-w-6xl flex-col overflow-hidden"
            role="dialog"
            aria-modal="true"
            aria-label="Criteria of the list"
        >
            <div class="border-border flex items-center gap-2 border-b px-4 py-3">
                <Icon name="heroicons:funnel" size="15" class="text-lava" />
                <h3 class="text-sm font-semibold text-white">Criteria of the list</h3>
                <span class="text-dim text-[11px]">
                    Your ratings, likes, O and views are used; the sort orders the list.
                </span>
                <button
                    class="text-dim ml-auto hover:text-white"
                    title="Close"
                    @click="emit('close')"
                >
                    <Icon name="heroicons:x-mark" size="16" />
                </button>
            </div>
            <div class="px-4 pt-3">
                <SearchBar sort-on-mobile @submit="searchNow" />
            </div>
            <SearchActiveFilters class="px-4 pt-2" />
            <div class="flex min-h-0 flex-1 gap-5 px-4 pt-3">
                <div class="hidden min-h-0 w-56 shrink-0 overflow-y-auto pb-4 lg:block">
                    <SearchFilters />
                </div>
                <div class="min-h-0 min-w-0 flex-1 overflow-y-auto pb-4">
                    <div class="text-dim mb-3 text-xs">
                        <span v-if="store.isLoading">Searching...</span>
                        <span v-else>
                            {{ store.total }} video{{ store.total !== 1 ? 's' : '' }} match; the
                            first ones:
                        </span>
                    </div>
                    <div v-if="store.isLoading" class="flex justify-center py-12">
                        <LoadingSpinner />
                    </div>
                    <div
                        v-else
                        class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6"
                    >
                        <SceneCard
                            v-for="scene in store.scenes.slice(0, PREVIEW)"
                            :key="scene.id"
                            :scene="scene"
                            fluid
                            preview
                        />
                    </div>
                </div>
            </div>
            <div class="border-border flex justify-end gap-2 border-t px-4 py-3">
                <button class="text-dim text-xs hover:text-white" @click="store.resetFilters()">
                    Clear
                </button>
                <button class="text-dim text-xs hover:text-white" @click="emit('close')">
                    Cancel
                </button>
                <button
                    class="bg-lava hover:bg-lava/90 rounded-md px-3 py-1.5 text-xs font-medium
                        text-white"
                    @click="emit('apply', store.getCurrentFilters())"
                >
                    Use these criteria
                </button>
            </div>
        </div>
    </div>
</template>
