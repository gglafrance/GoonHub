import type { PlaylistDetail, PlaylistSceneEntry } from '~/types/playlist';
import type { PlaylistAutoAdvance } from '~/types/settings';

// Mulberry32 seeded PRNG — deterministic random from a 32-bit seed
function mulberry32(seed: number): () => number {
    return () => {
        seed |= 0;
        seed = (seed + 0x6d2b79f5) | 0;
        let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
        t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
        return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
}

export function seededShuffle(length: number, seed: number): number[] {
    const rng = mulberry32(seed);
    const indices = Array.from({ length }, (_, i) => i);
    for (let i = indices.length - 1; i > 0; i--) {
        const j = Math.floor(rng() * (i + 1));
        [indices[i], indices[j]] = [indices[j], indices[i]];
    }
    return indices;
}

export const usePlaylistPlayer = () => {
    const settingsStore = useSettingsStore();
    const api = useApiPlaylists();

    const playlist = ref<PlaylistDetail | null>(null);
    const currentIndex = ref(0);
    const isShuffled = ref(false);
    const shuffleOrder = ref<number[]>([]);
    const showCountdown = ref(false);
    const countdownRemaining = ref(0);
    const isLoading = ref(false);

    let countdownTimer: ReturnType<typeof setInterval> | null = null;
    let onNavigateCallback: ((entry: PlaylistSceneEntry, index: number) => void) | null = null;

    const effectiveOrder = computed<number[]>(() => {
        if (!playlist.value) return [];
        const len = playlist.value.scenes.length;
        if (isShuffled.value && shuffleOrder.value.length === len) {
            return shuffleOrder.value;
        }
        return Array.from({ length: len }, (_, i) => i);
    });

    const currentScene = computed<PlaylistSceneEntry | null>(() => {
        if (!playlist.value || effectiveOrder.value.length === 0) return null;
        const idx = effectiveOrder.value[currentIndex.value];
        return playlist.value.scenes[idx] ?? null;
    });

    const nextScene = computed<PlaylistSceneEntry | null>(() => {
        if (!playlist.value || currentIndex.value >= effectiveOrder.value.length - 1) return null;
        const idx = effectiveOrder.value[currentIndex.value + 1];
        return playlist.value.scenes[idx] ?? null;
    });

    const previousScene = computed<PlaylistSceneEntry | null>(() => {
        if (!playlist.value || currentIndex.value <= 0) return null;
        const idx = effectiveOrder.value[currentIndex.value - 1];
        return playlist.value.scenes[idx] ?? null;
    });

    const hasNext = computed(() => currentIndex.value < effectiveOrder.value.length - 1);
    const hasPrevious = computed(() => currentIndex.value > 0);

    const autoAdvanceMode = computed<PlaylistAutoAdvance>(
        () => settingsStore.settings?.playlist_auto_advance ?? 'countdown',
    );
    const countdownSeconds = computed(
        () => settingsStore.settings?.playlist_countdown_seconds ?? 5,
    );

    const loadPlaylist = async (uuid: string, startIndex = 0) => {
        isLoading.value = true;
        try {
            const detail = await api.fetchPlaylist(uuid);
            playlist.value = detail;
            currentIndex.value = startIndex;
            isShuffled.value = false;
            shuffleOrder.value = [];
        } finally {
            isLoading.value = false;
        }
    };

    const goToNext = (): PlaylistSceneEntry | null => {
        if (!hasNext.value) return null;
        currentIndex.value++;
        cancelCountdown();
        return currentScene.value;
    };

    const goToPrevious = (): PlaylistSceneEntry | null => {
        if (!hasPrevious.value) return null;
        currentIndex.value--;
        cancelCountdown();
        return currentScene.value;
    };

    const goToScene = (index: number): PlaylistSceneEntry | null => {
        if (!playlist.value || index < 0 || index >= effectiveOrder.value.length) return null;
        currentIndex.value = index;
        cancelCountdown();
        return currentScene.value;
    };

    const shuffleSeed = ref<number>(0);

    const shuffleWithSeed = (seed: number, startIndex = 0) => {
        if (!playlist.value) return;
        shuffleSeed.value = seed;
        shuffleOrder.value = seededShuffle(playlist.value.scenes.length, seed);
        isShuffled.value = true;
        currentIndex.value = startIndex;
    };

    const shuffle = () => {
        const seed = Math.floor(Math.random() * 0xffffffff);
        shuffleWithSeed(seed);
    };

    const unshuffle = () => {
        isShuffled.value = false;
        shuffleOrder.value = [];
        shuffleSeed.value = 0;
    };

    const startCountdown = () => {
        if (!hasNext.value) return;
        cancelCountdown();
        countdownRemaining.value = countdownSeconds.value;
        showCountdown.value = true;

        countdownTimer = setInterval(() => {
            countdownRemaining.value--;
            if (countdownRemaining.value <= 0) {
                cancelCountdown();
                const entry = goToNext();
                if (entry && onNavigateCallback) {
                    onNavigateCallback(entry, currentIndex.value);
                }
            }
        }, 1000);
    };

    const cancelCountdown = () => {
        showCountdown.value = false;
        countdownRemaining.value = 0;
        if (countdownTimer) {
            clearInterval(countdownTimer);
            countdownTimer = null;
        }
    };

    const onSceneEnd = (): PlaylistSceneEntry | null => {
        if (!hasNext.value) return null;

        switch (autoAdvanceMode.value) {
            case 'instant':
                return goToNext();
            case 'countdown':
                startCountdown();
                return null;
            case 'manual':
            default:
                return null;
        }
    };

    const onNavigate = (cb: (entry: PlaylistSceneEntry, index: number) => void) => {
        onNavigateCallback = cb;
    };

    const saveProgress = async (sceneId: number, positionS: number) => {
        if (!playlist.value) return;
        try {
            await api.updateProgress(playlist.value.uuid, sceneId, positionS);
        } catch {
            // Non-critical
        }
    };

    onUnmounted(() => {
        cancelCountdown();
    });

    return {
        playlist,
        currentIndex,
        isShuffled,
        shuffleSeed,
        showCountdown,
        countdownRemaining,
        isLoading,
        currentScene,
        nextScene,
        previousScene,
        hasNext,
        hasPrevious,
        effectiveOrder,
        loadPlaylist,
        goToNext,
        goToPrevious,
        goToScene,
        shuffle,
        shuffleWithSeed,
        unshuffle,
        startCountdown,
        cancelCountdown,
        onSceneEnd,
        onNavigate,
        saveProgress,
    };
};
