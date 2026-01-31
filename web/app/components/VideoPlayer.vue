<script setup lang="ts">
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import type { Video } from '~/types/video';
import type { Marker } from '~/types/marker';

type Player = ReturnType<typeof videojs>;

const props = defineProps<{
    videoUrl: string;
    posterUrl?: string;
    autoplay?: boolean;
    loop?: boolean;
    defaultVolume?: number;
    video?: Video;
    startTime?: number;
    markers?: Marker[];
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
const {
    setup: setupMarkerIndicators,
    cleanup: cleanupMarkerIndicators,
    update: updateMarkerIndicators,
} = useMarkerIndicators(
    player,
    computed(() => props.markers ?? []),
);

const videoRef = computed(() => props.video);
const { hasRecordedView, setupTracking, cleanup } = useWatchTracking({
    player,
    video: videoRef,
});

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

    player.value.on('play', () => emit('play'));
    player.value.on('pause', () => emit('pause'));
    player.value.on('error', (e: unknown) => emit('error', e));

    // Set up watch tracking (handles timeupdate, ended, and beforeunload)
    setupTracking();

    // Emit viewRecorded when first recorded
    watch(hasRecordedView, (recorded) => {
        if (recorded) emit('viewRecorded');
    });

    player.value.ready(() => {
        setupThumbnailPreview();
        setupMarkerIndicators();
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

// Watch for marker changes
watch(
    () => props.markers,
    () => {
        updateMarkerIndicators();
    },
    { deep: true },
);

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
    cleanup();
    cleanupMarkerIndicators();
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
    margin-bottom: 28px;
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

/* Marker tick container */
:deep(.vjs-marker-container) {
    position: absolute;
    top: 18px;
    left: 0;
    right: 0;
    height: 3px;
    pointer-events: none;
    z-index: 5;
}

/* Individual marker tick - larger hover zone with smaller visual dot */
:deep(.vjs-marker-tick) {
    position: absolute;
    width: 21px;
    height: 21px;
    transform: translate(-50%, -50%);
    top: 50%;
    cursor: pointer;
    pointer-events: auto;
}

:deep(.vjs-marker-tick::before) {
    content: '';
    position: absolute;
    top: 50%;
    left: 50%;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    transform: translate(-50%, -50%);
    background-color: var(--marker-color, #ffffff);
    box-shadow: 0 0 4px rgba(0, 0, 0, 0.5);
    transition:
        transform 0.15s ease,
        box-shadow 0.15s ease;
}

:deep(.vjs-marker-tick:hover::before) {
    transform: translate(-50%, -50%) scale(1.5);
    box-shadow: 0 0 8px var(--marker-color, #ffffff);
}

/* Tooltip container - matches sprite preview style */
:deep(.vjs-marker-tooltip) {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 28px;
    background: #050505;
    border: 1px solid rgba(255, 77, 77, 0.5);
    border-radius: 4px;
    overflow: hidden;
    width: 320px;
    opacity: 0;
    visibility: hidden;
    transition:
        opacity 0.15s ease,
        visibility 0.15s ease;
    pointer-events: none;
    z-index: 20;
    box-shadow:
        0 4px 20px rgba(0, 0, 0, 0.8),
        0 0 15px rgba(255, 77, 77, 0.15);
}

:deep(.vjs-marker-tick:hover .vjs-marker-tooltip) {
    opacity: 1;
    visibility: visible;
}

/* Tooltip thumbnail */
:deep(.vjs-marker-tooltip-img) {
    width: 320px;
    height: 180px;
    object-fit: cover;
    display: block;
}

/* Tooltip placeholder when no thumbnail */
:deep(.vjs-marker-tooltip-placeholder) {
    width: 320px;
    height: 180px;
    background: linear-gradient(135deg, rgba(255, 77, 77, 0.1) 0%, rgba(255, 77, 77, 0.05) 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

/* Tooltip info container */
:deep(.vjs-marker-tooltip-info) {
    padding: 6px 8px;
    background: linear-gradient(to bottom, rgba(255, 77, 77, 0.05), transparent);
}

/* Tooltip label */
:deep(.vjs-marker-tooltip-label) {
    font-size: 11px;
    color: white;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-bottom: 2px;
}

/* Tooltip timestamp */
:deep(.vjs-marker-tooltip-time) {
    font-size: 10px;
    font-family: 'JetBrains Mono', monospace;
    color: rgba(255, 255, 255, 0.6);
}

:deep(.vjs-icon-placeholder) {
    transform: translateY(-15px);
}

:deep(.vjs-volume-control) {
    transform: translateY(5px);
}
</style>
