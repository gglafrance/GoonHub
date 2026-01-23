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
        class="group border-border bg-surface hover:border-border-hover hover:bg-elevated relative
            block overflow-hidden rounded-lg border transition-all duration-200"
    >
        <div class="bg-void relative aspect-video w-full">
            <img
                v-if="thumbnailUrl"
                :src="thumbnailUrl"
                class="absolute inset-0 h-full w-full object-cover transition-transform duration-300
                    group-hover:scale-[1.03]"
                :alt="video.title"
                loading="lazy"
            />

            <div v-else-if="isProcessing" class="absolute inset-0 flex items-center justify-center">
                <div
                    class="border-border border-t-lava h-5 w-5 animate-spin rounded-full border-2"
                ></div>
            </div>

            <div
                v-else
                class="text-dim group-hover:text-lava absolute inset-0 flex items-center
                    justify-center transition-colors"
            >
                <Icon name="heroicons:play" size="32" />
            </div>

            <!-- Duration badge -->
            <div
                v-if="video.duration > 0"
                class="bg-void/90 absolute right-1.5 bottom-1.5 rounded px-1.5 py-0.5 font-mono
                    text-[10px] font-medium text-white backdrop-blur-sm"
            >
                {{ formatDuration(video.duration) }}
            </div>

            <!-- Hover overlay -->
            <div
                class="bg-lava/0 group-hover:bg-lava/5 absolute inset-0 transition-colors
                    duration-200"
            ></div>
        </div>

        <div class="p-3">
            <h3
                class="truncate text-xs font-medium text-white/90 transition-colors
                    group-hover:text-white"
                :title="video.title"
            >
                {{ video.title }}
            </h3>
            <div class="text-dim mt-1.5 flex items-center justify-between font-mono text-[10px]">
                <span>{{ formatSize(video.size) }}</span>
                <span>{{ formatDate(video.created_at) }}</span>
            </div>
        </div>
    </NuxtLink>
</template>
