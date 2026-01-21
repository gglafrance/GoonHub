<script setup lang="ts">
import type { Video } from '~/types/video';

const props = defineProps<{
    video: Video;
}>();

const formatSize = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

const formatDate = (dateStr: string) => {
    return new Date(dateStr).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
    });
};
</script>

<template>
    <div
        class="group bg-secondary/50 hover:bg-secondary hover:shadow-neon-green/10
            hover:border-neon-green/50 relative overflow-hidden rounded-2xl border border-white/5
            backdrop-blur-md transition-all duration-300 hover:shadow-lg"
    >
        <!-- Thumbnail Placeholder -->
        <div class="relative aspect-video w-full bg-black/50">
            <div
                class="group-hover:text-neon-green absolute inset-0 flex items-center justify-center
                    text-gray-600 transition-colors"
            >
                <svg
                    xmlns="http://www.w3.org/2000/svg"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke-width="1.5"
                    stroke="currentColor"
                    class="h-12 w-12"
                >
                    <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z"
                    />
                </svg>
            </div>

            <!-- Duration Badge (Mock) -->
            <div
                class="absolute right-2 bottom-2 rounded bg-black/80 px-1.5 py-0.5 text-xs
                    font-medium text-white"
            >
                00:00
            </div>
        </div>

        <!-- Info -->
        <div class="p-4">
            <h3
                class="group-hover:text-neon-green truncate text-lg font-bold text-white
                    transition-colors"
                :title="video.title"
            >
                {{ video.title }}
            </h3>
            <div class="mt-2 flex items-center justify-between text-xs text-gray-400">
                <span>{{ formatSize(video.size) }}</span>
                <span>{{ formatDate(video.created_at) }}</span>
            </div>
        </div>
    </div>
</template>
