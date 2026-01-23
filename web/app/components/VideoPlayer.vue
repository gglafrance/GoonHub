<script setup lang="ts">
import videojs from 'video.js';
import 'video.js/dist/video-js.css';
import type { Video } from '~/types/video';

interface VttCue {
    start: number;
    end: number;
    url: string;
    x: number;
    y: number;
    w: number;
    h: number;
}

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
const player = ref<any>(null);
const vttCues = ref<VttCue[]>([]);
const vttUrl = computed(() => {
    if (!props.video?.vtt_path) return null;
    return `/vtt/${props.video.id}`;
});

function parseVttTime(timeStr: string): number {
    const parts = timeStr.trim().split(':');
    if (parts.length < 3) return 0;
    const hours = parseInt(parts[0] || '0');
    const minutes = parseInt(parts[1] || '0');
    const secParts = (parts[2] || '0').split('.');
    const seconds = parseInt(secParts[0] || '0');
    const millis = parseInt(secParts[1] || '0');
    return hours * 3600 + minutes * 60 + seconds + millis / 1000;
}

async function loadVttCues(url: string) {
    try {
        const response = await fetch(url);
        const text = await response.text();
        const cues: VttCue[] = [];

        const blocks = text.split('\n\n');
        for (const block of blocks) {
            const lines = block.trim().split('\n');
            for (let i = 0; i < lines.length; i++) {
                const line = lines[i];
                if (line && line.includes('-->')) {
                    const [startStr, endStr] = line.split('-->');
                    if (!startStr || !endStr) continue;
                    const start = parseVttTime(startStr);
                    const end = parseVttTime(endStr);
                    const urlLine = lines[i + 1]?.trim();
                    if (!urlLine) continue;

                    const hashIndex = urlLine.indexOf('#xywh=');
                    if (hashIndex === -1) continue;

                    const spriteUrl = urlLine.substring(0, hashIndex);
                    const coords = urlLine
                        .substring(hashIndex + 6)
                        .split(',')
                        .map(Number);
                    cues.push({
                        start,
                        end,
                        url: spriteUrl,
                        x: coords[0] ?? 0,
                        y: coords[1] ?? 0,
                        w: coords[2] ?? 0,
                        h: coords[3] ?? 0,
                    });
                }
            }
        }
        vttCues.value = cues;
    } catch (e) {
        console.error('Failed to load VTT cues:', e);
    }
}

function setupThumbnailPreview() {
    if (!player.value) return;

    const progressControl = player.value.controlBar?.progressControl;
    if (!progressControl) return;

    const seekBar = progressControl.seekBar;
    if (!seekBar) return;

    const thumbEl = document.createElement('div');
    thumbEl.className = 'vjs-thumb-preview';
    thumbEl.style.display = 'none';
    seekBar.el().appendChild(thumbEl);

    const imgEl = document.createElement('img');
    imgEl.style.display = 'block';
    thumbEl.appendChild(imgEl);

    let currentSpriteUrl = '';

    const onMouseMove = (e: MouseEvent) => {
        if (vttCues.value.length === 0) return;

        const seekBarRect = seekBar.el().getBoundingClientRect();
        const percent = (e.clientX - seekBarRect.left) / seekBarRect.width;
        const duration = player.value.duration();
        if (!duration) return;

        const time = percent * duration;
        const cue = vttCues.value.find((c) => time >= c.start && time < c.end);
        if (!cue) {
            thumbEl.style.display = 'none';
            return;
        }

        thumbEl.style.display = 'block';

        if (currentSpriteUrl !== cue.url) {
            imgEl.src = cue.url;
            currentSpriteUrl = cue.url;
        }

        imgEl.style.objectFit = 'none';
        imgEl.style.objectPosition = `-${cue.x}px -${cue.y}px`;
        imgEl.style.width = `${cue.w}px`;
        imgEl.style.height = `${cue.h}px`;

        thumbEl.style.width = `${cue.w}px`;
        thumbEl.style.height = `${cue.h}px`;

        const thumbLeft = e.clientX - seekBarRect.left - cue.w / 2;
        const clampedLeft = Math.max(0, Math.min(thumbLeft, seekBarRect.width - cue.w));
        thumbEl.style.left = `${clampedLeft}px`;
    };

    const onMouseOut = () => {
        thumbEl.style.display = 'none';
    };

    seekBar.el().addEventListener('mousemove', onMouseMove);
    seekBar.el().addEventListener('mouseout', onMouseOut);
}

onMounted(async () => {
    if (!videoElement.value) return;

    player.value = videojs(videoElement.value, {
        controls: true,
        autoplay: props.autoplay ? 'any' : false,
        preload: 'metadata',
        fluid: true,
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

    player.value.on('play', () => emit('play'));
    player.value.on('pause', () => emit('pause'));
    player.value.on('error', (e: any) => emit('error', e));

    player.value.ready(() => {
        setupThumbnailPreview();
        if (vttUrl.value) {
            loadVttCues(vttUrl.value);
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

onBeforeUnmount(() => {
    if (player.value) {
        player.value.dispose();
    }
});
</script>

<template>
    <div class="video-wrapper">
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
    aspect-ratio: 16/9;
    height: 100%;
    background: #050505;
    overflow: hidden;
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
