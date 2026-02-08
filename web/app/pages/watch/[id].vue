<script setup lang="ts">
import type { VttCue } from '~/composables/useVttParser';
import { WATCH_PAGE_DATA_KEY } from '~/composables/useWatchPageData';

const route = useRoute();
const router = useRouter();
const settingsStore = useSettingsStore();

const sceneId = computed(() => parseInt(route.params.id as string));

// Playlist mode
const playlistUuid = computed(() => route.query.playlist as string | undefined);
const playlistPos = computed(() => {
    const pos = route.query.pos;
    return pos ? parseInt(pos as string, 10) : 0;
});
const playlistShuffleSeed = computed(() => {
    const s = route.query.shuffle as string | undefined;
    return s ? parseInt(s, 10) : 0;
});
const isPlaylistMode = computed(() => !!playlistUuid.value);
const playlistPlayer = usePlaylistPlayer();

// Centralized data loading with priority tiers
const watchPageData = useWatchPageData(sceneId);
const { scene, markers, resumePosition, loading, error, refreshMarkers } = watchPageData;

// Provide the entire data object to child components
provide(WATCH_PAGE_DATA_KEY, watchPageData);

// Legacy provides for backwards compatibility during migration
provide('watchScene', scene);
provide('refreshMarkers', refreshMarkers);

const pageTitle = computed(() => scene.value?.title || 'Watch');
useHead({ title: pageTitle });

