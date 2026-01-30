<script setup lang="ts">
import type { Video } from '~/types/video';

const route = useRoute();
const router = useRouter();
const { fetchVideo, getResumePosition } = useApi();
const settingsStore = useSettingsStore();
const { formatDuration } = useFormatter();

const video = ref<Video | null>(null);
const isLoading = ref(true);
const error = ref<string | null>(null);

const pageTitle = computed(() => video.value?.title || 'Watch');
useHead({ title: pageTitle });
const playerError = ref<unknown>(null);
const playerRef = ref<{ getCurrentTime: () => number } | null>(null);
const resumePosition = ref(0);
const showResumePrompt = ref(false);
const startTime = ref(0);

const thumbnailVersion = ref(0);
const detailsRefreshKey = ref(0);

provide('getPlayerTime', () => playerRef.value?.getCurrentTime() ?? 0);
provide('watchVideo', video);
provide('thumbnailVersion', thumbnailVersion);
provide('detailsRefreshKey', detailsRefreshKey);
provide('seekToTime', (time: number) => {
    startTime.value = time;
    showResumePrompt.value = false;
});

const videoId = computed(() => parseInt(route.params.id as string));

const isProcessing = computed(() => video.value?.processing_status === 'pending');
const hasProcessingError = computed(() => (video.value ? hasVideoError(video.value) : false));
const isPortrait = computed(() => {
    return video.value?.width && video.value?.height && video.value.height > video.value.width;
});

const streamUrl = computed(() => {
    if (!video.value) return '';
    return `/api/v1/videos/${video.value.id}/stream`;
});

const posterUrl = computed(() => {
    if (!video.value || !video.value.thumbnail_path) return '';
    const base = `/thumbnails/${video.value.id}?size=lg`;
    const v =
        thumbnailVersion.value ||
        (video.value.updated_at ? new Date(video.value.updated_at).getTime() : 0);
    return v ? `${base}&v=${v}` : base;
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
        showResumePrompt.value = false;
        resumePosition.value = 0;
        startTime.value = 0;

        video.value = await fetchVideo(videoId.value);

        // Fetch resume position
        try {
            const res = await getResumePosition(videoId.value);
            if (res.position > 0) {
                resumePosition.value = res.position;
                showResumePrompt.value = true;
            }
        } catch {
            // Ignore errors - resume is optional
        }
    } catch (err: unknown) {
        error.value = err instanceof Error ? err.message : 'Failed to load video';
    } finally {
        isLoading.value = false;
    }
};

const handleResume = () => {
    startTime.value = resumePosition.value;
    showResumePrompt.value = false;
};

const handleStartOver = () => {
    startTime.value = 0;
    showResumePrompt.value = false;
};

const goBack = () => {
    router.push('/');
};

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen">
        <!-- Back Navigation Bar -->
        <div
            class="border-border bg-void/90 sticky top-12 z-40 border-b px-4 py-2.5 backdrop-blur-md
                sm:px-5"
        >
            <div class="mx-auto flex max-w-415 items-center justify-between">
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

        <div class="mx-auto max-w-415 p-4 sm:px-5 lg:py-6">
            <!-- Loading State -->
            <div v-if="isLoading" class="flex h-[70vh] items-center justify-center">
                <LoadingSpinner label="Loading..." />
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
                    <div v-if="isProcessing" class="space-y-4">
                        <div
                            class="border-border bg-surface flex aspect-video flex-col items-center
                                justify-center rounded-xl border text-center"
                        >
                            <LoadingSpinner />
                            <h2 class="mt-4 text-sm font-semibold text-white">Processing</h2>
                            <p class="text-dim mt-1 text-xs">Optimization in progress...</p>
                        </div>
                        <WatchDetailTabs />
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
                                            Failed to initialize player
                                        </span>
                                    </div>
                                </div>
                            </div>
                        </Transition>

                        <div
                            class="border-border bg-void relative overflow-hidden rounded-xl border"
                            :class="{ 'mx-auto max-w-xl': isPortrait }"
                        >
                            <!-- Resume Prompt (overlaid on video) -->
                            <Transition
                                enter-active-class="transition duration-200 ease-out"
                                enter-from-class="opacity-0"
                                enter-to-class="opacity-100"
                                leave-active-class="transition duration-150 ease-in"
                                leave-from-class="opacity-100"
                                leave-to-class="opacity-0"
                            >
                                <div
                                    v-if="showResumePrompt"
                                    class="absolute inset-x-0 top-0 z-20 p-3"
                                >
                                    <div
                                        class="border-lava/30 bg-void/75 rounded-lg border px-4 py-3
                                            backdrop-blur-md"
                                    >
                                        <div class="flex items-center justify-between">
                                            <div class="flex items-center gap-3">
                                                <Icon
                                                    name="heroicons:play-circle"
                                                    size="20"
                                                    class="text-lava"
                                                />
                                                <div>
                                                    <span class="text-xs font-medium text-white">
                                                        Resume watching?
                                                    </span>
                                                    <span class="text-dim ml-2 text-[11px]">
                                                        You left off at
                                                        {{ formatDuration(resumePosition) }}
                                                    </span>
                                                </div>
                                            </div>
                                            <div class="flex items-center gap-2">
                                                <button
                                                    class="text-dim px-3 py-1.5 text-[11px]
                                                        font-medium transition-colors
                                                        hover:text-white"
                                                    @click="handleStartOver"
                                                >
                                                    Start Over
                                                </button>
                                                <button
                                                    class="bg-lava hover:bg-lava/80 rounded-md px-3
                                                        py-1.5 text-[11px] font-medium text-white
                                                        transition-colors"
                                                    @click="handleResume"
                                                >
                                                    Resume
                                                </button>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </Transition>

                            <VideoPlayer
                                ref="playerRef"
                                :video-url="streamUrl"
                                :poster-url="posterUrl"
                                :video="video"
                                :autoplay="settingsStore.autoplay"
                                :loop="settingsStore.loop"
                                :default-volume="settingsStore.defaultVolume"
                                :start-time="startTime"
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
                                    <NuxtTime :datetime="video.created_at" format="short" />
                                </span>
                            </div>
                        </div>

                        <!-- Detail tabs (Jobs, etc.) -->
                        <WatchDetailTabs />
                    </div>
                </div>

                <!-- Sidebar Metadata (Desktop) -->
                <div class="hidden xl:block">
                    <VideoMetadata :video="video" />
                </div>
            </div>

            <!-- Related Videos -->
            <WatchRelatedVideos v-if="video && !isProcessing && !hasProcessingError" />
        </div>
    </div>
</template>
