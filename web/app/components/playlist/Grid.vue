<script setup lang="ts">
import type { PlaylistListItem } from '~/types/playlist';

defineProps<{
    playlists: PlaylistListItem[];
}>();
</script>

<template>
    <div class="c_grid grid gap-4">
        <PlaylistCard v-for="p in playlists" :key="p.uuid" :playlist="p" fluid />
    </div>
</template>

<style scoped>
.c_grid {
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
}

@media (min-width: 640px) {
    .c_grid {
        grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
    }
}
</style>
