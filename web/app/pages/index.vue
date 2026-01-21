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
</script>

<template>
    <div class="bg-primary min-h-screen px-4 py-8 sm:px-6 lg:px-8">
        <div class="mx-auto max-w-7xl">
            <!-- Header -->
            <header class="mb-12 flex items-center justify-between">
                <div>
                    <h1 class="text-4xl font-extrabold tracking-tight text-white sm:text-5xl">
                        Goon<span class="text-neon-green">Hub</span>
                    </h1>
                </div>
            </header>

            <!-- Upload Section -->
            <VideoUpload />

            <!-- Content Area -->
            <div class="mt-12">
                <div class="mb-6 flex items-center justify-between">
                    <h2 class="text-2xl font-bold text-white">Library</h2>
                    <span class="rounded-full bg-white/5 px-3 py-1 text-xs text-gray-400">
                        {{ store.total }} Videos
                    </span>
                </div>

                <!-- Loading State -->
                <div
                    v-if="store.isLoading && store.videos.length === 0"
                    class="flex h-64 items-center justify-center"
                >
                    <div
                        class="border-t-neon-green h-8 w-8 animate-spin rounded-full border-4
                            border-white/10"
                    ></div>
                </div>

                <!-- Empty State -->
                <div
                    v-else-if="store.videos.length === 0"
                    class="flex h-64 flex-col items-center justify-center rounded-2xl border
                        border-dashed border-white/10 bg-white/5 text-center"
                >
                    <p class="text-lg text-gray-400">No videos yet</p>
                    <p class="text-sm text-gray-600">Upload your first video to get started</p>
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
