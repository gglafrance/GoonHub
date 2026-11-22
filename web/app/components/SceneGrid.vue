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
    <div class="scene-grid grid justify-center" :class="sceneGridClass" :style="sceneGridStyle">
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
