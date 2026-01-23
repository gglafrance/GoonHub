<script setup lang="ts">
import { useVideoStore } from '~/stores/videos';

const store = useVideoStore();

// Initial load
onMounted(() => {
    store.loadVideos();
});

// Watch page changes
watch(
    () => store.currentPage,
    (newPage) => {
        store.loadVideos(newPage);
    },
);

definePageMeta({
    title: 'Library - GoonHub',
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen px-4 py-6 sm:px-5">
        <div class="mx-auto max-w-400">
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
                    <div class="flex flex-col items-center gap-3">
                        <div
                            class="border-border border-t-lava h-6 w-6 animate-spin rounded-full
                                border-2"
                        ></div>
                        <span class="text-dim text-[11px]">Loading library...</span>
                    </div>
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
