<script setup lang="ts">
import type { Video } from '~/types/video';

defineProps<{
    videos: Video[];
}>();
</script>

<template>
    <div
        class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 2xl:grid-cols-6"
    >
        <VideoCard v-for="video in videos" :key="video.id" :video="video" />
    </div>
</template>
