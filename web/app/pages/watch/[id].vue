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
    <div class="min-h-screen">
        <!-- Back Navigation Bar -->
        <div
            class="border-border bg-void/90 sticky top-12 z-40 border-b px-4 py-2.5 backdrop-blur-md
                sm:px-5"
        >
            <div class="mx-auto flex max-w-400 items-center justify-between">
                <button
                    class="group text-dim flex items-center gap-2 transition-colors
                        hover:text-white"
                    @click="goBack"
                >
                    <div
                        class="border-border bg-panel group-hover:border-lava/30
                            group-hover:text-lava flex h-6 w-6 items-center justify-center
                            rounded-md border transition-all"
                    >
                        <Icon name="heroicons:arrow-left" size="12" />
                    </div>
                    <span class="text-xs font-medium">Library</span>
                </button>

                <div v-if="video" class="text-dim hidden truncate text-xs sm:block">
                    {{ video.title }}
                </div>
            </div>
        </div>

        <div class="mx-auto max-w-400 p-4 sm:px-5 lg:py-6">
            <!-- Loading State -->
            <div v-if="isLoading" class="flex h-[70vh] items-center justify-center">
                <div class="flex flex-col items-center gap-3">
                    <div
                        class="border-border border-t-lava h-6 w-6 animate-spin rounded-full
                            border-2"
                    ></div>
                    <span class="text-dim text-[11px]">Loading...</span>
                </div>
            </div>

            <!-- Error State -->
            <div
                v-else-if="error || hasProcessingError"
                class="flex h-[70vh] flex-col items-center justify-center text-center"
            >
                <div
                    class="border-lava/20 bg-lava/5 flex h-12 w-12 items-center justify-center
                        rounded-xl border"
                >
                    <Icon name="heroicons:exclamation-triangle" size="24" class="text-lava" />
                </div>
                <h2 class="mt-4 text-lg font-semibold text-white">Video Unavailable</h2>
                <p class="text-dim mt-1 text-xs">
                    {{ error || 'Video processing failed. Please try reprocessing.' }}
                </p>
                <button
                    class="border-border bg-surface text-muted hover:border-border-hover mt-6
                        rounded-lg border px-4 py-2 text-xs font-medium transition-all
                        hover:text-white"
                    @click="goBack"
                >
                    Return to Library
                </button>
            </div>

            <!-- Video Player & Content -->
            <div v-else-if="video" class="grid gap-5 xl:grid-cols-[1fr_280px]">
                <div class="min-w-0 space-y-4">
                    <!-- Processing State -->
                    <div
                        v-if="isProcessing"
                        class="border-border bg-surface flex aspect-video flex-col items-center
                            justify-center rounded-xl border text-center"
                    >
                        <div
                            class="border-border border-t-lava h-6 w-6 animate-spin rounded-full
                                border-2"
                        ></div>
                        <h2 class="mt-4 text-sm font-semibold text-white">Processing</h2>
                        <p class="text-dim mt-1 text-xs">Optimization in progress...</p>
                    </div>

                    <!-- Player -->
                    <div v-else class="space-y-4">
                        <!-- Player Error Alert -->
                        <Transition
                            enter-active-class="transition duration-200 ease-out"
                            enter-from-class="transform -translate-y-2 opacity-0"
                            enter-to-class="transform translate-y-0 opacity-100"
                            leave-active-class="transition duration-150 ease-in"
                            leave-from-class="transform translate-y-0 opacity-100"
                            leave-to-class="transform -translate-y-2 opacity-0"
                        >
                            <div
                                v-if="playerError"
                                class="border-lava/30 bg-lava/5 rounded-lg border px-3 py-2
                                    backdrop-blur-sm"
                            >
                                <div class="flex items-center gap-2">
                                    <Icon
                                        name="heroicons:exclamation-triangle"
                                        size="14"
                                        class="text-lava"
                                    />
                                    <div>
                                        <span class="text-xs font-medium text-white"
                                            >Playback Error</span
                                        >
                                        <span class="text-dim ml-2 text-[11px]">
                                            {{
                                                playerError?.message ||
                                                'Failed to initialize player'
                                            }}
                                        </span>
                                    </div>
                                </div>
                            </div>
                        </Transition>

                        <div class="border-border bg-void overflow-hidden rounded-xl border">
                            <VideoPlayer
                                :video-url="streamUrl"
                                :poster-url="posterUrl"
                                :video="video"
                                @error="playerError = $event"
                            />
                        </div>

                        <!-- Mobile Metadata -->
                        <div class="block xl:hidden">
                            <h1 class="text-sm font-semibold text-white">
                                {{ video.title }}
                            </h1>
                            <div class="text-dim mt-2 flex flex-wrap gap-3 font-mono text-[11px]">
                                <span class="flex items-center gap-1">
                                    <Icon name="heroicons:eye" size="12" class="text-lava" />
                                    {{ video.view_count }} views
                                </span>
                                <span class="flex items-center gap-1">
                                    <Icon name="heroicons:calendar" size="12" class="text-lava" />
                                    {{ formatDate(video.created_at) }}
                                </span>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- Sidebar Metadata (Desktop) -->
                <div class="hidden xl:block">
                    <div class="sticky top-28 space-y-3">
                        <div
                            class="border-border bg-surface/50 rounded-xl border p-4
                                backdrop-blur-sm"
                        >
                            <h1 class="text-sm leading-snug font-semibold text-white">
                                {{ video.title }}
                            </h1>

                            <div class="mt-4 space-y-0">
                                <div
                                    class="border-border flex items-center justify-between border-b
                                        py-2.5"
                                >
                                    <span class="text-dim text-[11px]">Duration</span>
                                    <span class="text-muted font-mono text-[11px]">
                                        {{ formatDuration(video.duration) }}
                                    </span>
                                </div>

                                <div
                                    class="border-border flex items-center justify-between border-b
                                        py-2.5"
                                >
                                    <span class="text-dim text-[11px]">Size</span>
                                    <span class="text-muted font-mono text-[11px]">
                                        {{ formatSize(video.size) }}
                                    </span>
                                </div>

                                <div
                                    class="border-border flex items-center justify-between border-b
                                        py-2.5"
                                >
                                    <span class="text-dim text-[11px]">Views</span>
                                    <span class="text-muted font-mono text-[11px]">
                                        {{ video.view_count }}
                                    </span>
                                </div>

                                <div class="flex items-center justify-between py-2.5">
                                    <span class="text-dim text-[11px]">Added</span>
                                    <span class="text-muted font-mono text-[11px]">
                                        {{ formatDate(video.created_at) }}
                                    </span>
                                </div>
                            </div>

                            <div class="border-border mt-4 border-t pt-3">
                                <span
                                    class="text-dim text-[10px] font-medium tracking-wider
                                        uppercase"
                                    >File</span
                                >
                                <p class="text-dim/70 mt-1 font-mono text-[10px] break-all">
                                    {{ video.original_filename }}
                                </p>
                            </div>
                        </div>

                        <!-- Actions -->
                        <div
                            class="border-border bg-surface/50 rounded-xl border p-3
                                backdrop-blur-sm"
                        >
                            <div class="flex gap-2">
                                <button
                                    class="border-border bg-panel text-dim hover:border-border-hover
                                        flex-1 rounded-lg border py-2 text-[11px] font-medium
                                        transition-all hover:text-white"
                                >
                                    <Icon name="heroicons:share" size="12" class="mr-1" />
                                    Share
                                </button>
                                <button
                                    class="border-border bg-panel text-dim hover:border-lava/30
                                        hover:text-lava flex-1 rounded-lg border py-2 text-[11px]
                                        font-medium transition-all"
                                >
                                    <Icon name="heroicons:heart" size="12" class="mr-1" />
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
