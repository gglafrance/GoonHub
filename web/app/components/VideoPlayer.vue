<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch, computed } from 'vue';
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import type { Video } from '~/types/video';

const props = defineProps<{
    videoUrl: string;
    posterUrl?: string;
    autoplay?: boolean;
    video?: Video;
}>();

const emit = defineEmits<{
    play: [];
    pause: [];
    error: [error: any];
}>();

const videoElement = ref<HTMLVideoElement>();
const previewContainer = ref<HTMLElement>();
const previewThumbnail = ref<HTMLImageElement>();
const showPreview = ref(false);
const previewX = ref(0);

let player: any = null;
let framesCache: HTMLImageElement[] = [];

const canShowPreview = computed(() => {
    return (
        props.video &&
        props.video.frame_paths &&
        props.video.frame_count &&
        props.video.frame_count > 0 &&
        props.video.frame_interval
    );
});

const loadFrames = async () => {
    if (!props.video || !canShowPreview.value) return [];

    const framePaths = props.video.frame_paths!.split(',');
    const frameImages: HTMLImageElement[] = [];

    for (let i = 0; i < framePaths.length; i++) {
        const img = new Image();
        img.src = `/frames/${props.video.id}/${framePaths[i]}`;
        frameImages[i] = img;
    }

    return frameImages;
};

const getFrameByTimestamp = (
    timestamp: number,
    frames: HTMLImageElement[],
): HTMLImageElement | null => {
    if (!props.video?.frame_interval || frames.length === 0) return null;

    const index = Math.min(Math.floor(timestamp / props.video.frame_interval!), frames.length - 1);

    return frames[index] || null;
};

const handleProgressMouseMove = (event: MouseEvent) => {
    if (!previewContainer.value || !player || !canShowPreview.value) return;

    const rect = previewContainer.value.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const percent = Math.max(0, Math.min(1, x / rect.width));

    previewX.value = x;
    showPreview.value = true;

    if (framesCache.length === 0) {
        loadFrames().then((frames) => {
            framesCache = frames;
            updatePreview(percent);
        });
    } else {
        updatePreview(percent);
    }
};

const updatePreview = (percent: number) => {
    if (!player || framesCache.length === 0 || !previewThumbnail.value) return;

    const duration = player.duration();
    const timestamp = percent * duration;
    const frame = getFrameByTimestamp(timestamp, framesCache);

    if (frame) {
        previewThumbnail.value.src = frame.src;
    }
};

const handleProgressMouseLeave = () => {
    showPreview.value = false;
};

const initPlayer = () => {
    if (!videoElement.value || !props.videoUrl) return;

    if (player) {
        player.dispose();
    }

    player = videojs(videoElement.value, {
        controls: true,
        autoplay: props.autoplay || false,
        preload: 'auto',
        fluid: true,
        responsive: true,
        playbackRates: [0.5, 0.75, 1, 1.25, 1.5, 2],
        userActions: {
            doubleClick: true,
            hotkeys: true,
        },
        sources: [
            {
                src: props.videoUrl,
                type: 'video/mp4',
            },
        ],
        poster: props.posterUrl,
    });

    player.on('play', () => emit('play'));
    player.on('pause', () => emit('pause'));
    player.on('error', () => emit('error', player?.error()));

    player.ready(() => {
        const progressControl = player.controlBar.progressControl;

        if (progressControl && progressControl.el()) {
            const seekBar = progressControl.el().querySelector('.vjs-progress-holder');

            if (seekBar) {
                seekBar.addEventListener('mousemove', handleProgressMouseMove);
                seekBar.addEventListener('mouseleave', handleProgressMouseLeave);
                seekBar.addEventListener('click', handleProgressMouseLeave);
            }
        }
    });
};

onMounted(() => {
    if (props.videoUrl) {
        initPlayer();
    }
});

watch(
    () => props.videoUrl,
    (newUrl) => {
        if (newUrl) {
            initPlayer();
        }
    },
    { immediate: true },
);

watch(
    () => props.video?.id,
    () => {
        framesCache = [];
    },
);

onUnmounted(() => {
    if (player) {
        player.dispose();
        player = null;
    }
});
</script>

