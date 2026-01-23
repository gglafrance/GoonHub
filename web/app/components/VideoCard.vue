<script setup lang="ts">
import type { Video } from '~/types/video';

const props = defineProps<{
    video: Video;
}>();

const { formatDuration } = useTime();

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

const isProcessing = computed(() => {
    return (
        props.video.processing_status === 'pending' ||
        props.video.processing_status === 'processing'
    );
});

const thumbnailUrl = computed(() => {
    return props.video.thumbnail_path ? `/thumbnails/${props.video.id}` : null;
});
</script>

<template>
    <NuxtLink
        :to="`/watch/${video.id}`"
        class="group bg-secondary/50 hover:bg-secondary hover:shadow-neon-green/10
            hover:border-neon-green/50 relative block overflow-hidden rounded-2xl border
            border-white/5 backdrop-blur-md transition-all duration-300 hover:shadow-lg"
    >
        <div class="relative aspect-video w-full cursor-pointer bg-black/50">
            <img
                v-if="thumbnailUrl"
                :src="thumbnailUrl"
                class="absolute inset-0 h-full w-full object-cover transition-transform
                    duration-300 group-hover:scale-105"
                :alt="video.title"
                loading="lazy"
            />

            <div v-else-if="isProcessing" class="absolute inset-0 flex items-center justify-center">
                <Icon name="heroicons:arrow-path" size="48" class="animate-spin text-gray-600" />
            </div>

            <div
                v-else
                class="group-hover:text-neon-green absolute inset-0 flex items-center justify-center
                    text-gray-600 transition-colors"
            >
                <Icon name="heroicons:play" size="48" />
            </div>

            <div
                v-if="video.duration > 0"
                class="absolute right-2 bottom-2 rounded bg-black/80 px-1.5 py-0.5 text-xs
                    font-medium text-white backdrop-blur-sm"
            >
                {{ formatDuration(video.duration) }}
            </div>
        </div>

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
    </NuxtLink>
</template>
