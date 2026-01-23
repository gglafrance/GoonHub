<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue';
import Plyr from 'plyr';
import 'plyr/dist/plyr.css';

const props = defineProps<{
    videoUrl: string;
    posterUrl?: string;
    autoplay?: boolean;
}>();

const emit = defineEmits<{
    play: [];
    pause: [];
    error: [error: Event];
}>();

const videoElement = ref<HTMLVideoElement>();
let player: Plyr | null = null;

onMounted(() => {
    if (!videoElement.value) return;

    player = new Plyr(videoElement.value, {
        controls: [
            'play-large',
            'play',
            'progress',
            'current-time',
            'duration',
            'mute',
            'volume',
            'fullscreen',
        ],
        autoplay: props.autoplay || false,
        autoplayPolicy: 'allow',
        clickToPlay: true,
        keyboard: {
            focused: true,
            global: true,
        },
    });

    player.on('play', () => emit('play'));
    player.on('pause', () => emit('pause'));
    player.on('error', (event) => emit('error', event as unknown as Event));
});

onUnmounted(() => {
    if (player) {
        player.destroy();
        player = null;
    }
});

watch(
    () => props.videoUrl,
    (newUrl) => {
        if (player) {
            player.source = {
                type: 'video',
                sources: [
                    {
                        src: newUrl,
                        type: 'video/mp4',
                    },
                ],
                poster: props.posterUrl,
            };
        }
    },
);
</script>

<template>
    <div class="relative w-full overflow-hidden rounded-2xl bg-black">
        <video ref="videoElement" class="w-full" controls playsinline :poster="posterUrl">
            <source :src="videoUrl" type="video/mp4" />
        </video>
    </div>
</template>

<style>
.plyr {
    --plyr-color-main: #2ecc71;
    --plyr-video-background: #000000;
    --plyr-audio-background: #0f0f0f;
    --plyr-menu-background: #0f0f0f;
    --plyr-menu-color: #ffffff;
    --plyr-control-spacing: 12px;
    --plyr-video-controls-background: linear-gradient(rgba(0, 0, 0, 0.8), rgba(0, 0, 0, 0.9));
    border-radius: 16px;
}

.plyr__controls {
    padding: 16px;
    background: rgba(15, 15, 15, 0.95);
    backdrop-filter: blur(10px);
}

.plyr__control--overlaid {
    background: rgba(46, 204, 113, 0.9);
    backdrop-filter: blur(8px);
    border-radius: 50%;
    box-shadow: 0 4px 20px rgba(46, 204, 113, 0.3);
}

.plyr__control--overlaid:hover {
    background: rgba(46, 204, 113, 1);
    transform: scale(1.05);
}

.plyr__control:hover {
    background: rgba(46, 204, 113, 0.2);
    color: #2ecc71;
}

.plyr__progress__buffer {
    color: rgba(255, 255, 255, 0.2);
}

.plyr__progress--played,
.plyr__volume--display {
    color: #2ecc71;
}

.plyr__tooltip {
    background: rgba(0, 0, 0, 0.9);
    backdrop-filter: blur(8px);
    border-radius: 8px;
}

.plyr__menu__container {
    background: rgba(15, 15, 15, 0.95);
    backdrop-filter: blur(10px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 12px;
}

.plyr__menu__value {
    color: #2ecc71;
}
</style>