// Dynamic OG metadata
watch(
    scene,
    (s) => {
        if (s) {
            useSeoMeta({
                title: s.title,
                ogTitle: s.title,
                description: s.description || `Watch ${s.title} on GoonHub`,
                ogDescription: s.description || `Watch ${s.title} on GoonHub`,
                ogImage: s.thumbnail_path ? `/thumbnails/${s.id}?size=lg` : undefined,
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
const showResumePrompt = ref(false);
const startTime = ref(0);
const forceAutoplay = ref(false);

const thumbnailVersion = ref(0);
const detailsRefreshKey = ref(0);

// Keyboard shortcuts for scene player
useScenePlayerShortcuts({
    player: computed(() => playerRef.value?.player ?? null),
    scene: scene,
    onTheaterModeToggle: () => settingsStore.toggleTheaterMode(),
    onABLoopToggle: () => playerRef.value?.abLoop?.toggle(),
});

const isProcessing = computed(() => scene.value?.processing_status === 'pending');
const hasProcessingError = computed(() => (scene.value ? hasSceneError(scene.value) : false));
const isCorrupted = computed(() => (scene.value ? isSceneCorrupted(scene.value) : false));
const isPortrait = computed(() => {
    return scene.value?.width && scene.value?.height && scene.value.height > scene.value.width;
});

const playerAspectRatio = computed(() => {
    if (scene.value?.width && scene.value?.height) {
        return `${scene.value.width} / ${scene.value.height}`;
    }
    return '16 / 9';
});

const streamUrl = computed(() => {
    if (!scene.value) return '';
    return `/api/v1/scenes/${scene.value.id}/stream`;
});

const posterUrl = computed(() => {
    if (!scene.value || !scene.value.thumbnail_path) return '';
    const base = `/thumbnails/${scene.value.id}?size=lg`;
    const v =
        thumbnailVersion.value ||
        (scene.value.updated_at ? new Date(scene.value.updated_at).getTime() : 0);
    return v ? `${base}&v=${v}` : base;
});

// Ambient glow effect
const playerVttCues = ref<VttCue[]>([]);
const isScenePlaying = ref(false);
const { glowStyle } = useAmbientGlow(playerRef, playerVttCues, posterUrl, isScenePlaying);

// Sync vttCues when they become available (loaded asynchronously by ScenePlayer)
watch(
    () => playerRef.value?.vttCues,
    (cues) => {
        if (cues && cues.length > 0) {
            playerVttCues.value = cues;
        }
    },
    { immediate: true },
);

// Handle resume prompt based on centralized data
watch(
    resumePosition,
    (position) => {
        // Only show prompt if we have a position and no timestamp query param
        if (position > 0 && !route.query.t) {
            showResumePrompt.value = true;
        }
    },
    { immediate: true },
);

// Load data on mount and when scene ID changes
async function loadPage() {
    // Reset UI state
    showResumePrompt.value = false;
    startTime.value = 0;
    forceAutoplay.value = false;

    // Load all data via centralized composable
    await watchPageData.refreshAll();

    // Load playlist data if in playlist mode
    if (playlistUuid.value && !playlistPlayer.playlist.value) {
        await playlistPlayer.loadPlaylist(playlistUuid.value, playlistPos.value);
        if (playlistShuffleSeed.value) {
            playlistPlayer.shuffleWithSeed(playlistShuffleSeed.value, playlistPos.value);
        }
    }

    // Check for timestamp query parameter (e.g., ?t=120)
    const queryTime = route.query.t;
    if (queryTime) {
        const timestamp = parseInt(queryTime as string, 10);
        if (!isNaN(timestamp) && timestamp >= 0) {
            startTime.value = timestamp;
            forceAutoplay.value = true;
            showResumePrompt.value = false;
        }
    }
}

// Playlist navigation helpers
const navigateToPlaylistScene = (entry: { scene: { id: number } } | null, index: number) => {
    if (!entry || !playlistUuid.value) return;
    const query: Record<string, string> = { playlist: playlistUuid.value, pos: String(index) };
    if (playlistPlayer.shuffleSeed.value) {
        query.shuffle = String(playlistPlayer.shuffleSeed.value);
    }
    router.push({ path: `/watch/${entry.scene.id}`, query });
};

// Register callback for countdown-triggered navigation
playlistPlayer.onNavigate((entry, index) => {
    navigateToPlaylistScene(entry, index);
});

const handlePlaylistSceneEnd = () => {
    if (!isPlaylistMode.value) return;
    const next = playlistPlayer.onSceneEnd();
    if (next) {
        navigateToPlaylistScene(next, playlistPlayer.currentIndex.value);
    }
};

const handlePlaylistGoToScene = (orderIndex: number) => {
    const entry = playlistPlayer.goToScene(orderIndex);
    if (entry) {
        navigateToPlaylistScene(entry, orderIndex);
    }
};

const handlePlaylistNext = () => {
    const entry = playlistPlayer.goToNext();
    if (entry) {
        navigateToPlaylistScene(entry, playlistPlayer.currentIndex.value);
    }
};

const handlePlaylistPrevious = () => {
    const entry = playlistPlayer.goToPrevious();
    if (entry) {
        navigateToPlaylistScene(entry, playlistPlayer.currentIndex.value);
    }
};

const handlePlaylistCountdownPlayNow = () => {
    playlistPlayer.cancelCountdown();
    handlePlaylistNext();
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
    if (playlistUuid.value) {
        router.push(`/playlists/${playlistUuid.value}`);
    } else {
        router.push('/');
    }
};

// Tab state for DetailTabs (allows switching tabs from keyboard shortcuts)
const activeTab = ref<'jobs' | 'thumbnail' | 'details' | 'history' | 'markers'>('details');
const pendingMarkerAdd = ref(false);

provide('getPlayerTime', () => playerRef.value?.getCurrentTime() ?? 0);
provide('thumbnailVersion', thumbnailVersion);
provide('detailsRefreshKey', detailsRefreshKey);
provide('seekToTime', (time: number) => {
    startTime.value = time;
    showResumePrompt.value = false;
});
provide('activeTab', activeTab);
provide('pendingMarkerAdd', pendingMarkerAdd);

// Handle 'M' key globally to add markers (even when Markers tab is not open)
const handleMarkerShortcut = (e: KeyboardEvent) => {
    const target = e.target as HTMLElement;
    if (
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable ||
        e.ctrlKey ||
        e.metaKey ||
        e.altKey
    ) {
        return;
    }

    if (e.key === 'm' || e.key === 'M') {
        e.preventDefault();
        // Switch to markers tab and trigger marker add
        activeTab.value = 'markers';
        pendingMarkerAdd.value = true;
    }
};

onMounted(() => {
    window.addEventListener('keydown', handleMarkerShortcut);
    loadPage();
});

onUnmounted(() => {
    window.removeEventListener('keydown', handleMarkerShortcut);
});

watch(
    () => route.params.id,
    () => {
        loadPage();
    },
);

// Handle timestamp query parameter changes (e.g., clicking different markers for same scene)
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
                    <span class="text-xs font-medium">{{
                        isPlaylistMode ? 'Playlist' : 'Library'
                    }}</span>
                </button>

                <div v-if="scene" class="text-dim hidden truncate text-xs sm:block">
                    {{ scene.title }}
                </div>
            </div>
        </div>

        <div class="mx-auto max-w-415 p-4 sm:px-5 lg:py-6">
            <!-- Loading State -->
            <div v-if="loading.scene" class="flex h-[70vh] items-center justify-center">
                <LoadingSpinner label="Loading..." />
            </div>

            <!-- Corrupted State -->
            <div
                v-else-if="isCorrupted"
                class="flex h-[70vh] flex-col items-center justify-center text-center"
            >
                <div
                    class="flex h-12 w-12 items-center justify-center rounded-xl border
                        border-amber-500/20 bg-amber-500/5"
                >
                    <Icon name="heroicons:shield-exclamation" size="24" class="text-amber-400" />
                </div>
                <h2 class="mt-4 text-lg font-semibold text-white">Corrupted Video</h2>
                <p class="text-dim mt-1 text-xs">
                    This video file failed integrity checks and cannot be played. Try replacing the
                    file and reprocessing.
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
                <h2 class="mt-4 text-lg font-semibold text-white">Scene Unavailable</h2>
                <p class="text-dim mt-1 text-xs">
                    {{ error || 'Scene processing failed. Please try reprocessing.' }}
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

            <!-- Scene Player & Content -->
            <div
                v-else-if="scene"
                :class="['grid gap-5', settingsStore.theaterMode ? '' : 'xl:grid-cols-[1fr_280px]']"
            >
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
                                :style="{ aspectRatio: playerAspectRatio }"
                            >
                                <!-- Resume Prompt (overlaid on scene player) -->
                                <WatchResumePrompt
                                    :visible="showResumePrompt"
                                    :resume-position="resumePosition"
                                    :is-playing="isScenePlaying"
                                    @resume="handleResume"
                                    @start-over="handleStartOver"
                                    @dismiss="showResumePrompt = false"
                                />

                                <LazyScenePlayer
                                    ref="playerRef"
                                    :scene-url="streamUrl"
                                    :poster-url="posterUrl"
                                    :scene="scene"
                                    :markers="markers"
                                    :autoplay="
                                        isPlaylistMode || forceAutoplay || settingsStore.autoplay
                                    "
                                    :loop="isPlaylistMode ? false : settingsStore.loop"
                                    :default-volume="settingsStore.defaultVolume"
                                    :start-time="startTime"
                                    :playlist-mode="isPlaylistMode"
                                    :has-next="playlistPlayer.hasNext.value"
                                    :has-previous="playlistPlayer.hasPrevious.value"
                                    @play="isScenePlaying = true"
                                    @pause="isScenePlaying = false"
                                    @error="playerError = $event"
                                    @ended="handlePlaylistSceneEnd"
                                    @next="handlePlaylistNext"
                                    @previous="handlePlaylistPrevious"
                                />
                            </div>
                        </div>

                        <!-- Mobile Metadata -->
                        <div class="block xl:hidden">
                            <h1 class="text-sm font-semibold text-white">
                                {{ scene.title }}
                            </h1>
                            <div class="text-dim mt-2 flex flex-wrap gap-3 font-mono text-[11px]">
                                <span class="flex items-center gap-1">
                                    <Icon name="heroicons:eye" size="12" class="text-lava" />
                                    {{ scene.view_count }} views
                                </span>
                                <span class="flex items-center gap-1">
                                    <Icon name="heroicons:calendar" size="12" class="text-lava" />
                                    <NuxtTime :datetime="scene.created_at" format="short" />
                                </span>
                            </div>
                        </div>

                        <!-- Detail tabs (Jobs, etc.) -->
                        <WatchDetailTabs />
                    </div>
                </div>

                <!-- Sidebar: Playlist or Metadata (Desktop) -->
                <div v-if="!settingsStore.theaterMode" class="hidden xl:block">
                    <WatchPlaylistSidebar
                        v-if="isPlaylistMode && playlistPlayer.playlist.value"
                        :playlist="playlistPlayer.playlist.value"
                        :current-index="playlistPlayer.currentIndex.value"
                        :effective-order="playlistPlayer.effectiveOrder.value"
                        :is-shuffled="playlistPlayer.isShuffled.value"
                        @go-to-scene="handlePlaylistGoToScene"
                        @shuffle="playlistPlayer.shuffle()"
                        @unshuffle="playlistPlayer.unshuffle()"
                    />
                    <SceneMetadata v-else :scene="scene" />
                </div>
            </div>

            <!-- Related Scenes -->
            <WatchRelatedScenes v-if="scene && !isProcessing && !hasProcessingError" />
        </div>

        <!-- Playlist Mobile Mini-Bar -->
        <WatchPlaylistMobileBar
            v-if="isPlaylistMode && playlistPlayer.playlist.value && scene && !isProcessing"
            :playlist="playlistPlayer.playlist.value"
            :current-index="playlistPlayer.currentIndex.value"
            :effective-order="playlistPlayer.effectiveOrder.value"
            :is-shuffled="playlistPlayer.isShuffled.value"
            :has-next="playlistPlayer.hasNext.value"
            :has-previous="playlistPlayer.hasPrevious.value"
            :countdown-visible="playlistPlayer.showCountdown.value"
            @go-to-scene="handlePlaylistGoToScene"
            @next="handlePlaylistNext"
            @previous="handlePlaylistPrevious"
            @shuffle="playlistPlayer.shuffle()"
            @unshuffle="playlistPlayer.unshuffle()"
        />

        <!-- Playlist Auto-Advance Overlay -->
        <WatchPlaylistAutoAdvance
            v-if="isPlaylistMode"
            :visible="playlistPlayer.showCountdown.value"
            :next-scene="playlistPlayer.nextScene.value"
            :countdown-remaining="playlistPlayer.countdownRemaining.value"
            @play-now="handlePlaylistCountdownPlayNow"
            @cancel="playlistPlayer.cancelCountdown()"
        />
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
