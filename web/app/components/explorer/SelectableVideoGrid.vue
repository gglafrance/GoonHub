<script setup lang="ts">
import type { Video } from '~/types/video';

defineProps<{
    videos: Video[];
}>();
</script>

<template>
    <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
        <ExplorerSelectableVideoCard
            v-for="video in videos"
            :key="video.id"
            :video="video"
        />
    </div>
</template>
