<script setup lang="ts">
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import type { Video } from '~/types/video';

type Player = ReturnType<typeof videojs>;

const props = defineProps<{
    videoUrl: string;
    posterUrl?: string;
    autoplay?: boolean;
    loop?: boolean;
    defaultVolume?: number;
    video?: Video;
    startTime?: number;
}>();

const emit = defineEmits<{
    play: [];
    pause: [];
    error: [error: unknown];
    viewRecorded: [];
}>();

const videoElement = ref<HTMLVideoElement>();
const player = shallowRef<Player | null>(null);
const { vttCues, loadVttCues } = useVttParser();
const { setup: setupThumbnailPreview } = useThumbnailPreview(player, vttCues);
const { recordWatch } = useApi();
const authStore = useAuthStore();

// Watch tracking state
const hasRecordedView = ref(false);
const cumulativeWatchTime = ref(0);
const lastTimeUpdate = ref(0);
const lastSaveTime = ref(0);
const VIEW_THRESHOLD_SECONDS = 5;
const SAVE_DEBOUNCE_MS = 10000; // Only save every 10 seconds at most
const COMPLETION_THRESHOLD_SECONDS = 5; // Consider completed if within this many seconds of end

const aspectRatio = computed(() => {
    if (props.video?.width && props.video?.height) {
        return `${props.video.width} / ${props.video.height}`;
    }
    return '16 / 9';
});

const isPortrait = computed(() => {
    return props.video?.width && props.video?.height && props.video.height > props.video.width;
});

const vttUrl = computed(() => {
    if (!props.video?.vtt_path) return null;
    const base = `/vtt/${props.video.id}`;
    const v = props.video.updated_at ? new Date(props.video.updated_at).getTime() : '';
    return v ? `${base}?v=${v}` : base;
});

onMounted(async () => {
    if (!videoElement.value) return;

    // Add event listener after validation to prevent memory leak on early return
    window.addEventListener('beforeunload', handleBeforeUnload);

    player.value = videojs(videoElement.value, {
        controls: true,
        autoplay: props.autoplay ? 'any' : false,
        loop: props.loop ?? false,
        preload: 'metadata',
        fill: true,
        playbackRates: [0.5, 0.75, 1, 1.25, 1.5, 2],
        html5: {
            vhs: {
                overrideNative: true,
            },
            nativeAudioTracks: false,
            nativeVideoTracks: false,
        },
        controlBar: {
            children: [
                'playToggle',
                'volumePanel',
                'currentTimeDisplay',
                'timeDivider',
                'durationDisplay',
                'progressControl',
                'remainingTimeDisplay',
                'playbackRateMenuButton',
                'pipToggle',
                'fullscreenToggle',
            ],
        },
    });

    // Set initial volume (video.js uses 0-1 range)
    const volume = props.defaultVolume != null ? props.defaultVolume / 100 : 1;
    player.value.volume(volume);

    player.value.on('play', () => {
        lastTimeUpdate.value = player.value?.currentTime() ?? 0;
        emit('play');
    });
    player.value.on('pause', () => {
        emit('pause');
        // Don't save on every pause - we save on unmount/beforeunload
    });
    player.value.on('error', (e: unknown) => emit('error', e));

    // Track watch time
    player.value.on('timeupdate', () => {
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
            cumulativeWatchTime.value >= VIEW_THRESHOLD_SECONDS &&
            props.video
        ) {
            hasRecordedView.value = true;
            recordViewEvent();
        }
    });

    player.value.on('ended', () => {
        if (props.video) {
            saveProgress(true);
        }
    });

    player.value.ready(() => {
        setupThumbnailPreview();
        if (vttUrl.value) {
            loadVttCues(vttUrl.value);
        }

        // Seek to start time if provided
        if (props.startTime && props.startTime > 0) {
            player.value!.currentTime(props.startTime);
        }

        if (props.autoplay) {
            player.value!.play()?.catch(() => {
                // Autoplay prevented by browser policy
            });
        }
    });
});

// Record view event (after 5 seconds of watch time)
const recordViewEvent = async () => {
    if (!props.video) return;

    try {
        const currentTime = Math.floor(player.value?.currentTime() ?? 0);
        const duration = Math.floor(cumulativeWatchTime.value);
        await recordWatch(props.video.id, duration, currentTime, false);
        emit('viewRecorded');
    } catch {
        // Silently fail - view tracking is not critical
    }
};

// Save progress (debounced to prevent too many API calls)
const saveProgress = async (completed = false, force = false) => {
    if (!props.video || !hasRecordedView.value) return;

    // Debounce saves unless forced (for unmount/beforeunload) or completed
    const now = Date.now();
    if (!force && !completed && now - lastSaveTime.value < SAVE_DEBOUNCE_MS) {
        return;
    }
    lastSaveTime.value = now;

    try {
        const currentTime = Math.floor(player.value?.currentTime() ?? 0);
        const duration = Math.floor(cumulativeWatchTime.value);
        await recordWatch(props.video.id, duration, currentTime, completed);
    } catch {
        // Silently fail
    }
};

