<script setup lang="ts">
import type { Video } from '~/types/video';

defineProps<{
    videos: Video[];
}>();
</script>

<template>
    <div class="grid justify-center gap-3" style="grid-template-columns: repeat(auto-fill, 320px)">
        <VideoCard v-for="video in videos" :key="video.id" :video="video" />
    </div>
</template>
