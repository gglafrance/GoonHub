<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    scenes: SceneListItem[];
}>();
</script>

<template>
    <div class="c_grid grid grid-cols-2 gap-2 sm:gap-3">
        <SceneCard v-for="scene in scenes" :key="scene.id" :scene="scene" fluid />
    </div>
</template>

<style scoped>
.c_grid {
    @media (width >= 40rem /* 640px */) {
        grid-template-columns: repeat(auto-fill, 320px);
    }
}
</style>