// Save progress on page unload using fetch with keepalive (sendBeacon doesn't support auth headers)
const handleBeforeUnload = () => {
    if (!props.video || !hasRecordedView.value) return;

    const currentTime = Math.floor(player.value?.currentTime() ?? 0);
    const duration = Math.floor(cumulativeWatchTime.value);
    const videoDuration = props.video.duration ?? 0;
    const completed =
        videoDuration > 0 && currentTime >= videoDuration - COMPLETION_THRESHOLD_SECONDS;

    // Use fetch with keepalive and credentials to include HTTP-only auth cookie
    fetch(`/api/v1/videos/${props.video.id}/watch`, {
        method: 'POST',
        keepalive: true,
        credentials: 'include', // Send HTTP-only auth cookie
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

watch(
    () => props.videoUrl,
    () => {
        if (player.value) {
            player.value.src({ type: 'video/mp4', src: props.videoUrl });
            if (props.autoplay) {
                player.value.play();
            }
        }
    },
);

watch(vttUrl, (newVttUrl) => {
    if (newVttUrl) {
        loadVttCues(newVttUrl);
    }
});

// Watch for startTime changes (e.g., when user clicks Resume)
watch(
    () => props.startTime,
    (newStartTime) => {
        if (player.value && newStartTime && newStartTime > 0) {
            player.value.currentTime(newStartTime);
            player.value.play()?.catch(() => {
                // Autoplay may be blocked
            });
        }
    },
);

defineExpose({
    getCurrentTime: () => player.value?.currentTime() ?? 0,
});

onBeforeUnmount(() => {
    window.removeEventListener('beforeunload', handleBeforeUnload);
    saveProgress(false, true); // Force save on unmount
    if (player.value) {
        player.value.dispose();
    }
});
</script>

<template>
    <div
        class="video-wrapper"
        :class="{ 'video-wrapper--portrait': isPortrait }"
        :style="{ aspectRatio }"
    >
        <video
            ref="videoElement"
            class="video-js vjs-big-play-centered"
            controls
            :poster="posterUrl"
            crossorigin="anonymous"
        >
            <source :src="videoUrl" type="video/mp4" />
        </video>
    </div>
</template>

<style scoped>
.video-wrapper {
    width: 100%;
    margin: 0 auto;
    background: #050505;
    overflow: hidden;
}

.video-wrapper--portrait {
    max-height: 80vh;
}

:deep(.video-js) {
    font-family: 'Outfit', system-ui, sans-serif;
    --primary-color: #ff4d4d;
    --text-color: #ffffff;
}

:deep(.vjs-big-play-button) {
    background-color: rgba(255, 77, 77, 0.9);
    border: none;
    border-radius: 50%;
    width: 56px;
    height: 56px;
    line-height: 56px;
    margin-left: -28px;
    margin-top: -28px;
    transition: all 0.2s ease;
    box-shadow: 0 0 30px rgba(255, 77, 77, 0.3);
}

:deep(.vjs-big-play-button:hover) {
    background-color: #ff6b6b;
    transform: scale(1.1);
    box-shadow: 0 0 40px rgba(255, 77, 77, 0.5);
}

:deep(.vjs-control-bar) {
    background: linear-gradient(to top, rgba(5, 5, 5, 0.95) 0%, rgba(5, 5, 5, 0) 100%);
    backdrop-filter: blur(8px);
    height: 40px;
    font-size: 11px;
}

:deep(.vjs-progress-control) {
    height: 100%;
}

:deep(.vjs-progress-control .vjs-progress-holder) {
    margin: 0;
    height: 3px;
    padding-top: 18px;
    padding-bottom: 18px;
    background-clip: content-box;
    border-radius: 2px;
}

:deep(.vjs-progress-control .vjs-play-progress),
:deep(.vjs-progress-control .vjs-load-progress) {
    top: 18px;
    height: 3px;
    border-radius: 2px;
}

:deep(.vjs-progress-control .vjs-play-progress::before) {
    top: -4px;
    font-size: 10px;
    color: #ff4d4d;
}

:deep(.vjs-play-progress) {
    background-color: #ff4d4d;
    box-shadow: 0 0 8px rgba(255, 77, 77, 0.5);
}

:deep(.vjs-slider) {
    background-color: rgba(255, 255, 255, 0.1);
    border-radius: 2px;
}

:deep(.vjs-load-progress) {
    background: rgba(255, 255, 255, 0.15);
}

:deep(.vjs-control) {
    color: rgba(255, 255, 255, 0.7);
}

:deep(.vjs-control:hover) {
    color: #ff4d4d;
}

:deep(.vjs-time-control) {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    line-height: 40px;
    padding: 0 4px;
}

:deep(.vjs-playback-rate-value) {
    font-family: 'JetBrains Mono', monospace;
    font-size: 10px;
    line-height: 40px;
}

:deep(.vjs-volume-panel) {
    font-size: 11px;
}

:deep(.vjs-thumb-preview) {
    position: absolute;
    bottom: 100%;
    margin-bottom: 6px;
    pointer-events: none;
    border: 1px solid rgba(255, 77, 77, 0.5);
    border-radius: 4px;
    overflow: hidden;
    box-shadow:
        0 4px 20px rgba(0, 0, 0, 0.8),
        0 0 15px rgba(255, 77, 77, 0.15);
    z-index: 10;
    background: #050505;
}

:deep(.vjs-thumb-preview img) {
    display: block;
}
</style>
