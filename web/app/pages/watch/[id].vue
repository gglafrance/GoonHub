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
    <div class="bg-primary min-h-screen">
        <!-- Back Navigation Bar -->
        <div
            class="sticky top-0 z-50 border-b border-white/5 bg-black/80 px-4 py-4 backdrop-blur-md
                sm:px-6 lg:px-8"
        >
            <div class="mx-auto flex max-w-480 items-center justify-between">
                <button
                    class="group flex items-center gap-2 text-gray-400 transition-colors
                        hover:text-white"
                    @click="goBack"
                >
                    <div
                        class="group-hover:bg-neon-green flex h-8 w-8 items-center justify-center
                            rounded-full bg-white/5 transition-colors group-hover:text-black"
                    >
                        <Icon name="heroicons:arrow-left" size="16" />
                    </div>
                    <span class="font-medium tracking-wide">Back to Library</span>
                </button>

                <div v-if="video" class="hidden text-sm font-medium text-gray-400 sm:block">
                    {{ video.title }}
                </div>
            </div>
        </div>

        <div class="mx-auto max-w-480 p-4 sm:px-6 lg:px-8 lg:py-8">
            <!-- Loading State -->
            <div
                v-if="isLoading"
                class="flex h-[70vh] items-center justify-center rounded-3xl bg-white/5"
            >
                <div class="flex flex-col items-center gap-4">
                    <div
                        class="border-t-neon-green h-12 w-12 animate-spin rounded-full border-4
                            border-white/10"
                    ></div>
                    <span class="animate-pulse text-sm font-medium text-gray-400">
                        Loading Video...
                    </span>
                </div>
            </div>

            <!-- Error State -->
            <div
                v-else-if="error || hasProcessingError"
                class="flex h-[70vh] flex-col items-center justify-center rounded-3xl bg-white/5
                    text-center"
            >
                <Icon name="heroicons:exclamation-triangle" size="64" class="text-neon-red" />
                <h2 class="mt-4 text-3xl font-bold text-white">Video Unavailable</h2>
                <p class="mt-2 text-lg text-gray-400">
                    {{ error || 'Video processing failed. Please try reprocessing.' }}
                </p>
                <button
                    class="bg-neon-green mt-8 rounded-full px-8 py-3 font-bold text-black
                        transition-all hover:scale-105 hover:shadow-[0_0_20px_rgba(46,204,113,0.4)]"
                    @click="goBack"
                >
                    Return to Library
                </button>
            </div>

            <!-- Video Player & Content -->
            <div v-else-if="video" class="grid gap-8 xl:grid-cols-[1fr_350px]">
                <div class="min-w-0 space-y-6">
                    <!-- Processing State -->
                    <div
                        v-if="isProcessing"
                        class="flex aspect-video flex-col items-center justify-center rounded-3xl
                            bg-white/5 text-center"
                    >
                        <Icon
                            name="heroicons:arrow-path"
                            size="64"
                            class="text-neon-green animate-spin"
                        />
                        <h2 class="mt-6 text-2xl font-bold text-white">Processing Video</h2>
                        <p class="mt-2 text-gray-400">
                            Optimization in progress. This may take a few minutes.
                        </p>
                    </div>

                    <!-- Player -->
                    <div v-else class="space-y-6">
                        <!-- Player Error Alert -->
                        <Transition
                            enter-active-class="transition duration-300 ease-out"
                            enter-from-class="transform -translate-y-4 opacity-0"
                            enter-to-class="transform translate-y-0 opacity-100"
                            leave-active-class="transition duration-200 ease-in"
                            leave-from-class="transform translate-y-0 opacity-100"
                            leave-to-class="transform -translate-y-4 opacity-0"
                        >
                            <div
                                v-if="playerError"
                                class="border-neon-red/50 rounded-xl border bg-red-950/30 p-4
                                    backdrop-blur-md"
                            >
                                <div class="flex items-center gap-3">
                                    <Icon
                                        name="heroicons:exclamation-triangle"
                                        size="24"
                                        class="text-neon-red"
                                    />
                                    <div>
                                        <h3 class="font-bold text-white">Playback Error</h3>
                                        <p class="text-sm text-gray-300">
                                            {{
                                                playerError?.message ||
                                                'Failed to initialize player'
                                            }}
                                        </p>
                                    </div>
                                </div>
                            </div>
                        </Transition>

                        <div
                            class="overflow-hidden rounded-2xl bg-black shadow-2xl ring-1
                                ring-white/10"
                        >
                            <VideoPlayer
                                :video-url="streamUrl"
                                :poster-url="posterUrl"
                                :video="video"
                                @error="playerError = $event"
                            />
                        </div>

                        <!-- Mobile Metadata (shown below player on small screens) -->
                        <div class="block xl:hidden">
                            <h1 class="text-2xl font-bold text-white sm:text-3xl">
                                {{ video.title }}
                            </h1>
                            <div class="mt-4 flex flex-wrap gap-4 text-sm text-gray-400">
                                <span class="flex items-center gap-1.5">
                                    <Icon name="heroicons:eye" class="text-neon-green" />
                                    {{ video.view_count }} views
                                </span>
                                <span class="flex items-center gap-1.5">
                                    <Icon name="heroicons:calendar" class="text-neon-green" />
                                    {{ formatDate(video.created_at) }}
                                </span>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Sidebar Metadata (Desktop) -->
                <div class="hidden xl:block">
                    <div class="sticky top-24 space-y-6">
                        <div
                            class="rounded-3xl border border-white/5 bg-white/5 p-6
                                backdrop-blur-xl"
                        >
                            <h1 class="text-2xl leading-tight font-bold text-white">
                                {{ video.title }}
                            </h1>

                            <div class="mt-6 space-y-4">
                                <div
                                    class="flex items-center justify-between border-b border-white/5
                                        pb-4"
                                >
                                    <span class="text-gray-400">Duration</span>
                                    <span class="font-mono font-medium text-white">
                                        {{ formatDuration(video.duration) }}
                                    </span>
                                </div>

                                <div
                                    class="flex items-center justify-between border-b border-white/5
                                        pb-4"
                                >
                                    <span class="text-gray-400">Size</span>
                                    <span class="font-mono font-medium text-white">
                                        {{ formatSize(video.size) }}
                                    </span>
                                </div>

                                <div
                                    class="flex items-center justify-between border-b border-white/5
                                        pb-4"
                                >
                                    <span class="text-gray-400">Views</span>
                                    <span class="font-mono font-medium text-white">
                                        {{ video.view_count }}
                                    </span>
                                </div>

                                <div class="flex items-center justify-between pb-2">
                                    <span class="text-gray-400">Added</span>
                                    <span class="font-mono font-medium text-white">
                                        {{ formatDate(video.created_at) }}
                                    </span>
                                </div>
                            </div>

                            <div class="mt-8 border-t border-white/5 pt-6">
                                <h3
                                    class="text-sm font-medium tracking-wider text-gray-500
                                        uppercase"
                                >
                                    File Details
                                </h3>
                                <p class="mt-2 font-mono text-xs break-all text-gray-400">
                                    {{ video.original_filename }}
                                </p>
                            </div>
                        </div>

                        <!-- Actions Card (Placeholder for future features) -->
                        <div
                            class="rounded-3xl border border-white/5 bg-white/5 p-6
                                backdrop-blur-xl"
                        >
                            <div class="flex gap-2">
                                <button
                                    class="flex-1 rounded-xl bg-white/10 py-3 text-sm font-bold
                                        text-white transition-colors hover:bg-white/20"
                                >
                                    <Icon name="heroicons:share" class="mr-2" />
                                    Share
                                </button>
                                <button
                                    class="flex-1 rounded-xl bg-white/10 py-3 text-sm font-bold
                                        text-white transition-colors hover:bg-white/20"
                                >
                                    <Icon name="heroicons:heart" class="mr-2" />
                                    Favorite
                                </button>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </div>
</template>
