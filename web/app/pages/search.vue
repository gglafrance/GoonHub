<script setup lang="ts">
const searchStore = useSearchStore();
const route = useRoute();
const router = useRouter();

let debounceTimer: ReturnType<typeof setTimeout> | null = null;

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
</script>

<template>
    <div class="mx-auto max-w-400 px-4 py-6 sm:px-5">
        <div class="mb-5">
            <SearchBar />
        </div>

        <SearchActiveFilters v-if="searchStore.hasActiveFilters" class="mb-4" />

        <div class="flex gap-5">
            <SearchFilters class="hidden w-56 shrink-0 lg:block" />

            <div class="min-w-0 flex-1">
                <SearchResults />
            </div>
        </div>
    </div>
</template>
