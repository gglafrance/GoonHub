<script setup lang="ts">
import type { SavedSearch, SavedSearchFilters } from '~/types/saved_search';

const searchStore = useSearchStore();
const route = useRoute();
const router = useRouter();

const pageTitle = computed(() => (searchStore.query ? `${searchStore.query} - Search` : 'Search'));

useHead({ title: pageTitle });

useSeoMeta({
    title: pageTitle,
    ogTitle: computed(() => `${pageTitle.value} - GoonHub`),
    description: 'Search your scene library',
    ogDescription: 'Search your scene library',
});

definePageMeta({
    middleware: ['auth'],
});

let debounceTimer: ReturnType<typeof setTimeout> | null = null;
let isUpdatingUrl = false;
let isSyncingFromUrl = false;

// Saved searches state
const savedSearchesPanel = ref<{ reload: () => Promise<void> } | null>(null);
const showSaveModal = ref(false);
const showMobileFilters = ref(false);
const currentFilters = computed(() => searchStore.getCurrentFilters());

// Active filter count for mobile button badge
const activeFilterCount = computed(() => {
    let count = 0;
    if (searchStore.selectedTags.length > 0) count += searchStore.selectedTags.length;
    if (searchStore.selectedActors.length > 0) count += searchStore.selectedActors.length;
    if (searchStore.selectedMarkerLabels.length > 0)
        count += searchStore.selectedMarkerLabels.length;
    if (searchStore.studio) count++;
    if (searchStore.minDuration > 0 || searchStore.maxDuration > 0) count++;
    if (searchStore.minSize > 0 || searchStore.maxSize > 0) count++;
    count += searchStore.activeStatusCount;
    count += searchStore.selectedSourceSites.length;
    if (searchStore.minDate || searchStore.maxDate) count++;
    if (searchStore.resolution) count++;
    if (searchStore.liked) count++;
    if (searchStore.minRating > 0 || searchStore.maxRating > 0) count++;
    if (searchStore.minJizzCount > 0 || searchStore.maxJizzCount > 0) count++;
    if (searchStore.matchType !== 'broad') count++;
    return count;
});

// The whole search (filters, source sites, sort, random seed, page) lives in the URL: Back, a
// reload or a shared link restore it exactly. See utils/searchUrlState.ts for the mapping.
const syncFromUrl = (): boolean => {
    isSyncingFromUrl = true;
    const complete = searchStore.applyUrlState(searchStateFromQuery(route.query));
    nextTick(() => {
        isSyncingFromUrl = false;
    });
    return complete;
};

