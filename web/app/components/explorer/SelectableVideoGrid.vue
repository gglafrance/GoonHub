<script setup lang="ts">
import type { VideoListItem } from '~/types/video';

defineProps<{
    videos: VideoListItem[];
}>();
</script>

<template>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
        <ExplorerSelectableVideoCard v-for="video in videos" :key="video.id" :video="video" />
    </div>
</template>
