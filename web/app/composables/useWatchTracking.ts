import type { ShallowRef } from 'vue';
import type videojs from 'video.js';
import type { Video } from '~/types/video';

type Player = ReturnType<typeof videojs>;

interface WatchTrackingOptions {
    player: ShallowRef<Player | null>;
    video: Ref<Video | undefined>;
    viewThresholdSeconds?: number;
    saveDebounceMs?: number;
    completionThresholdSeconds?: number;
}

/**
 * Composable for tracking video watch time and recording views.
 */
export const useWatchTracking = (options: WatchTrackingOptions) => {
    const {
        player,
        video,
        viewThresholdSeconds = 5,
        saveDebounceMs = 10000,
        completionThresholdSeconds = 5,
    } = options;

    const { recordWatch } = useApiVideos();

    const hasRecordedView = ref(false);
    const cumulativeWatchTime = ref(0);
    const lastTimeUpdate = ref(0);
    const lastSaveTime = ref(0);

    const onTimeUpdate = () => {
        const currentTime = player.value?.currentTime() ?? 0;
        const delta = currentTime - lastTimeUpdate.value;

        // Only count forward progress (not seeking backwards)
        if (delta > 0 && delta < 2) {
            cumulativeWatchTime.value += delta;
        }
        lastTimeUpdate.value = currentTime;

        // Record view after threshold
        if (
            !hasRecordedView.value &&
            cumulativeWatchTime.value >= viewThresholdSeconds &&
            video.value
        ) {
            hasRecordedView.value = true;
            recordViewEvent();
        }
    };

    const onPlay = () => {
        lastTimeUpdate.value = player.value?.currentTime() ?? 0;
    };

    const recordViewEvent = async () => {
        if (!video.value) return;

        try {
            const currentTime = Math.floor(player.value?.currentTime() ?? 0);
            const duration = Math.floor(cumulativeWatchTime.value);
            await recordWatch(video.value.id, duration, currentTime, false);
        } catch {
            // Silently fail - view tracking is not critical
        }
    };

    const saveProgress = async (completed = false, force = false) => {
        if (!video.value || !hasRecordedView.value) return;

        // Debounce saves unless forced (for unmount/beforeunload) or completed
        const now = Date.now();
        if (!force && !completed && now - lastSaveTime.value < saveDebounceMs) {
            return;
        }
        lastSaveTime.value = now;

        try {
            const currentTime = Math.floor(player.value?.currentTime() ?? 0);
            const duration = Math.floor(cumulativeWatchTime.value);
            await recordWatch(video.value.id, duration, currentTime, completed);
        } catch {
            // Silently fail
        }
    };

    const handleBeforeUnload = () => {
        if (!video.value || !hasRecordedView.value) return;

        const currentTime = Math.floor(player.value?.currentTime() ?? 0);
        const duration = Math.floor(cumulativeWatchTime.value);
        const videoDuration = video.value.duration ?? 0;
        const completed =
            videoDuration > 0 && currentTime >= videoDuration - completionThresholdSeconds;

        // Use fetch with keepalive and credentials to include HTTP-only auth cookie
        fetch(`/api/v1/videos/${video.value.id}/watch`, {
            method: 'POST',
            keepalive: true,
            credentials: 'include',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                duration,
                position: currentTime,
                completed,
            }),
        }).catch(() => {
            // Silently fail - page is unloading anyway
        });
    };

    const setupTracking = () => {
        if (!player.value) return;

        player.value.on('play', onPlay);
        player.value.on('timeupdate', onTimeUpdate);
        player.value.on('ended', () => {
            if (video.value) {
                saveProgress(true);
            }
        });

        window.addEventListener('beforeunload', handleBeforeUnload);
    };

    const cleanup = () => {
        window.removeEventListener('beforeunload', handleBeforeUnload);
        saveProgress(false, true);
    };

    return {
        hasRecordedView,
        cumulativeWatchTime,
        setupTracking,
        cleanup,
        saveProgress,
    };
};
