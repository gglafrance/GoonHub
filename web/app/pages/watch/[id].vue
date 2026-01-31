<script setup lang="ts">
import type { Video } from '~/types/video';
import type { Marker } from '~/types/marker';
import type { VttCue } from '~/composables/useVttParser';

const route = useRoute();
const router = useRouter();
const { fetchVideo, getResumePosition } = useApi();
const { fetchMarkers } = useApiMarkers();
const settingsStore = useSettingsStore();

const video = ref<Video | null>(null);
const markers = ref<Marker[]>([]);
const isLoading = ref(true);
const error = ref<string | null>(null);

const pageTitle = computed(() => video.value?.title || 'Watch');
useHead({ title: pageTitle });

// Dynamic OG metadata
watch(
    video,
    (v) => {
        if (v) {
            useSeoMeta({
                title: v.title,
                ogTitle: v.title,
                description: v.description || `Watch ${v.title} on GoonHub`,
                ogDescription: v.description || `Watch ${v.title} on GoonHub`,
                ogImage: v.thumbnail_path ? `/thumbnails/${v.id}?size=lg` : undefined,
                ogType: 'video.other',
            });
        }
    },
    { immediate: true },
);
const playerError = ref<unknown>(null);
const playerRef = ref<{
    getCurrentTime: () => number;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    player?: any;
    vttCues?: VttCue[];
} | null>(null);
const resumePosition = ref(0);
const showResumePrompt = ref(false);
const startTime = ref(0);
const forceAutoplay = ref(false);

const thumbnailVersion = ref(0);
const detailsRefreshKey = ref(0);

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

// Ambient glow effect
const playerVttCues = ref<VttCue[]>([]);
const isVideoPlaying = ref(false);
const { glowStyle } = useAmbientGlow(playerRef, playerVttCues, posterUrl, isVideoPlaying);

// Sync vttCues when they become available (loaded asynchronously by VideoPlayer)
watch(
    () => playerRef.value?.vttCues,
    (cues) => {
        if (cues && cues.length > 0) {
            playerVttCues.value = cues;
        }
    },
    { immediate: true },
);

const loadMarkers = async () => {
    if (!video.value) return;
    try {
        const data = await fetchMarkers(video.value.id);
        markers.value = data.markers || [];
    } catch {
        // Silent fail - markers are optional
    }
};

const loadVideo = async () => {
    try {
        isLoading.value = true;
        error.value = null;
        showResumePrompt.value = false;
        resumePosition.value = 0;
        startTime.value = 0;
        markers.value = [];

        video.value = await fetchVideo(videoId.value);

        // Load markers after video is loaded
        await loadMarkers();

        // Check for timestamp query parameter (e.g., ?t=120)
        const queryTime = route.query.t;
        if (queryTime) {
            const timestamp = parseInt(queryTime as string, 10);
            if (!isNaN(timestamp) && timestamp >= 0) {
                startTime.value = timestamp;
                forceAutoplay.value = true;
                // Skip resume prompt when navigating to specific timestamp
                return;
            }
        }
        forceAutoplay.value = false;

        // Fetch resume position (only if no timestamp query param)
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

provide('getPlayerTime', () => playerRef.value?.getCurrentTime() ?? 0);
provide('watchVideo', video);
provide('thumbnailVersion', thumbnailVersion);
provide('detailsRefreshKey', detailsRefreshKey);
provide('seekToTime', (time: number) => {
    startTime.value = time;
    showResumePrompt.value = false;
});
provide('refreshMarkers', loadMarkers);

onMounted(async () => {
    await loadVideo();
});

watch(
    () => route.params.id,
    async () => {
        await loadVideo();
    },
);

// Handle timestamp query parameter changes (e.g., clicking different markers for same video)
watch(
    () => route.query.t,
    (newTime) => {
        if (newTime) {
            const timestamp = parseInt(newTime as string, 10);
            if (!isNaN(timestamp) && timestamp >= 0) {
                startTime.value = timestamp;
                forceAutoplay.value = true;
                showResumePrompt.value = false;
            }
        }
    },
);

definePageMeta({
    middleware: ['auth'],
});
</script>

<template>
    <div class="min-h-screen">
        <!-- Back Navigation Bar -->
        <div
            class="border-border bg-void/10 sticky top-12 z-40 border-b px-4 py-2.5 backdrop-blur-md
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
                            class="player-glow-container relative"
                            :class="{ 'mx-auto max-w-xl': isPortrait }"
                            :style="glowStyle"
                        >
                            <!-- Glow layers -->
                            <div class="player-glow player-glow--primary" aria-hidden="true" />
                            <div class="player-glow player-glow--secondary" aria-hidden="true" />

                            <div
                                class="border-border bg-void relative z-10 overflow-hidden
                                    rounded-xl border"
                            >
                                <!-- Resume Prompt (overlaid on video) -->
                                <WatchResumePrompt
                                    :visible="showResumePrompt"
                                    :resume-position="resumePosition"
                                    :is-playing="isVideoPlaying"
                                    @resume="handleResume"
                                    @start-over="handleStartOver"
                                    @dismiss="showResumePrompt = false"
                                />

                                <VideoPlayer
                                    ref="playerRef"
                                    :video-url="streamUrl"
                                    :poster-url="posterUrl"
                                    :video="video"
                                    :markers="markers"
                                    :autoplay="forceAutoplay || settingsStore.autoplay"
                                    :loop="settingsStore.loop"
                                    :default-volume="settingsStore.defaultVolume"
                                    :start-time="startTime"
                                    @play="isVideoPlaying = true"
                                    @pause="isVideoPlaying = false"
                                    @error="playerError = $event"
                                />
                            </div>
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

<style scoped>
.player-glow-container {
    --glow-color-primary: rgb(255, 77, 77);
    --glow-color-secondary: rgb(255, 45, 45);
    --glow-opacity: 1;
}

.player-glow {
    position: absolute;
    inset: -60px;
    border-radius: 32px;
    pointer-events: none;
    z-index: 0;
    filter: blur(40px);
    transition:
        background 0.3s ease-out,
        opacity 0.3s ease-out;
}

.player-glow--primary {
    background: radial-gradient(
        ellipse 150% 125% at 50% 50%,
        var(--glow-color-primary) 0%,
        transparent 60%
    );
    opacity: var(--glow-opacity);
}

.player-glow--secondary {
    background: radial-gradient(
        ellipse 80% 100% at 30% 70%,
        var(--glow-color-secondary) 0%,
        transparent 50%
    );
    opacity: calc(var(--glow-opacity) * 0.7);
}
</style>
