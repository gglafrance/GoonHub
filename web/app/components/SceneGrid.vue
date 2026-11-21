<script setup lang="ts">
import type { SceneListItem } from '~/types/scene';

defineProps<{
    scenes: SceneListItem[];
    ratings?: Record<string, number>;
    likes?: Record<string, boolean>;
    jizzCounts?: Record<string, number>;
    watched?: Record<string, boolean>;
}>();

const { sceneGridClass, sceneGridStyle } = useGridDensity();
</script>

<template>
    <div class="c_grid grid justify-center" :class="sceneGridClass" :style="sceneGridStyle">
        <SceneCard
            v-for="scene in scenes"
            :key="scene.id"
            :scene="scene"
            :rating="ratings?.[scene.id]"
            :liked="likes?.[scene.id]"
            :jizz-count="jizzCounts?.[scene.id]"
            :completed="watched?.[scene.id]"
            fluid
        />
    </div>
</template>

<style scoped>
.c_grid {
    @media (width >= 40rem /* 640px */) {
        grid-template-columns: repeat(auto-fill, var(--scene-card-width, 320px));
    }
}
</style>