<template>
    <div ref="previewContainer" class="video-container group relative">
        <video ref="videoElement" class="video-js vjs-theme-goonhub vjs-big-play-centered" />

        <!-- Thumbnail Preview Overlay -->
        <Transition name="preview-fade">
            <div
                v-if="showPreview && canShowPreview"
                class="preview-thumbnail border-neon-green pointer-events-none absolute z-100
                    rounded-lg border-2 bg-black shadow-[0_0_20px_rgba(46,204,113,0.3)]"
                :style="{
                    left: `${previewX}px`,
                    transform: `translateX(-50%) translateY(-100%) translateY(-75px)`,
                }"
            >
                <img ref="previewThumbnail" class="block w-full rounded-md" alt="Preview" />
                <div
                    class="absolute right-0 bottom-0 left-0 h-1/2 bg-linear-to-t from-black/80
                        to-transparent"
                ></div>
            </div>
        </Transition>
    </div>
</template>

<style>
/* Main Player Container */
.video-container {
    border-radius: 16px;
    overflow: hidden;
    background: #000;
    box-shadow: 0 20px 50px -10px rgba(0, 0, 0, 0.5);
    transition: transform 0.3s ease;
}

.video-js.vjs-fluid,
.video-js.vjs-16-9,
.video-js.vjs-4-3,
video.video-js,
video.vjs-tech {
    max-height: calc(75vh);
    position: relative !important;
    width: 100%;
    height: auto !important;
    max-width: 100% !important;
    padding-top: 0 !important;
    line-height: 0;
}

.video-js.vjs-theme-goonhub {
    font-family: 'Inter', system-ui, sans-serif;
    color: #fff;
    width: 100%;
    height: auto;
}

.video-js .vjs-tech {
    object-fit: contain;
}

/* Big Play Button */
.video-js.vjs-theme-goonhub .vjs-big-play-button {
    background-color: rgba(46, 204, 113, 0.1);
    border: 2px solid rgba(46, 204, 113, 0.8);
    border-radius: 50%;
    width: 88px;
    height: 88px;
    line-height: 84px; /* Adjust for border */
    font-size: 48px;
    margin-left: -44px;
    margin-top: -44px;
    color: #2ecc71;
    backdrop-filter: blur(4px);
    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
    box-shadow: 0 0 30px rgba(46, 204, 113, 0.2);
}

.video-js.vjs-theme-goonhub .vjs-big-play-button:hover {
    background-color: #2ecc71;
    color: #000;
    transform: scale(1.1);
    box-shadow: 0 0 50px rgba(46, 204, 113, 0.6);
    border-color: #2ecc71;
}

.video-js.vjs-theme-goonhub .vjs-big-play-button .vjs-icon-placeholder:before {
    content: '\f101'; /* Play icon */
    font-family: VideoJS;
}

/* Control Bar */
.video-js.vjs-theme-goonhub .vjs-control-bar {
    height: 72px;
    background: linear-gradient(to top, rgba(0, 0, 0, 0.9) 0%, rgba(0, 0, 0, 0) 100%);
    padding: 0 24px 12px 24px;
    display: flex;
    align-items: flex-end;
    transition: opacity 0.3s ease;
}

/* Progress Control / Timeline */
.video-js.vjs-theme-goonhub .vjs-progress-control {
    top: 0;
    height: 32px;
    width: auto;
    display: flex;
    align-items: center;
}

.video-js.vjs-theme-goonhub .vjs-progress-holder {
    height: 8px;
    border-radius: 2px;
    background: rgba(255, 255, 255, 0.2);
    margin: 0;
    transition: height 0.2s ease;
}

.video-js.vjs-theme-goonhub .vjs-progress-control:hover .vjs-progress-holder {
    height: 6px;
}

.video-js.vjs-theme-goonhub .vjs-play-progress {
    background: #2ecc71;
    border-radius: 2px;
    box-shadow: 0 0 10px rgba(46, 204, 113, 0.5);
}

.video-js.vjs-theme-goonhub .vjs-play-progress:before {
    content: '';
    display: block;
    position: absolute;
    right: -6px;
    top: 50%;
    transform: translateY(-50%) scale(0);
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: #fff;
    box-shadow: 0 0 10px rgba(46, 204, 113, 0.8);
    transition: transform 0.2s ease;
}

.video-js.vjs-theme-goonhub .vjs-progress-control:hover .vjs-play-progress:before {
    transform: translateY(-50%) scale(1);
}

.video-js.vjs-theme-goonhub .vjs-load-progress {
    background: rgba(255, 255, 255, 0.15);
    border-radius: 2px;
}

.video-js.vjs-theme-goonhub .vjs-load-progress div {
    background: transparent;
}

