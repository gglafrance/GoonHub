<script setup lang="ts">
import type { Video } from '~/types/video';
import { useTime } from '~/composables/useTime';

const props = defineProps<{
    video: Video;
    previewTimestamp: number;
}>();

const { formatDuration } = useTime();
</script>

<template>
    <div
        v-if="video.duration > 0"
        class="absolute bottom-0 left-0 right-0 h-2 bg-linear-to-t from-black/90 to-black/50 backdrop-blur-sm pointer-events-none"
    >
        <div
            class="absolute left-0 top-0 bottom-0 bg-neon-green/80 transition-none"
            :style="{ width: `${(previewTimestamp / video.duration) * 100}%` }"
        />

        <div
            class="absolute -top-6 rounded bg-black/90 px-2 py-0.5 text-xs font-medium text-white backdrop-blur-md transition-none"
            :style="{
                left: `${(previewTimestamp / video.duration) * 100}%`,
                transform: `translateX(-50%)`
            }"
        >
            {{ formatDuration(previewTimestamp) }}
        </div>
    </div>
</template>
