<script setup lang="ts">
import type { Video } from '~/types/video';
import { useApi } from '~/composables/useApi';
import { useTime } from '~/composables/useTime';

const route = useRoute();
const router = useRouter();
const { fetchVideo } = useApi();
const { formatDuration, formatSize, formatDate } = useTime();

const video = ref<Video | null>(null);
const isLoading = ref(true);
const error = ref<string | null>(null);
const playerError = ref<any>(null);

const videoId = computed(() => parseInt(route.params.id as string));

const isProcessing = computed(() => {
    return (
        video.value?.processing_status === 'pending' ||
        video.value?.processing_status === 'processing'
    );
});

const hasProcessingError = computed(() => {
    return video.value?.processing_status === 'failed';
});

const streamUrl = computed(() => {
    if (!video.value) return '';
    return `/api/v1/videos/${video.value.id}/stream`;
});

const posterUrl = computed(() => {
    if (!video.value || !video.value.thumbnail_path) return '';
    return `/thumbnails/${video.value.id}`;
});

onMounted(async () => {
    await loadVideo();
});

watch(
    () => route.params.id,
    async () => {
        await loadVideo();
    },
);

const loadVideo = async () => {
    try {
        isLoading.value = true;
        error.value = null;
        video.value = await fetchVideo(videoId.value);
    } catch (err: any) {
        error.value = err.message || 'Failed to load video';
    } finally {
        isLoading.value = false;
    }
};

const goBack = () => {
    router.push('/');
};

definePageMeta({
    middleware: ['auth'],
    title: 'Watch - GoonHub',
});
</script>

<template>
    <div class="bg-primary min-h-screen px-4 py-8 sm:px-6 lg:px-8">
        <div class="mx-auto max-w-6xl">
            <!-- Back Button -->
            <button
                class="mb-6 flex items-center gap-2 text-gray-400 transition-colors
                    hover:text-white"
                @click="goBack"
            >
                <Icon name="heroicons:arrow-left" size="20" />
                <span class="font-medium">Back to Library</span>
            </button>

            <!-- Loading State -->
            <div
                v-if="isLoading"
                class="flex h-96 items-center justify-center rounded-2xl bg-white/5"
            >
                <div
                    class="border-t-neon-green h-12 w-12 animate-spin rounded-full border-4
                        border-white/10"
                ></div>
            </div>

            <!-- Error State -->
            <div
                v-else-if="error || hasProcessingError"
                class="flex h-96 flex-col items-center justify-center rounded-2xl bg-white/5
                    text-center"
            >
                <Icon name="heroicons:exclamation-triangle" size="64" class="text-neon-red" />
                <h2 class="mt-4 text-2xl font-bold text-white">Video Not Available</h2>
                <p class="mt-2 text-gray-400">
                    {{ error || 'Video processing failed. Please try reprocessing.' }}
                </p>
                <button
                    class="bg-neon-green mt-6 rounded-xl px-6 py-3 font-bold text-black
                        transition-transform hover:scale-105"
                    @click="goBack"
                >
                    Go Back
                </button>
            </div>

            <!-- Video Player -->
            <div v-else-if="video">
                <!-- Processing State -->
                <div
                    v-if="isProcessing"
                    class="flex h-96 flex-col items-center justify-center rounded-2xl bg-white/5
                        text-center"
                >
                    <Icon
                        name="heroicons:arrow-path"
                        size="64"
                        class="text-neon-green animate-spin"
                    />
                    <h2 class="mt-4 text-2xl font-bold text-white">Processing Video</h2>
                    <p class="mt-2 text-gray-400">
                        Your video is being processed. Please check back in a few minutes.
                    </p>
                </div>

                <!-- Player -->
                <div v-else class="space-y-6">
                    <!-- Player Error Display -->
                    <div
                        v-if="playerError"
                        class="rounded-2xl border border-neon-red/50 bg-red-950/30 p-6
                            backdrop-blur-md"
                    >
                        <div class="flex items-center gap-3">
                            <Icon name="heroicons:exclamation-triangle" size="24" class="text-neon-red" />
                            <div>
                                <h3 class="font-bold text-white">Video Player Error</h3>
                                <p class="text-sm text-gray-300">
                                    {{ playerError?.message || 'Failed to load video' }}
                                </p>
                            </div>
                        </div>
                    </div>

                    <VideoPlayer
                        :video-url="streamUrl"
                        :poster-url="posterUrl"
                        :video="video"
                        class="rounded-2xl"
                        @error="playerError = $event"
                    />

                    <!-- Video Metadata -->
                    <div class="space-y-4 rounded-2xl bg-white/5 p-6 backdrop-blur-md">
                        <div class="flex items-start justify-between gap-4">
                            <div class="flex-1">
                                <h1 class="text-2xl font-bold text-white">{{ video.title }}</h1>
                                <p class="mt-1 text-sm text-gray-400">
                                    {{ video.original_filename }}
                                </p>
                            </div>
                        </div>

                        <div class="flex flex-wrap gap-6 text-sm">
                            <div class="flex items-center gap-2 text-gray-300">
                                <Icon name="heroicons:clock" size="18" class="text-neon-green" />
                                <span>{{ formatDuration(video.duration) }}</span>
                            </div>
                            <div class="flex items-center gap-2 text-gray-300">
                                <Icon name="heroicons:document" size="18" class="text-neon-green" />
                                <span>{{ formatSize(video.size) }}</span>
                            </div>
                            <div class="flex items-center gap-2 text-gray-300">
                                <Icon name="heroicons:eye" size="18" class="text-neon-green" />
                                <span>{{ video.view_count }} views</span>
                            </div>
                            <div class="flex items-center gap-2 text-gray-300">
                                <Icon name="heroicons:calendar" size="18" class="text-neon-green" />
                                <span>{{ formatDate(video.created_at) }}</span>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
