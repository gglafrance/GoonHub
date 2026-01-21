<script setup lang="ts">
import type { Video } from '~/types/video';

defineProps<{
    videos: Video[];
}>();
</script>

<template>
    <div
        class="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5"
    >
        <VideoCard v-for="video in videos" :key="video.id" :video="video" />
    </div>
</template>