/* Buttons & Controls */
.video-js.vjs-theme-goonhub .vjs-control {
    width: 44px;
    height: 44px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.video-js.vjs-theme-goonhub .vjs-button > .vjs-icon-placeholder:before {
    font-size: 24px;
    line-height: 44px;
    color: #e5e5e5;
    transition: all 0.2s ease;
    text-shadow: 0 2px 4px rgba(0, 0, 0, 0.5);
}

.video-js.vjs-theme-goonhub .vjs-button:hover > .vjs-icon-placeholder:before {
    color: #2ecc71;
    text-shadow: 0 0 15px rgba(46, 204, 113, 0.6);
    transform: scale(1.1);
}

/* Time Display */
.video-js.vjs-theme-goonhub .vjs-time-control {
    line-height: 44px;
    font-size: 13px;
    font-weight: 500;
    color: #e5e5e5;
    min-width: auto;
    padding: 0 8px;
}

.video-js.vjs-theme-goonhub .vjs-current-time {
    display: block;
    padding-right: 0;
}

.video-js.vjs-theme-goonhub .vjs-duration {
    display: block;
    padding-left: 0;
}

.video-js.vjs-theme-goonhub .vjs-time-divider {
    display: block;
    line-height: 44px;
    padding: 0 4px;
    color: rgba(255, 255, 255, 0.4);
}

/* Volume Panel */
.video-js.vjs-theme-goonhub .vjs-volume-panel {
    margin-left: 8px;
    width: auto;
    transition: width 0.2s;
}

.video-js.vjs-theme-goonhub .vjs-volume-panel:hover {
    width: 130px;
}

.video-js.vjs-theme-goonhub .vjs-volume-control.vjs-volume-horizontal {
    width: 80px;
    height: 44px;
    align-items: center;
    opacity: 0;
    transition: opacity 0.2s;
}

.video-js.vjs-theme-goonhub .vjs-volume-panel:hover .vjs-volume-control.vjs-volume-horizontal {
    opacity: 1;
}

.video-js.vjs-theme-goonhub .vjs-volume-bar {
    height: 4px;
    border-radius: 2px;
    background: rgba(255, 255, 255, 0.2);
    margin: 20px 10px;
}

.video-js.vjs-theme-goonhub .vjs-volume-level {
    background: #2ecc71;
    border-radius: 2px;
}

.video-js.vjs-theme-goonhub .vjs-volume-level:before {
    right: -5px;
    top: -3px;
    font-size: 10px;
    color: #fff;
    content: '●';
}

/* Spacer */
.video-js.vjs-theme-goonhub .vjs-custom-control-spacer {
    flex: 1;
}

/* Menus (Quality, Speed) */
.video-js.vjs-theme-goonhub .vjs-menu-button-popup .vjs-menu-content {
    bottom: 20px;
    left: 50%;
    width: auto;
    transform: translateX(-50%);
    background: rgba(15, 15, 15, 0.9);
    backdrop-filter: blur(12px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.5);
    padding: 6px;
    margin-bottom: 12px;
}

.video-js.vjs-theme-goonhub .vjs-menu-item {
    background: transparent;
    padding: 8px 12px;
    font-size: 13px;
    font-weight: 500;
    color: #ccc;
    text-transform: capitalize;
    border-radius: 6px;
    margin-bottom: 2px;
    transition: all 0.2s ease;
}

.video-js.vjs-theme-goonhub .vjs-menu-item:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
}

.video-js.vjs-theme-goonhub .vjs-menu-item.vjs-selected {
    background: rgba(46, 204, 113, 0.2);
    color: #2ecc71;
}

/* Loading Spinner */
.video-js.vjs-theme-goonhub .vjs-loading-spinner {
    border: none;
    background: none;
    border-radius: 0;
}

.video-js.vjs-theme-goonhub .vjs-loading-spinner:before,
.video-js.vjs-theme-goonhub .vjs-loading-spinner:after {
    display: none;
}

.vjs-seek-to-live-control {
    display: none !important;
}

/* Custom CSS Spinner implementation here if needed, or stick to simple for now */

/* Transitions */
.preview-fade-enter-active,
.preview-fade-leave-active {
    transition:
        opacity 0.2s ease,
        transform 0.2s ease;
}

.preview-fade-enter-from,
.preview-fade-leave-to {
    opacity: 0;
    transform: translateX(-50%) translateY(-100%) translateY(-16px) scale(0.95);
}

.preview-fade-enter-to,
.preview-fade-leave-from {
    opacity: 1;
    transform: translateX(-50%) translateY(-100%) translateY(-24px) scale(1);
}
</style>
