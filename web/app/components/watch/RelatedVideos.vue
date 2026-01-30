<script setup lang="ts">
import type { Video, VideoListItem } from '~/types/video';

const { fetchRelatedVideos } = useApiVideos();

const video = inject<Ref<Video | null>>('watchVideo');

const relatedVideos = ref<VideoListItem[]>([]);
const isLoading = ref(false);
const error = ref<string | null>(null);

const INITIAL_LIMIT = 15;

const loadRelatedVideos = async () => {
    if (!video?.value?.id) return;

    isLoading.value = true;
    error.value = null;

    try {
        const response = await fetchRelatedVideos(video.value.id, INITIAL_LIMIT);
        relatedVideos.value = response.data || [];
    } catch (err) {
        error.value = err instanceof Error ? err.message : 'Failed to load related videos';
    } finally {
        isLoading.value = false;
    }
};

onMounted(() => {
    loadRelatedVideos();
});

watch(
    () => video?.value?.id,
    () => {
        loadRelatedVideos();
    },
);
</script>

<template>
    <div v-if="relatedVideos.length > 0 || isLoading" class="mt-6">
        <!-- Section Header -->
        <div class="mb-4 flex items-center justify-between">
            <div class="flex items-center gap-2">
                <div
                    class="from-lava/20 to-lava/5 flex h-7 w-7 items-center justify-center
                        rounded-lg bg-linear-to-br"
                >
                    <Icon name="heroicons:sparkles" size="14" class="text-lava" />
                </div>
                <h2 class="text-sm font-semibold text-white">Related Videos</h2>
            </div>
            <span v-if="relatedVideos.length > 0" class="text-dim text-xs">
                {{ relatedVideos.length }} videos
            </span>
        </div>

        <!-- Loading State -->
        <div
            v-if="isLoading"
            class="border-border bg-surface/50 flex h-48 items-center justify-center rounded-xl
                border"
        >
            <LoadingSpinner />
        </div>

        <!-- Error State -->
        <div
            v-else-if="error"
            class="border-border bg-surface/50 flex h-48 items-center justify-center rounded-xl
                border"
        >
            <span class="text-dim text-xs">{{ error }}</span>
        </div>

        <!-- Related Videos Grid -->
        <template v-else>
            <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5">
                <VideoCard v-for="v in relatedVideos" :key="v.id" :video="v" fluid />
            </div>
        </template>
    </div>
</template>
