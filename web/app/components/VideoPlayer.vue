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
    const hours = parseInt(parts[0]);
    const minutes = parseInt(parts[1]);
    const secParts = parts[2].split('.');
    const seconds = parseInt(secParts[0]);
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
                if (lines[i].includes('-->')) {
                    const [startStr, endStr] = lines[i].split('-->');
                    const start = parseVttTime(startStr);
                    const end = parseVttTime(endStr);
                    const urlLine = lines[i + 1]?.trim();
                    if (!urlLine) continue;

                    const hashIndex = urlLine.indexOf('#xywh=');
                    if (hashIndex === -1) continue;

                    const spriteUrl = urlLine.substring(0, hashIndex);
                    const coords = urlLine.substring(hashIndex + 6).split(',').map(Number);
                    cues.push({
                        start,
                        end,
                        url: spriteUrl,
                        x: coords[0],
                        y: coords[1],
                        w: coords[2],
                        h: coords[3],
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
    background: #000;
    border-radius: 16px;
    overflow: hidden;
    box-shadow: 0 20px 50px -10px rgba(0, 0, 0, 0.5);
}

/* Override Video.js default theme to match GoonHub aesthetic */
:deep(.video-js) {
    font-family: 'Inter', system-ui, sans-serif;
    --primary-color: #2ecc71;
    --text-color: #ffffff;
}

:deep(.vjs-big-play-button) {
    background-color: rgba(46, 204, 113, 0.8);
    border: none;
    border-radius: 50%;
    width: 80px;
    height: 80px;
    line-height: 80px;
    margin-left: -40px;
    margin-top: -40px;
}

:deep(.vjs-big-play-button:hover) {
    background-color: #2ecc71;
}

:deep(.vjs-control-bar) {
    background: linear-gradient(to top, rgba(0, 0, 0, 0.9) 0%, rgba(0, 0, 0, 0) 100%);
    backdrop-filter: blur(10px);
}

:deep(.vjs-progress-control) {
    margin-right: 10px;
}

:deep(.vjs-play-progress) {
    background-color: #2ecc71;
}

:deep(.vjs-slider) {
    background-color: rgba(255, 255, 255, 0.2);
}

:deep(.vjs-load-progress) {
    background: rgba(255, 255, 255, 0.4);
}

:deep(.vjs-control) {
    color: #ffffff;
}

:deep(.vjs-control:hover) {
    color: #2ecc71;
}

:deep(.vjs-playback-rate-value) {
    font-size: 1em;
    line-height: 3.5em;
}

:deep(.vjs-thumb-preview) {
    position: absolute;
    bottom: 100%;
    margin-bottom: 8px;
    pointer-events: none;
    border: 2px solid rgba(46, 204, 113, 0.8);
    border-radius: 4px;
    overflow: hidden;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.6);
    z-index: 10;
    background: #000;
}

:deep(.vjs-thumb-preview img) {
    display: block;
}
</style>