const syncToUrl = () => {
    const query = searchStateToQuery(searchStore.getUrlState());
    if (sameQuery(route.query, query)) return;
    isUpdatingUrl = true;
    router.replace({ query }).finally(() => {
        isUpdatingUrl = false;
    });
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

watch(
    () => searchStore.query,
    () => {
        if (isSyncingFromUrl) return;
        debouncedSearch();
    },
);

watch(
    () => [
        searchStore.selectedTags,
        searchStore.selectedActors,
        searchStore.studio,
        searchStore.minDuration,
        searchStore.maxDuration,
        searchStore.minSize,
        searchStore.maxSize,
        JSON.stringify(searchStore.status),
        searchStore.selectedSourceSites,
        searchStore.minDate,
        searchStore.maxDate,
        searchStore.resolution,
        searchStore.sort,
        searchStore.liked,
        searchStore.minRating,
        searchStore.maxRating,
        searchStore.minJizzCount,
        searchStore.maxJizzCount,
        searchStore.selectedMarkerLabels,
        searchStore.matchType,
        searchStore.seed,
    ],
    () => {
        if (isSyncingFromUrl) return;
        searchStore.page = 1;
        triggerSearch();
    },
    { deep: true },
);

watch(
    () => searchStore.page,
    () => {
        if (isSyncingFromUrl) return;
        triggerSearch();
    },
);

// Handle browser back/forward navigation
watch(
    () => route.query,
    async () => {
        // Skip if we're the ones updating the URL
        if (isUpdatingUrl) return;
        // A saved search opened while already on this page (e.g. from the command palette)
        if (route.query.saved && (await loadSavedSearchFromUrl())) return;
        if (!syncFromUrl()) syncToUrl();
        searchStore.search();
    },
    { deep: true },
);

const loadSavedSearchFromUrl = async () => {
    const savedUuid = route.query.saved as string | undefined;
    if (!savedUuid) return false;

    try {
        const api = useApi();
        const savedSearch = await api.fetchSavedSearch(savedUuid);
        searchStore.loadFilters(savedSearch.filters);
        // Override seed from URL if provided (e.g. from homepage "See all")
        if (route.query.seed) {
            searchStore.seed = Number(route.query.seed);
        }
        syncToUrl(); // Expand to individual filter params (removes ?saved=)
        searchStore.search();
        return true;
    } catch {
        // Saved search not found, fall through to normal sync
        return false;
    }
};

onMounted(async () => {
    searchStore.loadFilterOptions();
    searchStore.loadSourceSites();
    const loaded = await loadSavedSearchFromUrl();
    if (!loaded) {
        if (!syncFromUrl()) syncToUrl();
        searchStore.search();
    }
});

const handleLoadSavedSearch = (filters: SavedSearchFilters) => {
    searchStore.loadFilters(filters);
    syncToUrl();
    searchStore.search();
};

const handleSearchSaved = (_search: SavedSearch) => {
    showSaveModal.value = false;
    savedSearchesPanel.value?.reload();
};
</script>

<template>
    <div class="mx-auto max-w-415 px-4 py-4 sm:py-6">
        <div class="mb-4 flex items-center gap-2 sm:mb-5 sm:gap-3">
            <div class="min-w-0 flex-1">
                <SearchBar />
            </div>

            <!-- Mobile: Filter button -->
            <button
                class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10 relative flex
                    h-10 w-10 shrink-0 items-center justify-center rounded-lg border transition-all
                    lg:hidden"
                title="Filters"
                @click="showMobileFilters = true"
            >
                <Icon name="heroicons:adjustments-horizontal" size="18" class="text-white" />
                <span
                    v-if="activeFilterCount > 0"
                    class="bg-lava absolute -top-1.5 -right-1.5 flex h-5 min-w-5 items-center
                        justify-center rounded-full px-1 text-[10px] font-bold text-white"
                >
                    {{ activeFilterCount }}
                </span>
            </button>

            <!-- Triage the results one at a time -->
            <NuxtLink
                v-if="searchStore.hasActiveFilters"
                :to="{ path: '/triage', query: searchStore.getSearchParams() }"
                class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10 flex h-10
                    shrink-0 items-center gap-1.5 rounded-lg border px-3 text-xs font-medium
                    text-white transition-all sm:py-2"
                title="Triage these scenes one at a time"
            >
                <Icon name="heroicons:bolt" size="14" />
                <span class="hidden sm:inline">Triage</span>
            </NuxtLink>

            <!-- Save search button -->
            <button
                v-if="searchStore.hasActiveFilters"
                class="border-border bg-surface hover:border-lava/40 hover:bg-lava/10 flex h-10
                    shrink-0 items-center gap-1.5 rounded-lg border px-3 text-xs font-medium
                    text-white transition-all sm:py-2"
                title="Save current search"
                @click="showSaveModal = true"
            >
                <Icon name="heroicons:bookmark" size="14" />
                <span class="hidden sm:inline">Save</span>
            </button>
        </div>

        <SearchActiveFilters class="mb-3 sm:mb-4" />

        <div class="flex gap-5">
            <aside class="hidden w-56 shrink-0 lg:block">
                <SearchSavedSearchesPanel ref="savedSearchesPanel" @load="handleLoadSavedSearch" />
                <SearchFilters />
            </aside>

            <div class="min-w-0 flex-1">
                <SearchResults />
            </div>
        </div>

        <!-- Mobile Filters Drawer -->
        <SearchMobileFilters :visible="showMobileFilters" @close="showMobileFilters = false" />

        <SearchSaveSearchModal
            :visible="showSaveModal"
            :filters="currentFilters"
            @close="showSaveModal = false"
            @saved="handleSearchSaved"
        />
    </div>
</template>
