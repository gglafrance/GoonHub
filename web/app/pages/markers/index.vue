<script setup lang="ts">
import type { MarkerLabelGroup } from '~/types/marker';

useHead({ title: 'Markers' });

useSeoMeta({
    title: 'Markers',
    ogTitle: 'Markers - GoonHub',
    description: 'Browse video markers and bookmarks',
    ogDescription: 'Browse video markers and bookmarks',
});

const { fetchLabelGroups } = useApiMarkers();

const groups = ref<MarkerLabelGroup[]>([]);
const total = ref(0);
const currentPage = ref(1);
const limit = ref(20);
const searchQuery = ref('');
const sortBy = ref('count_desc');
const isLoading = ref(false);
const error = ref<string | null>(null);

const sortOptions = [
    { value: 'count_desc', label: 'Most markers' },
    { value: 'count_asc', label: 'Fewest markers' },
    { value: 'label_asc', label: 'A-Z' },
    { value: 'label_desc', label: 'Z-A' },
    { value: 'recent', label: 'Recently added' },
];

let searchTimeout: ReturnType<typeof setTimeout> | null = null;

// Filter groups by search query (client-side)
const filteredGroups = computed(() => {
    if (!searchQuery.value.trim()) {
        return groups.value;
    }
    const query = searchQuery.value.toLowerCase().trim();
    return groups.value.filter((g) => g.label.toLowerCase().includes(query));
});

const loadGroups = async (page = 1) => {
    isLoading.value = true;
    error.value = null;
    try {
        const response = await fetchLabelGroups(page, limit.value, sortBy.value);
        groups.value = response.data;
        total.value = response.pagination.total_items;
        currentPage.value = page;
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load marker labels';
    } finally {
        isLoading.value = false;
    }
};

onMounted(() => {
    loadGroups();
});

watch(
    () => currentPage.value,
    (newPage) => {
        loadGroups(newPage);
    }
);

watch(sortBy, () => {
    currentPage.value = 1;
    loadGroups(1);
});

// Debounce search to avoid flickering
watch(searchQuery, () => {
    if (searchTimeout) {
        clearTimeout(searchTimeout);
    }
    searchTimeout = setTimeout(() => {
        // Client-side filtering, no need to reload
    }, 100);
});

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Header -->
            <div class="mb-6">
                <div class="flex items-center justify-between">
                    <h1 class="text-lg font-semibold text-white">Markers</h1>
                    <span
                        class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                            font-mono text-[11px]"
                    >
                        {{ total }} labels
                    </span>
                </div>

                <!-- Search bar and sort -->
                <div class="mt-4 flex gap-3">
                    <div class="relative flex-1">
                        <Icon
                            name="heroicons:magnifying-glass"
                            size="16"
                            class="text-dim absolute top-1/2 left-3 -translate-y-1/2"
                        />
                        <input
                            v-model="searchQuery"
                            type="text"
                            placeholder="Filter labels..."
                            class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20
                                w-full rounded-lg border py-2 pr-3 pl-9 text-sm text-white
                                placeholder-white/40 transition-all focus:ring-2 focus:outline-none"
                        />
                    </div>
                    <div class="relative">
                        <select
                            v-model="sortBy"
                            class="border-border bg-panel focus:border-lava/50 focus:ring-lava/20
                                h-full cursor-pointer appearance-none rounded-lg border py-2 pr-8
                                pl-3 text-sm text-white transition-all focus:ring-2 focus:outline-none"
                        >
                            <option
                                v-for="option in sortOptions"
                                :key="option.value"
                                :value="option.value"
                            >
                                {{ option.label }}
                            </option>
                        </select>
                        <Icon
                            name="heroicons:chevron-down"
                            size="14"
                            class="text-dim pointer-events-none absolute top-1/2 right-2.5
                                -translate-y-1/2"
                        />
                    </div>
                </div>
            </div>

            <!-- Error -->
            <ErrorAlert v-if="error" :message="error" class="mb-4" />

            <!-- Loading State -->
            <div
                v-if="isLoading && groups.length === 0"
                class="flex h-64 items-center justify-center"
            >
                <LoadingSpinner label="Loading markers..." />
            </div>

            <!-- Empty State -->
            <div
                v-else-if="groups.length === 0"
                class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                    border border-dashed text-center"
            >
                <div
                    class="bg-panel border-border flex h-10 w-10 items-center justify-center
                        rounded-lg border"
                >
                    <Icon name="heroicons:bookmark" size="20" class="text-dim" />
                </div>
                <p class="text-muted mt-3 text-sm">No marker labels found</p>
                <p class="text-dim mt-1 text-xs">Create markers on videos to see them here</p>
            </div>

            <!-- No search results -->
            <div
                v-else-if="filteredGroups.length === 0"
                class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                    border border-dashed text-center"
            >
                <div
                    class="bg-panel border-border flex h-10 w-10 items-center justify-center
                        rounded-lg border"
                >
                    <Icon name="heroicons:magnifying-glass" size="20" class="text-dim" />
                </div>
                <p class="text-muted mt-3 text-sm">No labels match your search</p>
                <p class="text-dim mt-1 text-xs">Try a different search term</p>
            </div>

            <!-- Label Grid -->
            <div v-else>
                <MarkerLabelGrid :groups="filteredGroups" />

                <Pagination v-model="currentPage" :total="total" :limit="limit" />
            </div>
        </div>
    </div>
</template>
