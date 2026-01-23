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
let player: any = null;

const previewContainer = ref<HTMLElement>();
const previewThumbnail = ref<HTMLImageElement>();
const showPreview = ref(false);
const previewX = ref(0);

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
    if (!props.video || !canShowPreview.value) return;

    const framePaths = props.video.frame_paths.split(',');
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

    const index = Math.min(Math.floor(timestamp / props.video.frame_interval), frames.length - 1);

    return frames[index] || null;
};

let framesCache: HTMLImageElement[] = [];

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
    <div ref="previewContainer" class="relative">
        <video ref="videoElement" class="video-js vjs-big-play-centered vjs-theme-goonhub" />

        <!-- Thumbnail Preview Overlay -->
        <Transition name="preview-fade">
            <div
                v-if="showPreview && canShowPreview"
                class="preview-thumbnail border-neon-green pointer-events-none absolute z-50
                    rounded-lg border-2 bg-black shadow-2xl"
                :style="{
                    left: `${previewX}px`,
                    transform: `translateX(-50%) translateY(-100%) translateY(-12px)`,
                }"
            >
                <img
                    ref="previewThumbnail"
                    class="block w-full rounded-lg"
                    :style="{ width: '160px', aspectRatio: '16/9' }"
                    alt="Preview"
                />
            </div>
        </Transition>
    </div>
</template>

<style>
.video-js {
    font-family: inherit;
    border-radius: 16px;
    overflow: hidden;
    background-color: #000000;
}

.video-js .vjs-big-play-button {
    background-color: rgba(46, 204, 113, 0.9);
    border: none;
    border-radius: 50%;
    width: 80px;
    height: 80px;
    line-height: 80px;
    font-size: 40px;
    color: #000000;
    backdrop-filter: blur(8px);
    box-shadow: 0 4px 20px rgba(46, 204, 113, 0.3);
    transition: all 0.3s ease;
}

.video-js .vjs-big-play-button:hover {
    background-color: #2ecc71;
    transform: scale(1.05);
    box-shadow: 0 6px 24px rgba(46, 204, 113, 0.4);
}

.video-js .vjs-control-bar {
    background: rgba(15, 15, 15, 0.95);
    backdrop-filter: blur(10px);
    padding: 12px;
    display: flex;
    align-items: center;
}

.video-js .vjs-play-progress {
    background-color: #2ecc71;
}

.video-js .vjs-load-progress {
    background: rgba(255, 255, 255, 0.2);
}

.video-js .vjs-slider {
    background-color: rgba(255, 255, 255, 0.1);
}

.video-js .vjs-play-progress,
.video-js .vjs-volume-level {
    background-color: #2ecc71;
}

.video-js .vjs-progress-holder .vjs-load-progress div {
    background: rgba(255, 255, 255, 0.3);
}

.video-js .vjs-control {
    color: #ffffff;
    transition: color 0.2s ease;
}

.video-js .vjs-control:hover {
    color: #2ecc71;
}

.video-js .vjs-time-control {
    color: #ffffff;
    font-size: 14px;
}

.video-js .vjs-current-time,
.video-js .vjs-duration {
    line-height: 40px;
}

.video-js .vjs-play-button .vjs-icon-placeholder::before,
.video-js .vjs-pause-button .vjs-icon-placeholder::before {
    color: #ffffff;
}

.video-js .vjs-play-button:hover .vjs-icon-placeholder::before,
.video-js .vjs-pause-button:hover .vjs-icon-placeholder::before {
    color: #2ecc71;
}

.video-js .vjs-volume-panel .vjs-mute-control:hover .vjs-icon-placeholder::before,
.video-js .vjs-volume-panel .vjs-volume-control:hover .vjs-icon-placeholder::before {
    color: #2ecc71;
}

.video-js .vjs-fullscreen-control:hover .vjs-icon-placeholder::before {
    color: #2ecc71;
}

.video-js .vjs-menu-button-popup .vjs-menu {
    background: rgba(15, 15, 15, 0.95);
    backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
    padding: 8px;
}

.video-js .vjs-menu .vjs-menu-item {
    color: #ffffff;
    font-size: 14px;
    padding: 8px 16px;
    border-radius: 8px;
    transition: background-color 0.2s ease;
}

.video-js .vjs-menu .vjs-menu-item:hover {
    background-color: rgba(46, 204, 113, 0.2);
    color: #2ecc71;
}

.video-js .vjs-menu .vjs-menu-item.vjs-selected {
    background-color: rgba(46, 204, 113, 0.3);
    color: #2ecc71;
}

.video-js .vjs-poster {
    background-size: cover;
}

.video-js .vjs-overlay {
    background: rgba(0, 0, 0, 0.8);
    backdrop-filter: blur(8px);
    border-radius: 12px;
    color: #ffffff;
}

.video-js .vjs-modal-dialog {
    background: rgba(15, 15, 15, 0.95);
    backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
}

.video-js .vjs-modal-dialog-content {
    color: #ffffff;
}

.video-js .vjs-captions-button .vjs-icon-placeholder::before {
    color: #ffffff;
}

.video-js .vjs-texttrack-display {
    text-shadow: 0 2px 4px rgba(0, 0, 0, 0.8);
}

.video-js .vjs-subs-caps-button .vjs-icon-placeholder::before {
    color: #ffffff;
}

.preview-thumbnail {
    position: absolute;
    bottom: 100%;
    transition: opacity 0.15s ease;
}

.preview-fade-enter-active,
.preview-fade-leave-active {
    transition: opacity 0.15s ease;
}

.preview-fade-enter-from,
.preview-fade-leave-to {
    opacity: 0;
}

.preview-fade-enter-to,
.preview-fade-leave-from {
    opacity: 1;
}

.video-js .vjs-progress-holder {
    position: relative;
}
</style>
