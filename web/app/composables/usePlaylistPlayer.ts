import type { PlaylistDetail } from '~/types/playlist';
import type { PlaylistAutoAdvance } from '~/types/settings';
import type { PlaylistQueueItem } from '~/utils/playlistQueue';

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
        const a = indices[i];
        const b = indices[j];
        if (a === undefined || b === undefined) continue;
        indices[i] = b;
        indices[j] = a;
    }
    return indices;
}

// Play order of a playlist (scenes or markers, see utils/playlistQueue): current item,
// next/previous, shuffle and the auto-advance countdown
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
    let onNavigateCallback: ((entry: PlaylistQueueItem, index: number) => void) | null = null;

    // Queue items in playlist order (scenes, or markers with their range)
    const items = computed<PlaylistQueueItem[]>(() =>
        playlist.value ? playlistQueueItems(playlist.value) : [],
    );
    const isMarkerPlaylist = computed(() => playlist.value?.kind === 'markers');

    const effectiveOrder = computed<number[]>(() => {
        if (!playlist.value) return [];
        const len = items.value.length;
        if (isShuffled.value && shuffleOrder.value.length === len) {
            return shuffleOrder.value;
        }
        return Array.from({ length: len }, (_, i) => i);
    });

    const itemAt = (orderIndex: number): PlaylistQueueItem | null => {
        const idx = effectiveOrder.value[orderIndex];
        if (idx === undefined) return null;
        return items.value[idx] ?? null;
    };

    const currentItem = computed<PlaylistQueueItem | null>(() => itemAt(currentIndex.value));
    const nextItem = computed<PlaylistQueueItem | null>(() => itemAt(currentIndex.value + 1));
    const previousItem = computed<PlaylistQueueItem | null>(() =>
        currentIndex.value > 0 ? itemAt(currentIndex.value - 1) : null,
    );

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

    const goToNext = (): PlaylistQueueItem | null => {
        if (!hasNext.value) return null;
        currentIndex.value++;
        cancelCountdown();
        return currentItem.value;
    };

    const goToPrevious = (): PlaylistQueueItem | null => {
        if (!hasPrevious.value) return null;
        currentIndex.value--;
        cancelCountdown();
        return currentItem.value;
    };

    const goToItem = (index: number): PlaylistQueueItem | null => {
        if (!playlist.value || index < 0 || index >= effectiveOrder.value.length) return null;
        currentIndex.value = index;
        cancelCountdown();
        return currentItem.value;
    };

    const shuffleSeed = ref<number>(0);

    const shuffleWithSeed = (seed: number, startIndex = 0) => {
        if (!playlist.value) return;
        shuffleSeed.value = seed;
        shuffleOrder.value = seededShuffle(items.value.length, seed);
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

    // End of the current item. A scene follows the auto-advance setting; a marker goes on to
    // the next one at once (a countdown between short clips would break the compilation),
    // unless the setting is manual.
    const onSceneEnd = (): PlaylistQueueItem | null => {
        if (!hasNext.value) return null;

        if (isMarkerPlaylist.value) {
            return autoAdvanceMode.value === 'manual' ? null : goToNext();
        }

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

    const onNavigate = (cb: (entry: PlaylistQueueItem, index: number) => void) => {
        onNavigateCallback = cb;
    };

    // Resume point of the playlist: the item played (its marker in a marker playlist) and the
    // time in its scene
    const saveProgress = async (item: PlaylistQueueItem, positionS: number) => {
        if (!playlist.value) return;
        try {
            await api.updateProgress(
                playlist.value.uuid,
                item.sceneId,
                Math.max(0, positionS),
                item.markerId ?? undefined,
            );
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
        items,
        isMarkerPlaylist,
        currentItem,
        nextItem,
        previousItem,
        hasNext,
        hasPrevious,
        effectiveOrder,
        loadPlaylist,
        goToNext,
        goToPrevious,
        goToItem,
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
