<script setup lang="ts">
import type { SavedSearch, SavedSearchFilters } from '~/types/saved_search';

const searchStore = useSearchStore();
const route = useRoute();
const router = useRouter();

useHead({ title: 'Search' });

useSeoMeta({
    title: 'Search',
    ogTitle: 'Search - GoonHub',
    description: 'Search your video library',
    ogDescription: 'Search your video library',
});

definePageMeta({
    middleware: ['auth'],
});

let debounceTimer: ReturnType<typeof setTimeout> | null = null;

// Saved searches state
const savedSearchesPanel = ref<{ reload: () => Promise<void> } | null>(null);
const showSaveModal = ref(false);
const currentFilters = computed(() => searchStore.getCurrentFilters());

const syncFromUrl = () => {
    const q = route.query;
    searchStore.query = (q.q as string) || '';
    searchStore.selectedTags = q.tags ? (q.tags as string).split(',') : [];
    searchStore.selectedActors = q.actors ? (q.actors as string).split(',') : [];
    searchStore.studio = (q.studio as string) || '';
    searchStore.minDuration = q.min_duration ? Number(q.min_duration) : 0;
    searchStore.maxDuration = q.max_duration ? Number(q.max_duration) : 0;
    searchStore.minDate = (q.min_date as string) || '';
    searchStore.maxDate = (q.max_date as string) || '';
    searchStore.resolution = (q.resolution as string) || '';
    searchStore.sort = (q.sort as string) || '';
    searchStore.page = q.page ? Number(q.page) : 1;
    searchStore.liked = q.liked === 'true';
    searchStore.minRating = q.min_rating ? Number(q.min_rating) : 0;
    searchStore.maxRating = q.max_rating ? Number(q.max_rating) : 0;
    searchStore.minJizzCount = q.min_jizz_count ? Number(q.min_jizz_count) : 0;
    searchStore.maxJizzCount = q.max_jizz_count ? Number(q.max_jizz_count) : 0;
    const matchType = q.match_type as string;
    searchStore.matchType =
        matchType === 'strict' || matchType === 'frequency' ? matchType : 'broad';
};

const syncToUrl = () => {
    const query: Record<string, string> = {};
    if (searchStore.query) query.q = searchStore.query;
    if (searchStore.selectedTags.length > 0) query.tags = searchStore.selectedTags.join(',');
    if (searchStore.selectedActors.length > 0) query.actors = searchStore.selectedActors.join(',');
    if (searchStore.studio) query.studio = searchStore.studio;
    if (searchStore.minDuration > 0) query.min_duration = String(searchStore.minDuration);
    if (searchStore.maxDuration > 0) query.max_duration = String(searchStore.maxDuration);
    if (searchStore.minDate) query.min_date = searchStore.minDate;
    if (searchStore.maxDate) query.max_date = searchStore.maxDate;
    if (searchStore.resolution) query.resolution = searchStore.resolution;
    if (searchStore.sort) query.sort = searchStore.sort;
    if (searchStore.page > 1) query.page = String(searchStore.page);
    if (searchStore.liked) query.liked = 'true';
    if (searchStore.minRating > 0) query.min_rating = String(searchStore.minRating);
    if (searchStore.maxRating > 0) query.max_rating = String(searchStore.maxRating);
    if (searchStore.minJizzCount > 0) query.min_jizz_count = String(searchStore.minJizzCount);
    if (searchStore.maxJizzCount > 0) query.max_jizz_count = String(searchStore.maxJizzCount);
    if (searchStore.matchType !== 'broad') query.match_type = searchStore.matchType;

    router.replace({ query });
};

const triggerSearch = () => {
    syncToUrl();
    searchStore.search();
};

const debouncedSearch = () => {
    if (debounceTimer) clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => {
        searchStore.page = 1;
        triggerSearch();
    }, 300);
};

watch(() => searchStore.query, debouncedSearch);

watch(
    () => [
        searchStore.selectedTags,
        searchStore.selectedActors,
        searchStore.studio,
        searchStore.minDuration,
        searchStore.maxDuration,
        searchStore.minDate,
        searchStore.maxDate,
        searchStore.resolution,
        searchStore.sort,
        searchStore.liked,
        searchStore.minRating,
        searchStore.maxRating,
        searchStore.minJizzCount,
        searchStore.maxJizzCount,
        searchStore.matchType,
    ],
    () => {
        searchStore.page = 1;
        triggerSearch();
    },
    { deep: true },
);

watch(() => searchStore.page, triggerSearch);

onMounted(() => {
    syncFromUrl();
    searchStore.loadFilterOptions();
    searchStore.search();
});

const handleLoadSavedSearch = (filters: SavedSearchFilters) => {
    searchStore.loadFilters(filters);
    syncToUrl();
    searchStore.search();
};

const handleSearchSaved = (search: SavedSearch) => {
    showSaveModal.value = false;
    savedSearchesPanel.value?.reload();
};
</script>

<template>
    <div class="mx-auto max-w-415 px-4 py-6 sm:px-5">
        <div class="mb-5 flex items-center gap-3">
            <div class="min-w-0 flex-1">
                <SearchBar />
            </div>
            <button
                v-if="searchStore.hasActiveFilters"
                @click="showSaveModal = true"
                class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10 flex shrink-0
                    items-center gap-1.5 rounded-lg border px-3 py-2 text-xs font-medium text-white
                    transition-all"
                title="Save current search"
            >
                <Icon name="heroicons:bookmark" size="14" />
                <span class="hidden sm:inline">Save</span>
            </button>
        </div>

        <SearchActiveFilters class="mb-4" />

        <div class="flex gap-5">
            <aside class="hidden w-56 shrink-0 lg:block">
                <SearchSavedSearchesPanel ref="savedSearchesPanel" @load="handleLoadSavedSearch" />
                <SearchFilters />
            </aside>

            <div class="min-w-0 flex-1">
                <SearchResults />
            </div>
        </div>

        <SearchSaveSearchModal
            :visible="showSaveModal"
            :filters="currentFilters"
            @close="showSaveModal = false"
            @saved="handleSearchSaved"
        />
    </div>
</template>
