<script setup lang="ts">
const store = useVideoStore();
const route = useRoute();
const router = useRouter();

useHead({ title: 'Library' });

// Get initial page from query parameter
const getPageFromQuery = (): number => {
    const pageParam = route.query.page;
    if (pageParam) {
        const parsed = parseInt(pageParam as string, 10);
        if (!isNaN(parsed) && parsed >= 1) {
            return parsed;
        }
    }
    return 1;
};

// Update URL query parameter
const updatePageQuery = (page: number) => {
    const query = { ...route.query };
    if (page === 1) {
        delete query.page;
    } else {
        query.page = String(page);
    }
    router.replace({ query });
};

// Track if initial load has completed to avoid duplicate fetches
const initialLoadDone = ref(false);

// Initial load with page from URL
onMounted(async () => {
    const initialPage = getPageFromQuery();
    // Set currentPage before loading to prevent watcher from triggering
    store.currentPage = initialPage;
    await store.loadVideos(initialPage);
    // Update URL if API returned a different page (e.g., invalid page requested)
    if (store.currentPage !== initialPage) {
        updatePageQuery(store.currentPage);
    }
    initialLoadDone.value = true;
});

// Watch page changes and sync URL (only after initial load)
watch(
    () => store.currentPage,
    (newPage, oldPage) => {
        if (initialLoadDone.value && newPage !== oldPage) {
            store.loadVideos(newPage);
            updatePageQuery(newPage);
        }
    },
);

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-415">
            <!-- Upload Section -->
            <VideoUpload />

            <!-- Content Area -->
            <div class="mt-8">
                <div class="mb-4 flex items-center justify-between">
                    <h2 class="text-sm font-semibold tracking-wide text-white uppercase">
                        Library
                    </h2>
                    <span
                        class="border-border bg-panel text-dim rounded-full border px-2.5 py-0.5
                            font-mono text-[11px]"
                    >
                        {{ store.total }} videos
                    </span>
                </div>

                <!-- Loading State -->
                <div
                    v-if="store.isLoading && store.videos.length === 0"
                    class="flex h-64 items-center justify-center"
                >
                    <LoadingSpinner label="Loading library..." />
                </div>

                <!-- Empty State -->
                <div
                    v-else-if="store.videos.length === 0"
                    class="border-border flex h-64 flex-col items-center justify-center rounded-xl
                        border border-dashed text-center"
                >
                    <div
                        class="bg-panel border-border flex h-10 w-10 items-center justify-center
                            rounded-lg border"
                    >
                        <Icon name="heroicons:film" size="20" class="text-dim" />
                    </div>
                    <p class="text-muted mt-3 text-sm">No videos yet</p>
                    <p class="text-dim mt-1 text-xs">Upload your first video to get started</p>
                </div>

                <!-- Video Grid -->
                <div v-else>
                    <VideoGrid :videos="store.videos" />

                    <Pagination
                        v-model="store.currentPage"
                        :total="store.total"
                        :limit="store.limit"
                    />
                </div>
            </div>
        </div>
    </div>
</template>
